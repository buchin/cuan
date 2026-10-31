package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

type activeAttempt struct {
	attempt *task.TaskAttempt
	lane    string
	cancel  context.CancelFunc
}

type Runner struct {
	cfg        *config.Config
	adapter    agent.Adapter
	gitClient  *git.Client
	httpClient *http.Client
	baseURL    string

	// Multi-Lane Concurrency Mutexes (Pemisahan Mutex Runner)
	gitDeployMu sync.Mutex   // Code lane: git operations, builds, deploys
	opsMu       sync.Mutex   // Ops lane: non-blocking operational tasks (artisan, sync, etc.)
	readersMu   sync.RWMutex // Readonly lane: shared reader pool

	mu             sync.Mutex // Protects active attempts registry & backward-compatible state
	activeAttempts map[string]*activeAttempt
	currentAttempt *task.TaskAttempt
	cancelAttempt  context.CancelFunc

	drainCoord *DrainCoordinator
	updater    *UpdateManager
}

func NewRunner(cfg *config.Config, adapter agent.Adapter, customHTTPClient ...*http.Client) *Runner {
	client := http.DefaultClient
	if len(customHTTPClient) > 0 && customHTTPClient[0] != nil {
		client = customHTTPClient[0]
	}

	baseURL := strings.TrimRight(cfg.RunnerServerURL, "/")

	return &Runner{
		cfg:            cfg,
		adapter:        adapter,
		gitClient:      git.NewClient(),
		httpClient:     client,
		baseURL:        baseURL,
		activeAttempts: make(map[string]*activeAttempt),
		drainCoord:     NewDrainCoordinator(""),
		updater:        NewUpdateManager(""),
	}
}

func (r *Runner) SetGitClient(client *git.Client) {
	r.gitClient = client
}

func (r *Runner) SetDrainCoordinator(dc *DrainCoordinator) {
	if dc != nil {
		r.drainCoord = dc
	}
}

func (r *Runner) SetUpdateManager(um *UpdateManager) {
	if um != nil {
		r.updater = um
	}
}

func (r *Runner) DrainCoordinator() *DrainCoordinator {
	return r.drainCoord
}

func (r *Runner) UpdateManager() *UpdateManager {
	return r.updater
}

func (r *Runner) RequestDrain(reason string) {
	if r.drainCoord != nil {
		r.drainCoord.RequestDrain(reason)
	}
}

func (r *Runner) IsDraining() bool {
	if r.drainCoord != nil {
		return r.drainCoord.IsDraining()
	}
	return false
}

func (r *Runner) DrainReason() string {
	if r.drainCoord != nil {
		return r.drainCoord.DrainReason()
	}
	return ""
}

func (r *Runner) ActiveAttemptsCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.activeAttempts)
}

func (r *Runner) WaitForInFlight(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if r.ActiveAttemptsCount() == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (r *Runner) CheckAndPerformSelfUpdate() bool {
	if r.updater == nil {
		return false
	}
	hasUpdate, newSHA, err := r.updater.CheckForUpdate()
	if err != nil {
		log.Printf("[Runner %s] Self-update check note: %v", r.cfg.RunnerID, err)
		return false
	}
	if !hasUpdate {
		return false
	}
	log.Printf("[Runner %s] New binary detected on disk (SHA: %s). Initiating graceful self-update handover...", r.cfg.RunnerID, newSHA[:12])
	if err := r.updater.PerformSelfUpdate(os.Args, os.Environ()); err != nil {
		log.Printf("[Runner %s] Self-update restart failed: %v", r.cfg.RunnerID, err)
		return false
	}
	return true
}

// Start runs the runner daemon: background heartbeat & queue polling loop.
func (r *Runner) Start(ctx context.Context) error {
	version, err := r.adapter.Version()
	if err != nil {
		return fmt.Errorf("check agent adapter: %w", err)
	}

	hostname, _ := os.Hostname()

	log.Printf("[Runner %s] Started on host '%s' with agent '%s' (ver: %s)", r.cfg.RunnerID, hostname, r.adapter.Name(), version)
	log.Printf("[Runner %s] Outbound server URL: %s, poll interval: %v", r.cfg.RunnerID, r.baseURL, r.cfg.RunnerInterval)

	// Start background drain flag monitor
	if r.drainCoord != nil {
		go r.drainCoord.StartMonitor(ctx, 1*time.Second)
	}

	// Immediate initial heartbeat
	r.sendHeartbeat(ctx, hostname)

	// Heartbeat ticker (every 3 seconds)
	heartbeatTicker := time.NewTicker(3 * time.Second)
	defer heartbeatTicker.Stop()

	// Queue poll ticker
	pollTicker := time.NewTicker(r.cfg.RunnerInterval)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Runner %s] Shutdown signaled, initiating graceful drain...", r.cfg.RunnerID)
			r.RequestDrain("shutdown context canceled")
			// Allow active in-flight tasks to finish completely without interruption
			drained := r.WaitForInFlight(30 * time.Minute)
			if !drained {
				log.Printf("[Runner %s] Graceful drain timeout reached, terminating in-flight work...", r.cfg.RunnerID)
				r.cancelCurrentExecution()
			} else {
				log.Printf("[Runner %s] All in-flight work completed cleanly.", r.cfg.RunnerID)
			}
			// Check for self-update if a new binary is on disk
			r.CheckAndPerformSelfUpdate()
			return nil

		case <-heartbeatTicker.C:
			r.sendHeartbeat(ctx, hostname)

		case <-pollTicker.C:
			r.pollAndExecute(ctx)
		}
	}
}

func (r *Runner) sendHeartbeat(ctx context.Context, hostname string) {
	metrics := CollectHostMetrics()

	r.mu.Lock()
	var attemptIDs []string
	primaryAttemptID := ""
	for attID := range r.activeAttempts {
		attemptIDs = append(attemptIDs, attID)
		if primaryAttemptID == "" {
			primaryAttemptID = attID
		}
	}
	if primaryAttemptID == "" && r.currentAttempt != nil {
		primaryAttemptID = r.currentAttempt.ID
		attemptIDs = append(attemptIDs, primaryAttemptID)
	}
	r.mu.Unlock()

	reqPayload := HeartbeatRequest{
		RunnerID:         r.cfg.RunnerID,
		Hostname:         hostname,
		SupportedAgents:  []string{r.adapter.Name()},
		CurrentAttemptID: primaryAttemptID,
		ActiveAttemptIDs: attemptIDs,
		CPUPercent:       metrics.CPUPercent,
		MemoryUsedBytes:  metrics.MemoryUsedBytes,
		MemoryTotalBytes: metrics.MemoryTotalBytes,
		DiskUsedBytes:    metrics.DiskUsedBytes,
		DiskTotalBytes:   metrics.DiskTotalBytes,
	}

	resp, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/heartbeat", reqPayload)
	if err != nil {
		log.Printf("[Runner %s] Heartbeat failed: %v", r.cfg.RunnerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var hbResp HeartbeatResponse
		if err := json.NewDecoder(resp.Body).Decode(&hbResp); err == nil {
			if len(hbResp.CanceledAttemptIDs) > 0 {
				for _, cid := range hbResp.CanceledAttemptIDs {
					log.Printf("[Runner %s] Heartbeat received cancellation directive for attempt %s", r.cfg.RunnerID, cid)
					r.cancelSpecificAttempt(cid)
				}
			} else if hbResp.ShouldCancel {
				log.Printf("[Runner %s] Heartbeat received cancellation directive for active attempts", r.cfg.RunnerID)
				r.cancelCurrentExecution()
			}
		}
	}
}

func (r *Runner) pollAndExecute(ctx context.Context) {
	// If runner is in graceful drain mode, reject claiming new tasks
	if r.IsDraining() {
		return
	}

	// Determine available lanes based on active executions
	var availableLanes []string
	r.mu.Lock()
	hasCode := false
	hasOps := false
	readCount := 0
	for _, act := range r.activeAttempts {
		switch act.lane {
		case task.ExecutionCategoryCode:
			hasCode = true
		case task.ExecutionCategoryOps:
			hasOps = true
		case task.ExecutionCategoryReadOnly:
			readCount++
		}
	}
	if !hasCode {
		availableLanes = append(availableLanes, task.ExecutionCategoryCode)
	}
	if !hasOps {
		availableLanes = append(availableLanes, task.ExecutionCategoryOps)
	}
	if readCount < 4 {
		availableLanes = append(availableLanes, task.ExecutionCategoryReadOnly)
	}
	r.mu.Unlock()

	if len(availableLanes) == 0 {
		return
	}

	claimReq := ClaimTaskRequest{
		RunnerID:        r.cfg.RunnerID,
		SupportedAgents: []string{r.adapter.Name()},
		AvailableLanes:  availableLanes,
	}

	resp, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/claim", claimReq)
	if err != nil {
		log.Printf("[Runner %s] Claim poll failed: %v", r.cfg.RunnerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var claimResp ClaimTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&claimResp); err != nil || !claimResp.Claimed || claimResp.Attempt == nil {
		return
	}

	lane := task.NormalizeExecutionCategory(claimResp.Task.ExecutionCategory)
	if lane == "" {
		lane = task.ExecutionCategoryCode
	}

	log.Printf("[Runner %s] Claimed %s task attempt %s (Task: '%s')", r.cfg.RunnerID, lane, claimResp.Attempt.ID, claimResp.Task.Title)

	// Acquire specific lane mutex:
	// - code: git deployment mutex (build/git isolated)
	// - ops: operational mutex (non-blocking vs code)
	// - readonly: reader pool RLock (shared concurrent readers)
	switch lane {
	case task.ExecutionCategoryCode:
		r.gitDeployMu.Lock()
	case task.ExecutionCategoryOps:
		r.opsMu.Lock()
	case task.ExecutionCategoryReadOnly:
		r.readersMu.RLock()
	}

	execCtx, cancelFunc := context.WithCancel(ctx)
	r.mu.Lock()
	if r.activeAttempts == nil {
		r.activeAttempts = make(map[string]*activeAttempt)
	}
	r.activeAttempts[claimResp.Attempt.ID] = &activeAttempt{
		attempt: claimResp.Attempt,
		lane:    lane,
		cancel:  cancelFunc,
	}
	r.currentAttempt = claimResp.Attempt
	r.cancelAttempt = cancelFunc
	r.mu.Unlock()

	// Execute attempt in background goroutine so heartbeat ticker and other lanes continue running concurrently
	go func(claim ClaimTaskResponse, claimedLane string) {
		defer func() {
			switch claimedLane {
			case task.ExecutionCategoryCode:
				r.gitDeployMu.Unlock()
			case task.ExecutionCategoryOps:
				r.opsMu.Unlock()
			case task.ExecutionCategoryReadOnly:
				r.readersMu.RUnlock()
			}

			r.mu.Lock()
			delete(r.activeAttempts, claim.Attempt.ID)
			if r.currentAttempt != nil && r.currentAttempt.ID == claim.Attempt.ID {
				r.currentAttempt = nil
				r.cancelAttempt = nil
				for _, rem := range r.activeAttempts {
					r.currentAttempt = rem.attempt
					r.cancelAttempt = rem.cancel
					break
				}
			}
			remainingInFlight := len(r.activeAttempts)
			r.mu.Unlock()

			// Check for self-update once all in-flight attempts complete
			if remainingInFlight == 0 {
				r.CheckAndPerformSelfUpdate()
			}
		}()

		r.runAttempt(execCtx, claim)
	}(claimResp, lane)
}

func (r *Runner) runAttempt(ctx context.Context, claim ClaimTaskResponse) {
	att := claim.Attempt
	t := claim.Task
	startTime := time.Now()

	lane := task.NormalizeExecutionCategory(t.ExecutionCategory)
	if lane == "" {
		lane = task.ExecutionCategoryCode
	}

	// Progressive Adaptive Timeout calculation (Attempt 1: 15m, Attempt 2: 20m, Attempt 3+: 25m)
	timeoutDuration := CalculateAdaptiveTimeout(att.AttemptNumber)
	gracePeriod := GetGracePeriod()

	// Stream initial start log
	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Runner '%s' memulai eksekusi attempt %s (%s lane, attempt #%d, timeout: %v) dengan agent '%s'.",
		r.cfg.RunnerID, att.ID, lane, att.AttemptNumber, timeoutDuration, r.adapter.Name()))

	// Prepare isolated workspace per task
	var ws *git.TaskWorkspace
	workingDir := ""
	if r.gitClient != nil && lane == task.ExecutionCategoryCode {
		branchName := t.BranchName
		if branchName == "" {
			branchName = fmt.Sprintf("task/%s", t.ID)
		}
		defaultBranch := claim.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}
		baseRepoDir := claim.WorkingDir
		if baseRepoDir == "" && t.ProjectID != "" {
			baseRepoDir = filepath.Join("data", "projects", t.ProjectID)
		}

		var wsErr error
		ws, wsErr = r.gitClient.PrepareTaskWorkspace(ctx, git.WorkspaceConfig{
			TaskID:            t.ID,
			ProjectID:         t.ProjectID,
			BaseRepoDir:       baseRepoDir,
			WorkspacesBaseDir: r.cfg.WorkspacesDir,
			BranchName:        branchName,
			DefaultBranch:     defaultBranch,
			AuthorName:        r.cfg.GitAuthorName,
			AuthorEmail:       r.cfg.GitAuthorEmail,
			RemoteURL:         claim.RepoURL,
		})
		if wsErr != nil {
			r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Catatan persiapan workspace: %v", wsErr))
		} else if ws != nil {
			workingDir = ws.Path
			r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Workspace terisolasi siap di '%s' pada branch '%s'.", ws.Path, ws.BranchName))
		}
	} else if lane != task.ExecutionCategoryCode {
		if claim.WorkingDir != "" {
			workingDir = claim.WorkingDir
		} else if t.ProjectID != "" {
			workingDir = filepath.Join("data", "projects", t.ProjectID)
		}
		r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Lane %s: menggunakan direktori operasional '%s' tanpa checkout git branch baru.", lane, workingDir))
	}

	// Prepare execution request
	prompt := t.Instructions
	if prompt == "" {
		prompt = t.Title
	}
	if claim.AgentPrompt != "" {
		prompt += claim.AgentPrompt
	}

	// Prepare workspace settings & MCP tool configurations for runner session
	home, _ := os.UserHomeDir()
	_ = agent.PrepareAntigravitySettings(home, workingDir)
	_ = agent.EnsureAntigravityMCPConfig(home)
	if workingDir != "" {
		_ = agent.EnsureWorkspaceMCPConfig(workingDir)
	}

	env := map[string]string{}
	if claim.DeployBranch != "" {
		env["CUAN_DEPLOY_BRANCH"] = claim.DeployBranch
	}
	if claim.TaskAgent != nil {
		encoded, _ := json.Marshal(claim.TaskAgent)
		env["CUAN_TASK_CAPABILITY_JSON"] = string(encoded)
	}
	env["CUAN_TASK_ATTEMPT"] = fmt.Sprintf("%d", att.AttemptNumber)
	env["CUAN_TASK_TIMEOUT"] = timeoutDuration.String()
	env["CUAN_TASK_GRACE_PERIOD"] = gracePeriod.String()

	req := agent.ExecutionRequest{
		TaskID:       t.ID,
		Prompt:       prompt,
		WorkingDir:   workingDir,
		Env:          env,
		Timeout:      timeoutDuration,
		OutputFormat: "json",
	}

	result, err := r.adapter.Execute(ctx, req)
	duration := time.Since(startTime)

	var compReq CompleteAttemptRequest
	compReq.RunnerID = r.cfg.RunnerID
	compReq.AgentReportRequired = claim.TaskAgent != nil

	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			compReq.Status = task.AttemptStatusCanceled
			compReq.ExitCode = 130
			compReq.ErrorMessage = "Execution canceled by runner context"
		} else {
			compReq.Status = task.AttemptStatusFailed
			compReq.ExitCode = 1
			compReq.ErrorMessage = err.Error()
		}
	} else if result != nil {
		compReq.PeakRSSBytes = result.PeakRSSBytes

		// Stream stdout & stderr logs (bounded & masked)
		r.streamOutputLogs(ctx, att.ID, result.Stdout, result.Stderr)

		if result.Status == "CANCELED" {
			compReq.Status = task.AttemptStatusCanceled
			compReq.ExitCode = 130
			compReq.ErrorMessage = "Execution canceled"
		} else if result.Status == "TIMEOUT" {
			compReq.Status = task.AttemptStatusFailed
			compReq.ExitCode = 124
			compReq.ErrorMessage = fmt.Sprintf("Execution timed out after adaptive timeout (%v)", timeoutDuration)
		} else if result.ExitCode != 0 || result.Status == "FAILED" {
			compReq.Status = task.AttemptStatusFailed
			compReq.ExitCode = result.ExitCode
			if compReq.ExitCode == 0 {
				compReq.ExitCode = 1
			}
			compReq.ErrorMessage = result.Error
			if compReq.ErrorMessage == "" {
				compReq.ErrorMessage = "Agent reported execution failure"
			}
		} else {
			compReq.Status = task.AttemptStatusSucceeded
			compReq.ExitCode = 0
			compReq.AgentReportConfirmed = r.confirmAgentReport(ctx, claim)
			if !compReq.AgentReportConfirmed {
				compReq.Status = task.AttemptStatusFailed
				compReq.ErrorMessage = "Agent finished without a confirmed final task report"
			}
			if compReq.Status == task.AttemptStatusSucceeded {
				verification := runProjectVerification(ctx, workingDir)
				compReq.TestSummary = verification.Summary
				if !verification.Passed {
					compReq.Status = task.AttemptStatusFailed
					compReq.ErrorMessage = verification.Error
				}
			}
			if compReq.Status == task.AttemptStatusSucceeded && claim.AutoDeploy && lane == task.ExecutionCategoryCode {
				deployed, errNote := r.confirmAutoDeployCompliance(ctx, claim)
				if !deployed {
					compReq.Status = task.AttemptStatusFailed
					compReq.ErrorMessage = errNote
				} else {
					compReq.LiveDeploymentConfirmed = true
				}
			}
		}
	} else {
		compReq.Status = task.AttemptStatusFailed
		compReq.ExitCode = 1
		compReq.ErrorMessage = "No execution result returned from agent adapter"
	}

	// Post-execution Git & GitHub PR Flow (Code lane only)
	r.postExecutionGitFlow(ctx, ws, t, att, claim, &compReq, lane)

	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Attempt %s selesai: %s (exit: %d, durasi: %v, peak RSS: %d bytes).",
		att.ID, compReq.Status, compReq.ExitCode, duration.Round(time.Millisecond), compReq.PeakRSSBytes))

	// Send complete notification to server
	urlPath := fmt.Sprintf("/api/v1/runner/attempts/%s/complete", att.ID)
	_, _ = r.doRequest(context.Background(), http.MethodPost, urlPath, compReq)
	log.Printf("[Runner %s] Attempt %s finalized with status '%s' (exit %d)", r.cfg.RunnerID, att.ID, compReq.Status, compReq.ExitCode)
}

func (r *Runner) cancelCurrentExecution() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancelAttempt != nil {
		r.cancelAttempt()
	}
	for _, act := range r.activeAttempts {
		if act.cancel != nil {
			act.cancel()
		}
	}
}

func (r *Runner) cancelSpecificAttempt(attemptID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if act, ok := r.activeAttempts[attemptID]; ok && act.cancel != nil {
		act.cancel()
	}
}

func (r *Runner) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	reqURL := r.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CuanRunner/"+r.cfg.RunnerID)
	// Runner authentication via token
	if r.cfg.RunnerToken != "" {
		req.Header.Set("X-Runner-Token", r.cfg.RunnerToken)
		req.Header.Set("Authorization", "Bearer "+r.cfg.RunnerToken)
	}

	return r.httpClient.Do(req)
}

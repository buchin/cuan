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
	"sync/atomic"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	cuanPrompt "github.com/buchin/cuan/internal/prompt"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

type activeAttempt struct {
	attempt   *task.TaskAttempt
	lane      string
	cancel    context.CancelFunc
	steerFunc func(prompt string) error
	steerChan chan string
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

	dodEvaluator *DoDEvaluator
	eventBus     *EventBus
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
		dodEvaluator:   NewDoDEvaluator(),
		eventBus:       GlobalEventBus,
	}
}

func (r *Runner) SetDoDEvaluator(eval *DoDEvaluator) {
	if eval != nil {
		r.dodEvaluator = eval
	}
}

func (r *Runner) DoDEvaluator() *DoDEvaluator {
	return r.dodEvaluator
}

func (r *Runner) SetEventBus(bus *EventBus) {
	if bus != nil {
		r.eventBus = bus
	}
}

func (r *Runner) EventBus() *EventBus {
	return r.eventBus
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

	// Initial workspace GC on runner startup to clean up leftover checkouts from prior crashes or restarts
	if r.gitClient != nil && r.cfg.WorkspacesDir != "" {
		if pruned, _ := r.gitClient.PruneStaleWorkspaces(r.cfg.WorkspacesDir, 24*time.Hour, "", 5); pruned > 0 {
			log.Printf("[Runner %s] Startup GC: pruned %d stale workspace(s)", r.cfg.RunnerID, pruned)
		}
	}

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

			// Process interactive steering directives from server
			if len(hbResp.SteeringDirectives) > 0 {
				for _, dir := range hbResp.SteeringDirectives {
					log.Printf("[Runner %s] Heartbeat received steering directive for attempt %s: %s", r.cfg.RunnerID, dir.AttemptID, dir.Prompt)
					if sErr := r.SteerAttempt(dir.AttemptID, dir.Prompt); sErr != nil {
						r.streamLogLine(ctx, dir.AttemptID, "system", fmt.Sprintf("Steering fallback: %v. Arahan dicatat sebagai follow-up.", sErr))
					} else {
						r.streamLogLine(ctx, dir.AttemptID, "system", fmt.Sprintf("Interactive steering injected ke agent (dari %s): %s", dir.Author, dir.Prompt))
					}
				}
			}
		}
	}
}

// SteerAttempt delivers an interactive steering prompt into the stdin of an active agent execution.
func (r *Runner) SteerAttempt(attemptID, prompt string) error {
	r.mu.Lock()
	act, exists := r.activeAttempts[attemptID]
	r.mu.Unlock()

	if !exists || act == nil {
		return fmt.Errorf("attempt %s is not active on runner %s", attemptID, r.cfg.RunnerID)
	}

	if act.steerFunc != nil {
		return act.steerFunc(prompt)
	}
	if act.steerChan != nil {
		select {
		case act.steerChan <- prompt:
			return nil
		default:
			return errors.New("steer channel is full or blocked")
		}
	}
	return errors.New("agent adapter does not support interactive steering")
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
	steerChan := make(chan string, 16)
	r.mu.Lock()
	if r.activeAttempts == nil {
		r.activeAttempts = make(map[string]*activeAttempt)
	}
	r.activeAttempts[claimResp.Attempt.ID] = &activeAttempt{
		attempt:   claimResp.Attempt,
		lane:      lane,
		cancel:    cancelFunc,
		steerChan: steerChan,
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

	// Adaptive Timeout calculation with pruned 5m timeout for investigation/readonly tasks
	timeoutDuration := CalculateTaskTimeout(lane, att.AttemptNumber, t.Title, t.Instructions)
	gracePeriod := GetGracePeriod()

	// Stream initial start log
	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Runner '%s' memulai eksekusi attempt %s (%s lane, attempt #%d, timeout: %v) dengan agent '%s'.",
		r.cfg.RunnerID, att.ID, lane, att.AttemptNumber, timeoutDuration, r.adapter.Name()))

	// Prepare isolated workspace per task
	var ws *git.TaskWorkspace
	var err error
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
			r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Gagal menyiapkan workspace: %v", wsErr))
			err = fmt.Errorf("prepare task workspace: %w", wsErr)
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

	// Prepare execution request with AGENTS.md guidance, DoD checklist, and Scrum rules injected
	var dodTexts []string
	for _, item := range t.DoDChecklist {
		if strings.TrimSpace(item.Text) != "" {
			dodTexts = append(dodTexts, item.Text)
		}
	}
	prompt := cuanPrompt.BuildRunnerPromptWithDoD(t.Title, t.Instructions, claim.AgentPrompt, dodTexts, workingDir)

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

	var liveStreamedCount int64
	var result *agent.ExecutionResult

	if err == nil {
		// If using the official agy adapter on a host where agy binary is present,
		// execute inside a native OS pseudo-terminal (creack/pty) and expose a live
		// Unix domain socket bridge for zero-downtime streaming.
		if _, isAGY := r.adapter.(*agent.AGYAdapter); isAGY {
			if agyPath, rErr := agent.ResolveAGYBinaryPath(); rErr == nil && agyPath != "" {
				result, err = r.executeWithPTY(ctx, claim, t, att, prompt, workingDir, env, timeoutDuration, &liveStreamedCount)
			}
		}

		// Fallback to standard adapter execution (used by mock adapters in test suite)
		if result == nil && err == nil {
			req := agent.ExecutionRequest{
				TaskID:       t.ID,
				Prompt:       prompt,
				WorkingDir:   workingDir,
				Env:          env,
				Timeout:      timeoutDuration,
				OutputFormat: "stream-json",
				OnLog: func(stream, line string) {
					atomic.AddInt64(&liveStreamedCount, 1)
					r.streamLogLine(ctx, att.ID, stream, line)
				},
				RegisterSteer: func(steerFn func(prompt string) error) {
					r.mu.Lock()
					if act, ok := r.activeAttempts[att.ID]; ok {
						act.steerFunc = steerFn
					}
					r.mu.Unlock()
				},
			}

			r.mu.Lock()
			if act, ok := r.activeAttempts[att.ID]; ok {
				req.SteerChan = act.steerChan
			}
			r.mu.Unlock()

			result, err = r.adapter.Execute(ctx, req)
		}
	}
	duration := time.Since(startTime)

	var compReq CompleteAttemptRequest
	compReq.RunnerID = r.cfg.RunnerID
	compReq.AgentReportRequired = claim.TaskAgent != nil
	if claim.Task != nil && claim.Task.SourceRoomID != "" {
		compReq.RoomID = claim.Task.SourceRoomID
	}

	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			compReq.Status = task.AttemptStatusCanceled
			compReq.ExitCode = 130
			compReq.ErrorMessage = "Execution canceled by runner context"
		} else if IsInfraFault("", err.Error(), 1) {
			compReq.Status = task.AttemptStatusInterrupted
			compReq.ExitCode = 137
			compReq.ErrorMessage = err.Error()
		} else {
			compReq.Status = task.AttemptStatusFailed
			compReq.ExitCode = 1
			compReq.ErrorMessage = err.Error()
		}
	} else if result != nil {
		compReq.PeakRSSBytes = result.PeakRSSBytes

		// If no logs were live-streamed during execution (e.g. mock adapter), stream buffered output
		if atomic.LoadInt64(&liveStreamedCount) == 0 {
			r.streamOutputLogs(ctx, att.ID, result.Stdout, result.Stderr)
		}

		if result.Status == "CANCELED" {
			compReq.Status = task.AttemptStatusCanceled
			compReq.ExitCode = 130
			compReq.ErrorMessage = "Execution canceled"
		} else if result.Status == "INTERRUPTED" || IsInfraFault(result.Status, result.Error, result.ExitCode) {
			compReq.Status = task.AttemptStatusInterrupted
			compReq.ExitCode = result.ExitCode
			if compReq.ExitCode == 0 {
				compReq.ExitCode = 137
			}
			compReq.ErrorMessage = result.Error
			if compReq.ErrorMessage == "" {
				compReq.ErrorMessage = "Runner connection lost or process interrupted"
			}
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
			if compReq.Status == task.AttemptStatusSucceeded && claim.AutoDeploy {
				hasCodeChanges, diffSummary := r.detectRepoCodeChanges(ctx, ws, claim.DefaultBranch)
				compReq.HasCodeChanges = &hasCodeChanges
				requiresDeploy := hasCodeChanges && lane == task.ExecutionCategoryCode
				compReq.RequiresDeploy = &requiresDeploy
				if diffSummary != "" && compReq.DiffSummary == "" {
					compReq.DiffSummary = diffSummary
				}

				if requiresDeploy {
					deployed, errNote := r.confirmAutoDeployCompliance(ctx, claim, true)
					if !deployed {
						compReq.Status = task.AttemptStatusFailed
						compReq.ErrorMessage = errNote
					} else {
						compReq.LiveDeploymentConfirmed = true
					}
				} else {
					// Non-code or investigation tasks pass compliance without forcing fake live deploy
					passed, errNote := r.confirmAutoDeployCompliance(ctx, claim, false)
					if !passed {
						compReq.Status = task.AttemptStatusFailed
						compReq.ErrorMessage = errNote
					} else {
						r.streamLogLine(ctx, att.ID, "system", "Tugas investigasi/non-kode tanpa modifikasi berkas repo: meloloskan gate kepatuhan auto_deploy tanpa deployment palsu.")
					}
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

	// Emisi event task:attempt_succeeded & evaluasi DoD otonom pada worker
	if compReq.Status == task.AttemptStatusSucceeded {
		r.processCompletionEventAndDoD(ctx, ws, t, att, claim, &compReq, lane, workingDir)
	}

	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Attempt %s selesai: %s (exit: %d, durasi: %v, peak RSS: %d bytes).",
		att.ID, compReq.Status, compReq.ExitCode, duration.Round(time.Millisecond), compReq.PeakRSSBytes))

	// Send complete notification to server
	urlPath := fmt.Sprintf("/api/v1/runner/attempts/%s/complete", att.ID)
	_, _ = r.doRequest(context.Background(), http.MethodPost, urlPath, compReq)
	log.Printf("[Runner %s] Attempt %s finalized with status '%s' (exit %d)", r.cfg.RunnerID, att.ID, compReq.Status, compReq.ExitCode)

	// Clean up stale or excess workspaces post-execution
	if r.gitClient != nil && r.cfg.WorkspacesDir != "" {
		if pruned, _ := r.gitClient.PruneStaleWorkspaces(r.cfg.WorkspacesDir, 24*time.Hour, "", 5); pruned > 0 {
			log.Printf("[Runner %s] Post-execution GC: pruned %d stale workspace(s)", r.cfg.RunnerID, pruned)
		}
	}
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

// executeWithPTY spawns agy inside a native pseudo-terminal (creack/pty) and binds
// Unix domain sockets for decoupled streaming to cuan-server.
func (r *Runner) executeWithPTY(
	ctx context.Context,
	claim ClaimTaskResponse,
	t *task.Task,
	att *task.TaskAttempt,
	prompt, workingDir string,
	env map[string]string,
	timeoutDuration time.Duration,
	liveStreamedCount *int64,
) (*agent.ExecutionResult, error) {
	agyBin, err := agent.ResolveAGYBinaryPath()
	if err != nil || agyBin == "" {
		return nil, fmt.Errorf("resolve agy binary: %w", err)
	}

	execCtx, cancelExec := context.WithTimeout(ctx, timeoutDuration)
	defer cancelExec()

	r.mu.Lock()
	if act, ok := r.activeAttempts[att.ID]; ok {
		act.cancel = cancelExec
	}
	r.mu.Unlock()

	ptyArgs := []string{
		"--dangerously-skip-permissions",
		"--prompt-interactive", prompt,
		"--add-dir", workingDir,
	}
	if claim.Task != nil && claim.Task.AssignedAgent != "" && claim.Task.AssignedAgent != "agy" {
		ptyArgs = append(ptyArgs, "--model", claim.Task.AssignedAgent)
	}

	ptyEnv := []string{
		"SSH_CONNECTION=127.0.0.1 22 127.0.0.1 22",
		"SSH_CLIENT=127.0.0.1 22 22",
		"SSH_TTY=/dev/pts/0",
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	}
	for k, v := range env {
		ptyEnv = append(ptyEnv, fmt.Sprintf("%s=%s", k, v))
	}

	proc, err := terminal.SpawnPTY(execCtx, terminal.PTYConfig{
		TaskID:      t.ID,
		WorkDir:     workingDir,
		Shell:       agyBin,
		Args:        ptyArgs,
		Env:         ptyEnv,
		InitialCols: 80,
		InitialRows: 24,
	})
	if err != nil {
		return nil, fmt.Errorf("spawn pty: %w", err)
	}
	defer proc.Close()

	sessionID := "runner-sess-" + att.ID
	sess := terminal.NewSession(sessionID, t.ID, proc, terminal.ModeInteractive)
	defer sess.Close()

	// Expose Unix domain socket bridge for both task ID and attempt ID
	bridgeTask, _ := terminal.NewSocketBridge(sess, terminal.SocketPath(t.ID))
	if bridgeTask != nil {
		defer bridgeTask.Close()
	}
	bridgeAtt, _ := terminal.NewSocketBridge(sess, terminal.SocketPath(att.ID))
	if bridgeAtt != nil {
		defer bridgeAtt.Close()
	}

	// Register steering function
	steerFunc := func(steerPrompt string) error {
		_, wErr := sess.WriteInput("steer", []byte(steerPrompt+"\n"))
		return wErr
	}

	r.mu.Lock()
	var steerCh <-chan string
	if act, ok := r.activeAttempts[att.ID]; ok {
		act.steerFunc = steerFunc
		steerCh = act.steerChan
	}
	r.mu.Unlock()

	if steerCh != nil {
		go func() {
			for {
				select {
				case <-execCtx.Done():
					return
				case p, ok := <-steerCh:
					if !ok {
						return
					}
					_ = steerFunc(p)
				}
			}
		}()
	}

	// Subscribe logger to feed database logs in real time
	logSubID := "logger-" + att.ID
	logSub, _, _ := sess.Subscribe(logSubID, terminal.ModeWatch)
	defer sess.Unsubscribe(logSubID)

	var logBuf bytes.Buffer
	var logMu sync.Mutex
	go func() {
		for {
			select {
			case <-execCtx.Done():
				return
			case chunk, ok := <-logSub.SendChan:
				if !ok {
					return
				}
				logMu.Lock()
				logBuf.Write(chunk)
				for {
					line, lErr := logBuf.ReadString('\n')
					if len(line) > 0 {
						clean := strings.TrimRight(line, "\r\n")
						if clean != "" {
							atomic.AddInt64(liveStreamedCount, 1)
							r.streamLogLine(ctx, att.ID, "stdout", clean)
						}
					}
					if lErr != nil {
						break
					}
				}
				logMu.Unlock()
			}
		}
	}()

	// Monitor agent task report completion in background
	reportCheckTicker := time.NewTicker(500 * time.Millisecond)
	defer reportCheckTicker.Stop()

	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		for {
			select {
			case <-execCtx.Done():
				return
			case <-reportCheckTicker.C:
				if r.confirmAgentReport(ctx, claim) {
					// Agent has completed task report, flush output and stop
					time.Sleep(1500 * time.Millisecond)
					_ = sess.Close()
					return
				}
			}
		}
	}()

	// Wait for PTY process completion or context cancellation
	waitErr := proc.Wait()
	exitCode := proc.ExitCode()

	res := &agent.ExecutionResult{
		Status:   "SUCCESS",
		ExitCode: exitCode,
		Duration: timeoutDuration,
	}
	if exitCode != 0 {
		res.Status = "FAILED"
		res.Error = fmt.Sprintf("exit code %d", exitCode)
	}
	if waitErr != nil && errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		res.Status = "TIMEOUT"
		res.Error = "execution timed out"
	} else if waitErr != nil && errors.Is(execCtx.Err(), context.Canceled) {
		res.Status = "CANCELED"
		res.Error = "execution canceled"
	}

	return res, nil
}

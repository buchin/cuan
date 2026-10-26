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
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

type Runner struct {
	cfg        *config.Config
	adapter    agent.Adapter
	gitClient  *git.Client
	httpClient *http.Client
	baseURL    string

	mu             sync.Mutex
	currentAttempt *task.TaskAttempt
	cancelAttempt  context.CancelFunc
}

func NewRunner(cfg *config.Config, adapter agent.Adapter, customHTTPClient ...*http.Client) *Runner {
	client := http.DefaultClient
	if len(customHTTPClient) > 0 && customHTTPClient[0] != nil {
		client = customHTTPClient[0]
	}

	baseURL := strings.TrimRight(cfg.RunnerServerURL, "/")

	return &Runner{
		cfg:        cfg,
		adapter:    adapter,
		gitClient:  git.NewClient(),
		httpClient: client,
		baseURL:    baseURL,
	}
}

func (r *Runner) SetGitClient(client *git.Client) {
	r.gitClient = client
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
			log.Printf("[Runner %s] Shutdown signaled, terminating active work...", r.cfg.RunnerID)
			r.cancelCurrentExecution()
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
	attemptID := ""
	if r.currentAttempt != nil {
		attemptID = r.currentAttempt.ID
	}
	r.mu.Unlock()

	reqPayload := HeartbeatRequest{
		RunnerID:         r.cfg.RunnerID,
		Hostname:         hostname,
		SupportedAgents:  []string{r.adapter.Name()},
		CurrentAttemptID: attemptID,
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
			if hbResp.ShouldCancel {
				log.Printf("[Runner %s] Heartbeat received cancellation directive for active attempt %s", r.cfg.RunnerID, attemptID)
				r.cancelCurrentExecution()
			}
		}
	}
}

func (r *Runner) pollAndExecute(ctx context.Context) {
	// CONCURRENCY LIMIT: if currently busy executing, do not claim another task!
	r.mu.Lock()
	if r.currentAttempt != nil {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	claimReq := ClaimTaskRequest{
		RunnerID:        r.cfg.RunnerID,
		SupportedAgents: []string{r.adapter.Name()},
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

	log.Printf("[Runner %s] Claimed task attempt %s (Task: '%s')", r.cfg.RunnerID, claimResp.Attempt.ID, claimResp.Task.Title)

	execCtx, cancelFunc := context.WithCancel(ctx)
	r.mu.Lock()
	r.currentAttempt = claimResp.Attempt
	r.cancelAttempt = cancelFunc
	r.mu.Unlock()

	// Execute attempt in background goroutine so heartbeat ticker continues running
	go func(claim ClaimTaskResponse) {
		defer func() {
			r.mu.Lock()
			r.currentAttempt = nil
			r.cancelAttempt = nil
			r.mu.Unlock()
		}()

		r.runAttempt(execCtx, claim)
	}(claimResp)
}

func (r *Runner) runAttempt(ctx context.Context, claim ClaimTaskResponse) {
	att := claim.Attempt
	t := claim.Task
	startTime := time.Now()

	// Stream initial start log
	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Runner '%s' memulai eksekusi attempt %s dengan agent '%s'.", r.cfg.RunnerID, att.ID, r.adapter.Name()))

	// Prepare isolated workspace per task
	var ws *git.TaskWorkspace
	workingDir := ""
	if r.gitClient != nil {
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
	}

	// Prepare execution request
	prompt := t.Instructions
	if prompt == "" {
		prompt = t.Title
	}
	if claim.AgentPrompt != "" {
		prompt += claim.AgentPrompt
	}
	env := map[string]string{}
	if claim.TaskAgent != nil {
		encoded, _ := json.Marshal(claim.TaskAgent)
		env["CUAN_TASK_CAPABILITY_JSON"] = string(encoded)
	}

	req := agent.ExecutionRequest{
		TaskID:       t.ID,
		Prompt:       prompt,
		WorkingDir:   workingDir,
		Env:          env,
		Timeout:      10 * time.Minute,
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
			compReq.ErrorMessage = "Execution timed out"
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

	// Post-execution Git & GitHub PR Flow
	if compReq.Status == task.AttemptStatusSucceeded && ws != nil && r.gitClient != nil {
		defaultBranch := claim.DefaultBranch
		if defaultBranch == "" {
			defaultBranch = "main"
		}

		// 1. Commit changes with configured human author identity
		commitSHA, commitErr := r.gitClient.CommitChanges(ctx, ws.Path, fmt.Sprintf("feat(%s): %s", t.ID, t.Title), r.cfg.GitAuthorName, r.cfg.GitAuthorEmail)
		if commitErr == nil && commitSHA != "" {
			compReq.CommitSHA = commitSHA
			compReq.BranchName = ws.BranchName
			r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Commit created: %s by %s <%s>", commitSHA, r.cfg.GitAuthorName, r.cfg.GitAuthorEmail))
		}

		// 2. Diff summary
		diffSummary, _ := r.gitClient.GetDiffSummary(ctx, ws.Path, defaultBranch)
		compReq.DiffSummary = diffSummary
		if compReq.TestSummary == "" {
			compReq.TestSummary = "No project verifier was available; agent report evidence retained."
		}

		// 3. Push branch & generate/update Pull Request if remote repo info is present
		if claim.RepoOwner != "" && claim.RepoName != "" {
			token := os.Getenv("GH_TOKEN")
			if token == "" {
				token = os.Getenv("GITHUB_TOKEN")
			}

			pushErr := r.gitClient.PushTaskBranch(ctx, ws.Path, ws.BranchName, defaultBranch, token)
			if pushErr != nil {
				r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Catatan git push: %v", pushErr))
			} else {
				r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Task branch '%s' pushed to GitHub.", ws.BranchName))

				// Create or Update Pull Request (no duplicate PR on revision)
				prRes, prErr := r.gitClient.CreateOrUpdatePullRequest(ctx, git.PullRequestRequest{
					Owner:         claim.RepoOwner,
					Repo:          claim.RepoName,
					Title:         fmt.Sprintf("Task %s: %s", t.ID, t.Title),
					HeadBranch:    ws.BranchName,
					BaseBranch:    defaultBranch,
					Description:   t.Instructions,
					DiffSummary:   diffSummary,
					Verification:  "Run targeted automated tests:\n```bash\n./scripts/test.sh --fast\n```",
					TestStatus:    compReq.TestSummary,
					IsRevision:    att.AttemptNumber > 1,
					RevisionNotes: fmt.Sprintf("Revision cycle on attempt #%d", att.AttemptNumber),
					Token:         token,
				})
				if prErr == nil && prRes != nil {
					compReq.PRURL = prRes.HTMLURL
					compReq.PRNumber = prRes.Number
					actionWord := "created"
					if prRes.IsUpdated {
						actionWord = "diperbarui (siklus revisi)"
					}
					r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Pull Request #%d %s di GitHub: %s", prRes.Number, actionWord, prRes.HTMLURL))
				} else if prErr != nil {
					r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Catatan pembuatan PR: %v", prErr))
				}
			}
		}
	}

	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Attempt %s selesai: %s (exit: %d, durasi: %v, peak RSS: %d bytes).",
		att.ID, compReq.Status, compReq.ExitCode, duration.Round(time.Millisecond), compReq.PeakRSSBytes))

	// Send complete notification to server
	urlPath := fmt.Sprintf("/api/v1/runner/attempts/%s/complete", att.ID)
	_, _ = r.doRequest(context.Background(), http.MethodPost, urlPath, compReq)
	log.Printf("[Runner %s] Attempt %s finalized with status '%s' (exit %d)", r.cfg.RunnerID, att.ID, compReq.Status, compReq.ExitCode)
}

type verificationResult struct {
	Passed  bool
	Summary string
	Error   string
}

// runProjectVerification uses only repository-local, fixed commands. It never
// executes a command supplied by the agent or task text.
func runProjectVerification(parent context.Context, workingDir string) verificationResult {
	if workingDir == "" {
		return verificationResult{Passed: true, Summary: "No workspace was assigned; no project verifier available."}
	}
	command := ""
	args := []string{}
	testScript := filepath.Join(workingDir, "scripts", "test.sh")
	if info, err := os.Stat(testScript); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
		command, args = testScript, []string{"--fast"}
	} else if _, err := os.Stat(filepath.Join(workingDir, "go.mod")); err == nil {
		command, args = "go", []string{"test", "./..."}
	} else {
		return verificationResult{Passed: true, Summary: "No supported project verifier was available; agent report evidence retained."}
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workingDir
	output, err := cmd.CombinedOutput()
	const maxOutput = 12000
	if len(output) > maxOutput {
		output = output[len(output)-maxOutput:]
	}
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	summary := fmt.Sprintf("Project verification: %s %s (exit %d)\n%s", filepath.Base(command), strings.Join(args, " "), exitCode, strings.TrimSpace(string(output)))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return verificationResult{Passed: false, Summary: summary, Error: "Project verification timed out after 5 minutes"}
		}
		return verificationResult{Passed: false, Summary: summary, Error: "Project verification failed"}
	}
	return verificationResult{Passed: true, Summary: summary}
}

func (r *Runner) confirmAgentReport(ctx context.Context, claim ClaimTaskResponse) bool {
	if claim.TaskAgent == nil || claim.TaskAgent.API == "" || claim.TaskAgent.Token == "" || claim.Attempt == nil {
		return false
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, claim.TaskAgent.API, nil)
	if err != nil {
		return false
	}
	request.Header.Set("Authorization", "Bearer "+claim.TaskAgent.Token)
	response, err := r.httpClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var snapshot struct {
		Summary      string `json:"summary"`
		CompleteNote string `json:"complete_note"`
		Activities   []struct {
			AttemptID    *string `json:"attempt_id"`
			ActivityType string  `json:"activity_type"`
			NewStatus    string  `json:"new_status"`
		} `json:"activities"`
	}
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		return false
	}
	if strings.TrimSpace(snapshot.Summary) == "" || strings.TrimSpace(snapshot.CompleteNote) == "" {
		return false
	}
	for i := len(snapshot.Activities) - 1; i >= 0; i-- {
		activity := snapshot.Activities[i]
		if activity.AttemptID != nil && *activity.AttemptID == claim.Attempt.ID && activity.ActivityType == "agent_report" {
			return activity.NewStatus == "review" || activity.NewStatus == "done"
		}
	}
	return false
}

func (r *Runner) confirmAutoDeployCompliance(ctx context.Context, claim ClaimTaskResponse) (bool, string) {
	if !claim.AutoDeploy {
		return true, ""
	}
	if claim.TaskAgent == nil || claim.TaskAgent.API == "" || claim.TaskAgent.Token == "" {
		return false, "Kepatuhan auto_deploy: sesi task tidak memiliki kapabilitas agent untuk memvalidasi deployment live"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, claim.TaskAgent.API, nil)
	if err != nil {
		return false, "Kepatuhan auto_deploy: gagal membuat request verifikasi: " + err.Error()
	}
	request.Header.Set("Authorization", "Bearer "+claim.TaskAgent.Token)
	response, err := r.httpClient.Do(request)
	if err != nil {
		return false, "Kepatuhan auto_deploy: gagal mengakses API task: " + err.Error()
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("Kepatuhan auto_deploy: API task mengembalikan status %d", response.StatusCode)
	}
	var snapshot struct {
		Status       string `json:"status"`
		Summary      string `json:"summary"`
		CompleteNote string `json:"complete_note"`
		Activities   []struct {
			ActivityType string `json:"activity_type"`
			NewStatus    string `json:"new_status"`
			Message      string `json:"message"`
		} `json:"activities"`
	}
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		return false, "Kepatuhan auto_deploy: gagal decode snapshot task: " + err.Error()
	}
	combined := strings.ToLower(snapshot.CompleteNote + " " + snapshot.Summary)
	hasDeployProof := (strings.Contains(combined, "deploy") || strings.Contains(combined, "rilis") || strings.Contains(combined, "release")) &&
		(strings.Contains(combined, "live") || strings.Contains(combined, "production") || strings.Contains(combined, "healthz") || strings.Contains(combined, "smoke"))

	for _, a := range snapshot.Activities {
		msg := strings.ToLower(a.Message)
		if strings.Contains(msg, "auto_deploy") || strings.Contains(msg, "live deployment") || strings.Contains(msg, "release") {
			hasDeployProof = true
			break
		}
	}

	if !hasDeployProof {
		return false, "Kepatuhan auto_deploy: runner wajib memastikan perubahan kode telah di-deploy dan terverifikasi live di production sebelum menyatakan task selesai; commit/push lokal saja tidak memenuhi syarat"
	}
	return true, ""
}

func (r *Runner) streamOutputLogs(ctx context.Context, attemptID, stdout, stderr string) {
	var records []task.LogRecord
	now := time.Now().UTC()

	if stdout != "" {
		lines := strings.Split(stdout, "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				records = append(records, task.LogRecord{
					Stream:   "stdout",
					Line:     l,
					LoggedAt: now,
				})
			}
		}
	}

	if stderr != "" {
		lines := strings.Split(stderr, "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				records = append(records, task.LogRecord{
					Stream:   "stderr",
					Line:     l,
					LoggedAt: now,
				})
			}
		}
	}

	// Bound logs to RunnerMaxLogLines
	maxLines := r.cfg.RunnerMaxLogLines
	if maxLines <= 0 {
		maxLines = 5000
	}
	if len(records) > maxLines {
		truncatedMsg := fmt.Sprintf("[LOG TRUNCATED: log output exceeded %d lines]", maxLines)
		records = append(records[:maxLines], task.LogRecord{
			Stream:   "system",
			Line:     truncatedMsg,
			LoggedAt: now,
		})
	}

	if len(records) == 0 {
		return
	}

	// Stream to server
	payload := StreamLogsRequest{
		RunnerID:  r.cfg.RunnerID,
		AttemptID: attemptID,
		Logs:      records,
	}

	urlPath := fmt.Sprintf("/api/v1/runner/attempts/%s/logs", attemptID)
	_, _ = r.doRequest(ctx, http.MethodPost, urlPath, payload)
}

func (r *Runner) streamLogLine(ctx context.Context, attemptID, stream, line string) {
	payload := StreamLogsRequest{
		RunnerID:  r.cfg.RunnerID,
		AttemptID: attemptID,
		Logs: []task.LogRecord{
			{
				Stream:   stream,
				Line:     line,
				LoggedAt: time.Now().UTC(),
			},
		},
	}
	urlPath := fmt.Sprintf("/api/v1/runner/attempts/%s/logs", attemptID)
	_, _ = r.doRequest(ctx, http.MethodPost, urlPath, payload)
}

func (r *Runner) cancelCurrentExecution() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancelAttempt != nil {
		r.cancelAttempt()
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

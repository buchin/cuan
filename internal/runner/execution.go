package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

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

func (r *Runner) postExecutionGitFlow(ctx context.Context, ws *git.TaskWorkspace, t *task.Task, att *task.TaskAttempt, claim ClaimTaskResponse, compReq *CompleteAttemptRequest, lane string) {
	if lane != task.ExecutionCategoryCode || compReq.Status != task.AttemptStatusSucceeded || ws == nil || r.gitClient == nil {
		if lane != task.ExecutionCategoryCode {
			if compReq.TestSummary == "" {
				compReq.TestSummary = fmt.Sprintf("Lane '%s' executed operational/read-only task without modifying git deployment branch.", lane)
			}
			r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Lane '%s' selesai: git branch deployment dan PR tidak dimodifikasi.", lane))
		}
		return
	}

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

func (r *Runner) processCompletionEventAndDoD(ctx context.Context, ws *git.TaskWorkspace, t *task.Task, att *task.TaskAttempt, claim ClaimTaskResponse, compReq *CompleteAttemptRequest, lane, workingDir string) {
	// 1. Emisi internal event task:attempt_succeeded beserta metadata attempt & status
	evt := AttemptSucceededEvent{
		EventID:       fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		EventType:     EventTaskAttemptSucceeded,
		TaskID:        t.ID,
		AttemptID:     att.ID,
		AttemptNumber: att.AttemptNumber,
		WorkingDir:    workingDir,
		BranchName:    compReq.BranchName,
		CommitSHA:     compReq.CommitSHA,
		ExecutionLane: lane,
		Status:        compReq.Status,
		Summary:       t.Summary,
		CompleteNote:  t.CompleteNote,
		EmittedAt:     time.Now().UTC(),
		Metadata: map[string]any{
			"runner_id":      r.cfg.RunnerID,
			"exit_code":      compReq.ExitCode,
			"peak_rss_bytes": compReq.PeakRSSBytes,
			"diff_summary":   compReq.DiffSummary,
			"test_summary":   compReq.TestSummary,
		},
	}

	if r.eventBus != nil {
		r.eventBus.Publish(ctx, evt)
	} else {
		EmitAttemptSucceeded(ctx, evt)
	}

	compEvt := AttemptCompletedEvent{
		EventID:       evt.EventID,
		EventType:     evt.EventType,
		TaskID:        t.ID,
		AttemptID:     att.ID,
		AttemptNumber: att.AttemptNumber,
		RunnerID:      r.cfg.RunnerID,
		ExitCode:      compReq.ExitCode,
		WorkingDir:    workingDir,
		BranchName:    compReq.BranchName,
		CommitSHA:     compReq.CommitSHA,
		ExecutionLane: lane,
		Status:        compReq.Status,
		Summary:       t.Summary,
		CompleteNote:  t.CompleteNote,
		ErrorMessage:  att.ErrorMessage,
		SourceRoomID:  t.SourceRoomID,
		EmittedAt:     evt.EmittedAt,
		Metadata:      evt.Metadata,
	}
	if r.eventBus != nil {
		r.eventBus.PublishCompleted(ctx, compEvt)
	} else {
		EmitAttemptCompleted(ctx, compEvt)
	}

	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Event '%s' dipancarkan untuk attempt %s (Task: '%s').",
		EventTaskAttemptSucceeded, att.ID, t.Title))

	// 2. Background Autonomous DoD Evaluator validation
	if r.dodEvaluator != nil {
		evalReq := DoDValidationRequest{
			TaskID:        t.ID,
			AttemptID:     att.ID,
			AttemptNumber: att.AttemptNumber,
			WorkingDir:    workingDir,
			TaskTitle:     t.Title,
			Instructions:  t.Instructions,
			Summary:       t.Summary,
			CompleteNote:  t.CompleteNote,
			ExecutionLane: lane,
			Metadata:      evt.Metadata,
		}

		evalRes, err := r.dodEvaluator.EvaluateDoD(ctx, evalReq)
		if err == nil && evalRes != nil {
			compReq.DoDValidation = evalRes
			if evalRes.Passed {
				r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Autonomous DoD Evaluator: LULUS 100%% (%v). Clean tree: %t, Tests: %t, Build: %t, Lean: %t, Docs/LP: %t.",
					evalRes.Duration.Round(time.Millisecond), evalRes.CleanWorkingTree, evalRes.TargetedTestsPass, evalRes.BuildZeroError, evalRes.LeanStandardPass, evalRes.DocsAndLPPass))
			} else {
				r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Autonomous DoD Evaluator: Ditemukan %d gap. Tindakan: %s.",
					len(evalRes.Gaps), evalRes.Action))
				if evalRes.Action == "auto_requeue" {
					compReq.RequeueFeedback = evalRes.RequeueInstructions
					r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Auto-Requeue: Task dijadwalkan ulang ke attempt #%d dengan timeout adaptif %v.",
						evalRes.NextAttemptNumber, evalRes.NextTimeout))
				}
			}
		} else if err != nil {
			r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Catatan evaluasi DoD otonom: %v", err))
		}
	}
}

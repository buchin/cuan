package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/buchin/cuan/internal/executionpolicy"
	"io"
	"net/http"
	"net/url"
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
func runProjectVerification(parent context.Context, workingDir string, defaultBranch ...string) verificationResult {
	if len(defaultBranch) > 0 {
		if result := verifyDocumentationChanges(parent, workingDir, defaultBranch[0]); result != nil {
			return *result
		}
	}
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
	if claim.TaskAgent == nil || claim.TaskAgent.API == "" || claim.TaskAgent.Token == "" || claim.Attempt == nil || claim.Task == nil {
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
	if (response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden) && r.cfg.RunnerToken != "" {
		// A restarted server can lose its ephemeral agent grant. The worker may
		// read only its active attempt report through the existing runner identity.
		response.Body.Close()
		query := url.Values{"task_id": {claim.Task.ID}, "runner_id": {r.cfg.RunnerID}}
		response, err = r.doRequest(ctx, http.MethodGet, "/api/v1/runner/attempts/"+url.PathEscape(claim.Attempt.ID)+"/report?"+query.Encode(), nil)
		if err != nil {
			return false
		}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var snapshot struct {
		Checklist    []task.DoDItem `json:"dod_checklist"`
		Summary      string         `json:"summary"`
		CompleteNote string         `json:"complete_note"`
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
			if activity.NewStatus != "review" && activity.NewStatus != "done" {
				return false
			}
			claim.Task.Summary = snapshot.Summary
			claim.Task.CompleteNote = snapshot.CompleteNote
			claim.Task.DoDChecklist = snapshot.Checklist
			return true
		}
	}
	return false
}

func (r *Runner) confirmAutoDeployCompliance(ctx context.Context, claim ClaimTaskResponse, hasCodeChanges bool, expectedCommit string) (bool, string) {
	if !claim.AutoDeploy {
		return true, ""
	}
	if claim.Task == nil || claim.Attempt == nil || claim.TaskAgent == nil || claim.TaskAgent.API == "" || claim.TaskAgent.Token == "" {
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
	if (response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden) && r.cfg.RunnerToken != "" {
		response.Body.Close()
		query := url.Values{"task_id": {claim.Task.ID}, "runner_id": {r.cfg.RunnerID}}
		response, err = r.doRequest(ctx, http.MethodGet, "/api/v1/runner/attempts/"+url.PathEscape(claim.Attempt.ID)+"/report?"+query.Encode(), nil)
		if err != nil {
			return false, "Kepatuhan auto_deploy: attempt report is unavailable"
		}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("Kepatuhan auto_deploy: API task mengembalikan status %d", response.StatusCode)
	}
	var snapshot struct {
		Summary      string               `json:"summary"`
		CompleteNote string               `json:"complete_note"`
		Activities   []*task.TaskActivity `json:"activities"`
	}
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		return false, "Kepatuhan auto_deploy: gagal decode snapshot task: " + err.Error()
	}

	// Smart Compliance Gate: If there are NO code modifications in the repo
	// (investigation, RCA, data clarification, third-party outage),
	// allow completion without enforcing fake live deployment.
	if !hasCodeChanges {
		if strings.TrimSpace(snapshot.Summary) == "" && strings.TrimSpace(snapshot.CompleteNote) == "" {
			return false, "Kepatuhan investigasi: tugas investigasi wajib menyertakan ringkasan temuan dan draf solusi"
		}
		return true, ""
	}

	if !task.VerifiedDeployment(snapshot.Activities, claim.Attempt, expectedCommit, time.Now().UTC()) {
		return false, "Kepatuhan auto_deploy: verified production deployment receipt is missing for this exact task, attempt and repository commit; report text is not deployment evidence"
	}
	return true, ""
}

// detectRepoCodeChanges distinguishes runtime changes from a verified Markdown
// deliverable. Original task acceptance/publication criteria still apply.
func workspaceComparisonBase(ws *git.TaskWorkspace, fallback string) string {
	if ws != nil && ws.BaseCommit != "" {
		return ws.BaseCommit
	}
	if ws != nil && ws.BaseRef != "" {
		return ws.BaseRef
	}
	return fallback
}

func (r *Runner) detectRepoCodeChanges(ctx context.Context, ws *git.TaskWorkspace, defaultBranch string) (bool, string, error) {
	if ws == nil || r.gitClient == nil || ws.Path == "" {
		return false, "", nil
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	cmdStatus := exec.CommandContext(ctx, "git", "-C", ws.Path, "status", "--porcelain")
	statusOut, err := cmdStatus.Output()
	if err != nil {
		return false, "", fmt.Errorf("repository worktree state could not be verified: %w", err)
	}
	hasUncommitted := len(strings.TrimSpace(string(statusOut))) > 0

	diffSummary, err := r.gitClient.GetDiffSummary(ctx, ws.Path, workspaceComparisonBase(ws, defaultBranch))
	if err != nil {
		return false, "", fmt.Errorf("pinned repository baseline comparison could not be verified: %w", err)
	}
	hasCommittedDiff := len(strings.TrimSpace(diffSummary)) > 0
	if hasUncommitted || hasCommittedDiff {
		if documentation := verifyDocumentationChanges(ctx, ws.Path, workspaceComparisonBase(ws, defaultBranch)); documentation != nil && documentation.Passed {
			return false, diffSummary, nil
		}
	}

	return (hasUncommitted || hasCommittedDiff), diffSummary, nil
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
	if response, err := r.doRequest(ctx, http.MethodPost, urlPath, payload); err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		response.Body.Close()
	}
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
	if response, err := r.doRequest(ctx, http.MethodPost, urlPath, payload); err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		response.Body.Close()
	}
}

func (r *Runner) postExecutionGitFlow(ctx context.Context, ws *git.TaskWorkspace, t *task.Task, att *task.TaskAttempt, claim ClaimTaskResponse, compReq *CompleteAttemptRequest, lane string) {
	if executionpolicy.FromInstructions(t.Instructions).Restrict(claim.PublicationLimits).DenyPush {
		r.streamLogLine(ctx, att.ID, "system", "Task scope forbids publication; local files retained without automatic commit, push or PR.")
		return
	}
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
	diffSummary, _ := r.gitClient.GetDiffSummary(ctx, ws.Path, workspaceComparisonBase(ws, defaultBranch))
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
			// Fallback: if runner host has no GH_TOKEN, request server push using project's encrypted token
			if claim.TaskAgent != nil && claim.TaskAgent.API != "" && claim.TaskAgent.Token != "" {
				pushEndpoint := strings.TrimRight(claim.TaskAgent.API, "/") + "/push"
				pushReq, httpErr := http.NewRequestWithContext(ctx, http.MethodPost, pushEndpoint, nil)
				if httpErr == nil {
					pushReq.Header.Set("Authorization", "Bearer "+claim.TaskAgent.Token)
					pushResp, doErr := r.httpClient.Do(pushReq)
					if doErr == nil {
						defer pushResp.Body.Close()
						if pushResp.StatusCode == http.StatusOK {
							pushErr = nil
							r.streamLogLine(ctx, att.ID, "system", "Task branch pushed to GitHub via project connection.")
						} else {
							pushErr = fmt.Errorf("project push returned status %d", pushResp.StatusCode)
						}
					}
				}
			}
		}

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
			RepositoryReadOnlyVerified: compReq.RepositoryVerification.Unchanged() && executionpolicy.FromInstructions(t.Instructions).Restrict(claim.PublicationLimits).ReadOnlyRepository,
			Checklist:                  t.DoDChecklist, TaskID: t.ID,
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
				r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Acceptance evidence gate passed (%v). Workspace checks (false may mean not checked): clean tree: %t, Tests: %t, Build: %t, Lean: %t, Docs/LP: %t.",
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

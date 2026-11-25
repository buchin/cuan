package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
)

func (s *Server) runnerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Runner-Token")
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				token = strings.TrimSpace(authHeader[7:])
			}
		}

		if token != "" {
			if token == s.cfg.RunnerToken || token == s.cfg.AuthSecret {
				next.ServeHTTP(w, r)
				return
			}
			if owner, err := s.authSvc.ValidateToken(r.Context(), token); err == nil && owner != nil {
				next.ServeHTTP(w, r)
				return
			}
		}

		respondJSONError(w, http.StatusUnauthorized, "unauthorized: runner token is invalid or missing")
	})
}

func (s *Server) handleRunnerHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req runner.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.runnerSvc.Heartbeat(r.Context(), req, s.cfg.RunnerLeaseDuration)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("heartbeat failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRunnerClaim(w http.ResponseWriter, r *http.Request) {
	var req runner.ClaimTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.runnerSvc.Claim(r.Context(), req, s.cfg.RunnerLeaseDuration)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("claim failed: %v", err))
		return
	}

	if resp.Claimed && resp.NativeRelease != nil {
		// No model capability, visual recipe or generic project workspace may
		// be issued to a native stage. The native execution path prepares its
		// own exact source; it remains disabled until that path is wired.
		resp.WorkingDir, resp.Environment, resp.AgentPrompt = "", "", ""
		resp.AutoDeploy = false
		resp.OwnerID = resp.NativeRelease.OwnerID
		respondJSON(w, http.StatusOK, resp)
		return
	}
	if resp.Claimed && resp.Verification != nil && resp.Verification.FinalizeAttemptID != "" {
		respondJSON(w, http.StatusOK, resp)
		return
	}

	if resp.Claimed && resp.Task != nil && resp.Attempt != nil {
		resp.OwnerID, _ = s.authSvc.OwnerID(r.Context())
		claimRevision := ""
		detail, detailErr := s.taskSvc.GetTaskDetail(r.Context(), resp.Task.ID)
		if detailErr != nil && resp.LiveInspection != nil {
			_, _ = s.runnerSvc.Complete(r.Context(), resp.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "Inspection task context could not be verified; no model execution was started"})
			respondJSONError(w, http.StatusServiceUnavailable, "Inspection task context is unavailable")
			return
		}
		if detailErr == nil {
			revision := ""
			if launches, ok := s.taskSvc.Store().(task.AttemptLaunchStore); ok {
				launch, launchErr := launches.LaunchForAttempt(r.Context(), resp.Attempt.ID)
				if launchErr != nil {
					respondJSONError(w, http.StatusInternalServerError, "claim launch unavailable")
					return
				}
				if launch != nil {
					revision = launch.Revision
					claimRevision = launch.Revision
				}
			}
			resp.RequiresVision = s.runnerClaimRequiresVision(r.Context(), resp, detail, revision)
			if resp.LiveInspection != nil {
				if !s.liveInspectionMatchesRelease(r.Context(), detail, resp.LiveInspection) {
					_, _ = s.runnerSvc.Complete(r.Context(), resp.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: task.StaleInspectionReleaseMessage})
					respondJSONError(w, http.StatusConflict, "Inspection no longer matches the running release")
					return
				}
				path, snapshotErr := s.projectSvc.VerificationSnapshot(r.Context(), detail.ProjectID, detail.ID, resp.LiveInspection.SnapshotSourceCommit(), resp.LiveInspection.TargetCommit)
				if snapshotErr != nil {
					_, _ = s.runnerSvc.Complete(r.Context(), resp.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "The prepared inspection snapshot changed or became unavailable; no model execution was started"})
					respondJSONError(w, http.StatusServiceUnavailable, "Inspection snapshot could not be verified")
					return
				}
				if recipeErr := s.bindRunnerVisualRecipe(r.Context(), resp, path, claimRevision); recipeErr != nil {
					_, _ = s.runnerSvc.Complete(r.Context(), resp.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "Registered visual recipe could not be bound; no model execution was started"})
					respondJSONError(w, http.StatusServiceUnavailable, "Visual verification recipe unavailable")
					return
				}

				resp.WorkingDir, resp.Environment, resp.AutoDeploy = path, "", false
				resp.OwnerID = resp.LiveInspection.OwnerID
			}
		}
		if executionpolicy.StandaloneArtifactWork(resp.Task.Instructions) {
			projectContext, contextErr := s.projectSvc.TaskContext(r.Context(), resp.Task.ProjectID, resp.Task.Instructions)
			if contextErr != nil {
				respondJSONError(w, 503, "The saved task context is unavailable")
				return
			}
			path, workspaceErr := s.projectSvc.TaskExecutionWorkspace(r.Context(), resp.Task.ID, resp.Task.Instructions, projectContext)
			if errors.Is(workspaceErr, project.ErrExecutionWorkspaceMissing) {
				path, workspaceErr = s.projectSvc.ReconstructTaskExecutionWorkspace(r.Context(), resp.Task.ID, projectContext)
			}
			if workspaceErr != nil {
				respondJSONError(w, 503, "The task output workspace could not be prepared")
				return
			}
			resp.WorkingDir, resp.Environment, resp.AutoDeploy = path, "", false
			resp.ProjectRepo, resp.RepoURL, resp.RepoOwner, resp.RepoName = "", "", "", ""
			resp.DefaultBranch, resp.DeployBranch = "", ""
		}
		claimCtx := r.Context()
		if resp.WorkingDir != "" {
			claimCtx = context.WithValue(claimCtx, taskAgentWorkspaceKey{}, resp.WorkingDir)
		}
		env, prompt, _, _, err := s.issueTaskAgent(claimCtx, resp.Task.ID, resp.Attempt.ID, resp.Attempt.AgentName, claimRevision)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("could not prepare task session: %v", err))
			return
		}
		if len(env) == 0 {
			respondJSONError(w, http.StatusInternalServerError, "could not prepare task capability")
			return
		}
		capPath := strings.TrimPrefix(env[0], "CUAN_TASK_CAPABILITY_FILE=")
		raw, err := os.ReadFile(capPath)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "could not read task capability")
			return
		}
		var cap taskcli.Capability
		if err := json.Unmarshal(raw, &cap); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "could not encode task capability")
			return
		}
		resp.TaskAgent = &runner.TaskAgentCapability{API: cap.API, Token: cap.Token, TaskID: cap.TaskID, AttemptID: cap.AttemptID, ArtifactsDir: cap.ArtifactsDir, ReportFile: cap.ReportFile}
		resp.AgentPrompt = prompt + s.runnerRecoveryContext(r.Context(), resp.Task.ID, resp.Attempt.ID)
		_ = os.Remove(capPath)
		_ = os.Remove(filepath.Dir(capPath))
	}

	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) runnerRecoveryContext(ctx context.Context, taskID, currentAttemptID string) string {
	detail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil || detail == nil {
		return ""
	}
	var b strings.Builder
	for _, attempt := range detail.Attempts {
		if attempt == nil || attempt.ID == currentAttemptID {
			continue
		}
		b.WriteString("\n\n## Previous attempt context\n")
		fmt.Fprintf(&b, "Attempt %s: status=%s", attempt.ID, attempt.Status)
		if attempt.ErrorMessage != "" {
			fmt.Fprintf(&b, "; error=%s", attempt.ErrorMessage)
		}
		b.WriteString("\n")
		previous := *detail
		previous.LatestAttempt = attempt
		b.WriteString(taskRecoveryContext(&previous))
		break
	}
	for i := len(detail.Activities) - 1; i >= 0; i-- {
		activity := detail.Activities[i]
		if activity == nil || activity.AttemptID == nil || *activity.AttemptID == currentAttemptID || activity.ActivityType != "agent_report" {
			continue
		}
		message := strings.TrimSpace(activity.Message)
		if len([]rune(message)) > 4000 {
			message = string([]rune(message)[:4000]) + "…"
		}
		b.WriteString("Previous agent report:\n")
		b.WriteString(message)
		b.WriteString("\nContinue from this evidence; re-check it before relying on it.\n")
		break
	}
	for i := len(detail.Activities) - 1; i >= 0; i-- {
		a := detail.Activities[i]
		if a != nil && strings.HasPrefix(a.Message, "Auto-Requeue:") {
			b.WriteString("\nSaved revision feedback (not new authorization):\n" + truncateProjectText(a.Message, 6000) + "\n")
			break
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func (s *Server) handleRunnerStreamLogs(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "id")
	var req runner.StreamLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.taskSvc.AppendAttemptLogs(r.Context(), attemptID, req.Logs, s.cfg.RunnerMaxLogLines); err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to append logs: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"appended": len(req.Logs),
	})
}

func (s *Server) handleRunnerGetStreamLogs(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "id")
	att, err := s.taskSvc.GetAttempt(r.Context(), attemptID)
	if err != nil || att == nil {
		respondJSONError(w, http.StatusNotFound, "Attempt not found")
		return
	}

	afterID, _ := strconv.ParseInt(r.URL.Query().Get("after_id"), 10, 64)
	if afterID == 0 {
		offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
		if offset > 0 {
			afterID = offset
		}
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}

	logs, err := s.taskSvc.GetAttemptLogs(r.Context(), attemptID, afterID, limit)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get logs: %v", err))
		return
	}
	if logs == nil {
		logs = []*task.LogRecord{}
	}

	nextOffset := afterID
	if len(logs) > 0 {
		nextOffset = logs[len(logs)-1].ID
	}

	isFinal := att.Status != task.AttemptStatusQueued && att.Status != task.AttemptStatusRunning

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"attempt_id": attemptID,
		"status":     att.Status,
		"is_final":   isFinal,
		"offset":     nextOffset,
		"total":      len(logs),
		"logs":       logs,
	})
}

func (s *Server) handleRunnerSteer(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "id")
	var req runner.SteerAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		respondJSONError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	if req.Author == "" {
		if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
			req.Author = owner.Email
		} else {
			req.Author = "user"
		}
	}

	if s.runnerSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "runner service not available")
		return
	}

	resp, err := s.runnerSvc.Steer(r.Context(), attemptID, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, resp)
}



func (s *Server) handleRunnerComplete(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "id")
	var req runner.CompleteAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	att, err := s.runnerSvc.Complete(r.Context(), attemptID, req)
	if err != nil {
		log.Printf("[runner-completion] attempt=%s runner=%s rejected: %v", attemptID, req.RunnerID, err)
		if errors.Is(err, task.ErrAttemptFinal) {
			saved, _ := s.taskSvc.GetAttempt(r.Context(), attemptID)
			respondJSON(w, http.StatusOK, map[string]any{"ignored": true, "attempt": saved})
			return
		}
		if errors.Is(err, task.ErrRequestConflict) || errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusConflict, "The execution context changed. Saved work was preserved; completion was not accepted.")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to complete attempt: %v", err))
		return
	}

	// Clean up only the completed task's terminal sessions; never reset the
	// project's interactive Project Terminal when a background task finishes.
	if att != nil {
		if s.terminalManager != nil {
			_ = s.terminalManager.CloseSession(att.TaskID)
			_ = s.terminalManager.CloseSession(att.ID)
		}
		if s.taskSvc != nil {
			_ = s.taskSvc.FinishTaskLaunches(r.Context(), att.TaskID, "stopped")
			s.kickTaskLaunches()
		}
	}

	// Trigger autonomous scrum engine for attempt completion
	s.onRunnerAttemptCompleted(r.Context(), att)

	respondJSON(w, http.StatusOK, att)
}

func (s *Server) handleListRunners(w http.ResponseWriter, r *http.Request) {
	_, _ = s.runnerSvc.ReconcileExpiredLeases(r.Context())

	runners, err := s.runnerSvc.ListRunners(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list runners: %v", err))
		return
	}
	if runners == nil {
		runners = []*runner.RunnerInfo{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"runners": runners,
		"total":   len(runners),
	})
}

func (s *Server) handleGetRunner(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rInfo, err := s.runnerSvc.GetRunner(r.Context(), id)
	if err != nil {
		if errors.Is(err, runner.ErrRunnerNotFound) {
			respondJSONError(w, http.StatusNotFound, "Runner not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get runner: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, rInfo)
}

func (s *Server) handleReconcileLeases(w http.ResponseWriter, r *http.Request) {
	interrupted, err := s.runnerSvc.ReconcileExpiredLeases(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("reconcile leases failed: %v", err))
		return
	}
	if interrupted == nil {
		interrupted = []*task.TaskAttempt{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"interrupted_count": len(interrupted),
		"attempts":          interrupted,
	})
}

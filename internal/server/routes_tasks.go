package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
)

// registerTaskRoutes mounts all task, attempt, and runner endpoints on the protected router.
func (s *Server) registerTaskRoutes(pr chi.Router) {
	// Tasks endpoints (T4 & T5)
	pr.Get("/tasks", s.handleListTasks)
	pr.Post("/tasks", s.handleCreateTask)
	pr.Get("/tasks/{id}", s.handleGetTask)
	pr.Put("/tasks/{id}", s.handleUpdateTask)
	pr.Delete("/tasks/{id}", s.handleDeleteTask)
	pr.Post("/tasks/{id}/comments", s.handleAddComment)
	pr.Post("/tasks/{id}/attempts", s.handleCreateAttempt)
	pr.Patch("/tasks/{id}/attempts/{attemptId}", s.handleUpdateAttempt)
	pr.Post("/tasks/{id}/cancel", s.handleCancelTask)
	pr.Post("/tasks/{id}/retry", s.handleRetryTask)
	pr.Post("/tasks/{id}/apply", s.handleApplyTask)
	pr.Get("/tasks/{id}/attempts/{attemptId}/logs", s.handleGetAttemptLogs)

	// Runners & Infrastructure (T5)
	pr.Get("/runners", s.handleListRunners)
	pr.Get("/runners/{id}", s.handleGetRunner)
	pr.Post("/runner/reconcile", s.handleReconcileLeases)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	var projectIDs []string
	if pids := r.URL.Query().Get("project_ids"); pids != "" {
		for _, id := range strings.Split(pids, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				projectIDs = append(projectIDs, id)
			}
		}
	}
	if qPids := r.URL.Query()["project_id"]; len(qPids) > 1 {
		for _, id := range qPids {
			id = strings.TrimSpace(id)
			if id != "" {
				projectIDs = append(projectIDs, id)
			}
		}
	}

	filter := task.TaskFilter{
		ProjectID:  r.URL.Query().Get("project_id"),
		ProjectIDs: projectIDs,
		Status:     r.URL.Query().Get("status"),
		Search:     r.URL.Query().Get("search"),
	}

	tasks, err := s.taskSvc.ListTasks(r.Context(), filter)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list tasks: %v", err))
		return
	}
	if tasks == nil {
		tasks = []*task.TaskItemResponse{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req task.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.ProjectID) == "" && len(req.ProjectIDs) == 0 {
		if s.projectSvc == nil {
			respondJSONError(w, 503, "Projects are unavailable")
			return
		}
		p, err := s.projectSvc.EnsureDefault(r.Context())
		if err != nil {
			respondJSONError(w, 503, "Default project is unavailable")
			return
		}
		if p.IsArchived {
			respondJSONError(w, 409, "Default project is archived. Choose an active project.")
			return
		}
		req.ProjectID = "default"
		req.ProjectIDs = []string{"default"}
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	created, err := s.taskSvc.CreateTask(r.Context(), author, req)
	if err != nil {
		if errors.Is(err, task.ErrRequestConflict) {
			respondJSONError(w, 409, err.Error())
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create task: %v", err))
		return
	}

	if s.taskProjectRepo != nil && len(req.ProjectIDs) > 0 {
		_ = s.taskProjectRepo.SetTaskProjects(r.Context(), created.ID, req.ProjectIDs)
	}

	respondJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := s.taskSvc.GetTaskDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			if s.projectStudioRepo != nil {
				if pt, ptErr := s.projectStudioRepo.GetTaskByID(r.Context(), id); ptErr == nil && pt != nil {
					respondJSON(w, http.StatusOK, map[string]interface{}{
						"task": pt.ToDTO(),
					})
					return
				}
			}
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get task: %v", err))
		return
	}

	if s.projectStudioRepo != nil && detail.SourceRoomID != "" && detail.LatestAttempt != nil {
		if m, err := s.projectStudioRepo.GetMessageByID(r.Context(), delegationMessageID("task-delivery:"+detail.LatestAttempt.ID)); err == nil && m != nil && m.RoomID == detail.SourceRoomID && m.AuthorType == "agent" && m.ReplyToID != nil && *m.ReplyToID == detail.SourceMessageID {
			_, ids := decodeAttachmentMessage(m.Content)
			if owner, err := s.taskArtifactOwner(r.Context(), detail); err == nil {
				for _, aid := range ids {
					if a, _, err := loadAttachment(owner, aid); err == nil {
						detail.Attachments = append(detail.Attachments, a)
					}
				}
			}
		}
	}

	respondJSON(w, http.StatusOK, detail)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req task.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	updated, err := s.taskSvc.UpdateTask(r.Context(), id, author, req)
	if err != nil {
		if errors.Is(err, task.ErrTaskActive) || errors.Is(err, task.ErrRequestConflict) {
			respondJSONError(w, 409, err.Error())
			return
		}
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update task: %v", err))
		return
	}

	if s.taskProjectRepo != nil && req.ProjectIDs != nil {
		_ = s.taskProjectRepo.SetTaskProjects(r.Context(), id, *req.ProjectIDs)
	}

	respondJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := s.taskSvc.DeleteTask(r.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskActive) {
			respondJSONError(w, 409, "Cancel the active session before deleting this task")
			return
		}
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete task: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Task deleted",
		"id":      id,
	})
}

func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req task.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	act, err := s.taskSvc.AddComment(r.Context(), id, author, req.Message)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to add comment: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, act)
}

func (s *Server) handleCreateAttempt(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req task.CreateAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	att, err := s.taskSvc.CreateAttempt(r.Context(), id, author, req)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create attempt: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, att)
}

func (s *Server) handleUpdateAttempt(w http.ResponseWriter, r *http.Request) {
	attemptId := chi.URLParam(r, "attemptId")
	var req task.UpdateAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if _, err := s.taskSvc.AttemptForTask(r.Context(), chi.URLParam(r, "id"), attemptId); err != nil {
		respondJSONError(w, http.StatusNotFound, "Attempt not found for this task")
		return
	}
	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	att, err := s.taskSvc.UpdateAttemptStatus(r.Context(), attemptId, author, req)
	if err != nil {
		if errors.Is(err, task.ErrAttemptNotFound) {
			respondJSONError(w, http.StatusNotFound, "Attempt not found")
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update attempt: %v", err))
		return
	}

	if att.Status != task.AttemptStatusRunning && att.Status != task.AttemptStatusQueued {
		if err := s.taskSvc.ReconcileLocalTask(r.Context(), att.TaskID, att.ID); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "Execution status saved; task status could not be refreshed. Retry the same update.")
			return
		}
	}
	respondJSON(w, http.StatusOK, att)
}

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

	if resp.Claimed && resp.Task != nil && resp.Task.ProjectID != "" {
		if p, err := s.projectSvc.Get(r.Context(), resp.Task.ProjectID); err == nil && p != nil {
			resp.ProjectRepo = p.RepoURL
			resp.WorkingDir = p.LocalPath
			settings := project.SettingsFromMetadata(p.Metadata)
			resp.AutoDeploy = settings.IsAutoDeployEnabled()
		}
	}
	if resp.Claimed && resp.Task != nil && resp.Attempt != nil {
		env, prompt, _, _, err := s.issueTaskAgent(r.Context(), resp.Task.ID, resp.Attempt.ID, resp.Attempt.AgentName)
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
		resp.TaskAgent = &runner.TaskAgentCapability{API: cap.API, Token: cap.Token, TaskID: cap.TaskID, AttemptID: cap.AttemptID}
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

func (s *Server) handleRunnerComplete(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "id")
	var req runner.CompleteAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	att, err := s.runnerSvc.Complete(r.Context(), attemptID, req)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to complete attempt: %v", err))
		return
	}

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

func (s *Server) handleGetAttemptLogs(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "attemptId")
	if _, err := s.taskSvc.AttemptForTask(r.Context(), chi.URLParam(r, "id"), attemptID); err != nil {
		respondJSONError(w, 404, "Attempt not found for this task")
		return
	}
	afterID, _ := strconv.ParseInt(r.URL.Query().Get("after_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
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

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": len(logs),
	})
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	cancelledLaunches, launchErr := s.taskSvc.CancelLaunches(r.Context(), taskID, time.Now().UTC())
	if launchErr != nil {
		respondJSONError(w, 503, "Could not save cancellation. Check the launch status before retrying.")
		return
	}
	att, err := s.taskSvc.CancelTask(r.Context(), taskID, author)
	if cancelledLaunches > 0 && (errors.Is(err, task.ErrAttemptNotFound) || errors.Is(err, task.ErrInvalidTaskInput)) {
		blocked := task.TaskStatusBlocked
		_, _ = s.taskSvc.UpdateTask(r.Context(), taskID, author, task.UpdateTaskRequest{Status: &blocked})
		if detail, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil {
			s.publishInbox(detail.SourceRoomID)
		}
		respondJSON(w, 200, map[string]string{"status": "cancelled"})
		return
	}
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		if errors.Is(err, task.ErrAttemptNotFound) {
			respondJSONError(w, http.StatusNotFound, "No attempt is available to cancel")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.terminalManager != nil {
		_ = s.terminalManager.CloseSession(taskID)
	}
	if s.taskSvc != nil {
		_ = s.taskSvc.FinishTaskLaunches(r.Context(), taskID, "cancelled")
		s.kickTaskLaunches()
	}
	if detail, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil && detail.SourceRoomID != "" {
		s.publishInbox(detail.SourceRoomID)
	}
	respondJSON(w, http.StatusOK, att)
}

func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	var req struct {
		RunnerID  string `json:"runner_id"`
		AgentName string `json:"agent_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	att, err := s.taskSvc.RetryTask(r.Context(), taskID, author, req.RunnerID, req.AgentName)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, att)
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

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
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
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
	pr.Patch("/tasks/{id}", s.handleUpdateTask)
	pr.Delete("/tasks/{id}", s.handleDeleteTask)
	pr.Post("/tasks/{id}/comments", s.handleAddComment)
	pr.Post("/tasks/{id}/attempts", s.handleCreateAttempt)
	pr.Patch("/tasks/{id}/attempts/{attemptId}", s.handleUpdateAttempt)
	pr.Post("/tasks/{id}/cancel", s.handleCancelTask)
	pr.Post("/tasks/{id}/retry", s.handleRetryTask)
	pr.Post("/tasks/{id}/apply", s.handleApplyTask)
	pr.Get("/tasks/{id}/attempts/{attemptId}/logs", s.handleGetAttemptLogs)
	pr.Post("/tasks/{id}/steer", s.handleTaskSteer)
	pr.Post("/tasks/{id}/files", s.handleUploadTaskFile)
	pr.Get("/tasks/{id}/files/raw", s.handleGetTaskWorkspaceFile)
	pr.Get("/tasks/{id}/usage", s.handleGetTaskUsage)
	pr.Get("/tasks/{id}/diff", s.handleGetTaskDiff)

	// Runners & Infrastructure (T5)
	pr.Get("/runners", s.handleListRunners)
	pr.Get("/runners/{id}", s.handleGetRunner)
	pr.Post("/runner/reconcile", s.handleReconcileLeases)
}

// registerTasksRootRoutes mounts task studio, terminal, and agent routes on the root router.
func (s *Server) registerTasksRootRoutes(r chi.Router) {
	// Project Tasks
	r.Post("/api/projects/{id}/tasks", s.handleCreateProjectTask)
	r.Get("/api/projects/{id}/tasks", s.handleListProjectTasks)
	r.Get("/api/projects/{id}/runner-state", s.handleGetProjectRunnerState)
	r.Get("/api/tasks/{id}", s.handleGetProjectTask)
	r.Put("/api/tasks/{id}", s.handleUpdateProjectTask)
	r.Patch("/api/tasks/{id}/status", s.handlePatchProjectTaskStatus)
	r.Delete("/api/tasks/{id}", s.handleDeleteProjectTask)

	// Project Tasks alias routes (/api/projects/tasks/{id})
	r.Get("/api/projects/tasks/{id}", s.handleGetProjectTask)
	r.Put("/api/projects/tasks/{id}", s.handleUpdateProjectTask)
	r.Patch("/api/projects/tasks/{id}/status", s.handlePatchProjectTaskStatus)
	r.Delete("/api/projects/tasks/{id}", s.handleDeleteProjectTask)

	// In-Task Agentic Execution Engine (Sprint 10 S10.3)
	r.Post("/api/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
	r.Post("/api/projects/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
	r.Post("/api/tasks/{id}/apply", s.handleApplyTask)
	r.Post("/api/projects/tasks/{id}/apply", s.handleApplyTask)

	// WebSocket Pseudo-Terminal (PTY) Engine (Sprint 9 S09.3)
	r.Get("/api/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
	r.Delete("/api/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
	r.Get("/api/projects/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
	r.Delete("/api/projects/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
	r.Get("/api/terminal/sessions", s.handleGetTerminalSessionsStatus)

	// Autonomous PTY Agent Task Runner (Sprint 22 S22.2)
	r.Post("/api/tasks/{id}/run-agent", s.handleRunTaskAgent)
	r.Post("/api/projects/tasks/{id}/run-agent", s.handleRunTaskAgent)
	r.Get("/api/tasks/{id}/run-agent/history", s.handleGetTaskRunHistory)
}

// registerTasksV1Routes mounts task studio, terminal, and agent routes inside /api/v1 router.
func (s *Server) registerTasksV1Routes(r chi.Router) {
	// Project Tasks in v1
	r.Post("/projects/{id}/tasks", s.handleCreateProjectTask)
	r.Get("/projects/{id}/tasks", s.handleListProjectTasks)
	r.Get("/projects/{id}/runner-state", s.handleGetProjectRunnerState)
	r.Get("/projects/tasks/{id}", s.handleGetProjectTask)
	r.Put("/projects/tasks/{id}", s.handleUpdateProjectTask)
	r.Patch("/projects/tasks/{id}/status", s.handlePatchProjectTaskStatus)
	r.Delete("/projects/tasks/{id}", s.handleDeleteProjectTask)
	r.Patch("/tasks/{id}/status", s.handlePatchProjectTaskStatus)

	// In-Task Agentic Execution Engine in v1 (Sprint 10 S10.3)
	r.Post("/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
	r.Post("/projects/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
	r.Post("/tasks/{id}/apply", s.handleApplyTask)
	r.Post("/projects/tasks/{id}/apply", s.handleApplyTask)

	// WebSocket Pseudo-Terminal (PTY) Engine in v1 (Sprint 9 S09.3)
	r.Get("/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
	r.Delete("/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
	r.Get("/projects/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
	r.Delete("/projects/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
	r.Get("/terminal/sessions", s.handleGetTerminalSessionsStatus)

	// Autonomous PTY Agent Task Runner (Sprint 22 S22.2)
	r.Post("/tasks/{id}/run-agent", s.handleRunTaskAgent)
	r.Post("/projects/tasks/{id}/run-agent", s.handleRunTaskAgent)
	r.Get("/tasks/{id}/run-agent/history", s.handleGetTaskRunHistory)

	// Workspace File Upload & Task Usage API (Sprint 43)
	r.Post("/api/tasks/{id}/files", s.handleUploadTaskFile)
	r.Get("/api/tasks/{id}/files/raw", s.handleGetTaskWorkspaceFile)
	r.Get("/api/tasks/{id}/usage", s.handleGetTaskUsage)
	r.Get("/api/tasks/{id}/diff", s.handleGetTaskDiff)
	r.Post("/tasks/{id}/files", s.handleUploadTaskFile)
	r.Get("/tasks/{id}/files/raw", s.handleGetTaskWorkspaceFile)
	r.Get("/tasks/{id}/usage", s.handleGetTaskUsage)
	r.Get("/tasks/{id}/diff", s.handleGetTaskDiff)
	r.Post("/api/v1/tasks/{id}/files", s.handleUploadTaskFile)
	r.Get("/api/v1/tasks/{id}/files/raw", s.handleGetTaskWorkspaceFile)
	r.Get("/api/v1/tasks/{id}/usage", s.handleGetTaskUsage)
	r.Get("/api/v1/tasks/{id}/diff", s.handleGetTaskDiff)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("view") == "board" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		result, err := s.taskSvc.ListTaskPage(r.Context(), task.PageOptions{ProjectID: r.URL.Query().Get("project_id"), Search: r.URL.Query().Get("search"), Status: r.URL.Query().Get("status"), Priority: r.URL.Query().Get("priority"), Sort: r.URL.Query().Get("sort"), Page: page, PageSize: size, ExcludeTerminals: r.URL.Query().Get("exclude_terminals") == "true"})
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "Could not load task page")
			return
		}
		respondJSON(w, http.StatusOK, result)
		return
	}

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

	if detail.SourceRoomID != "" && detail.LatestAttempt != nil && (detail.Status == task.TaskStatusReview || detail.Status == task.TaskStatusBlocked) {
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			requests, err := store.ListInboxRequests(r.Context(), detail.SourceRoomID)
			if err != nil {
				respondJSONError(w, 503, "Could not read task review progress")
				return
			}
			item := &task.TaskItemResponse{Task: detail.Task, LatestAttempt: detail.LatestAttempt}
			if err := s.projectInboxTaskReviews(r.Context(), []*task.TaskItemResponse{item}, requests, time.Now().UTC()); err != nil {
				respondJSONError(w, 503, "Could not read task review progress")
				return
			}
			detail.ReviewState = item.ReviewState
		}
	}
	if s.projectStudioRepo != nil && detail.SourceRoomID != "" && detail.LatestAttempt != nil {
		seen := map[string]bool{}
		for _, prefix := range []string{"task-visual-delivery:", "task-delivery:"} {
			m, err := s.projectStudioRepo.GetMessageByID(r.Context(), delegationMessageID(prefix+detail.LatestAttempt.ID))
			if err != nil || m == nil || m.RoomID != detail.SourceRoomID || m.AuthorType != "agent" || m.ReplyToID == nil || *m.ReplyToID != detail.SourceMessageID {
				continue
			}
			_, ids := decodeAttachmentMessage(m.Content)
			if owner, err := s.taskArtifactOwner(r.Context(), detail); err == nil {
				for _, aid := range ids {
					if !seen[aid] {
						if attachment, _, err := loadAttachment(owner, aid); err == nil {
							detail.Attachments = append(detail.Attachments, attachment)
							seen[aid] = true
						}
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

	if req.Status != nil {
		s.kickTaskLaunches()
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

	if s.taskSvc != nil {
		_ = s.taskSvc.FinishTaskLaunches(r.Context(), id, "stopped")
		s.kickTaskLaunches()
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
	if req.RunnerID == "" {
		req.RunnerID = "cuan-runner"
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

	if resp.Claimed && resp.Verification != nil && resp.Verification.FinalizeAttemptID != "" {
		respondJSON(w, http.StatusOK, resp)
		return
	}
	if resp.Claimed && resp.Task != nil && resp.Task.ProjectID != "" {
		if p, err := s.projectSvc.Get(r.Context(), resp.Task.ProjectID); err == nil && p != nil {
			resp.ProjectRepo = p.RepoURL
			resp.WorkingDir = p.LocalPath
			settings := project.SettingsFromMetadata(p.Metadata)
			resp.AutoDeploy = settings.IsAutoDeployEnabled() && !resp.PublicationLimits.DenyDeploy
			resp.DeployBranch = strings.TrimSpace(settings.DeployBranch)
			if resp.DeployBranch == "" {
				resp.DeployBranch = strings.TrimSpace(p.DefaultBranch)
			}
		}
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
			resp.RequiresVision = s.taskRequiresVision(r.Context(), resp.OwnerID, detail, "", revision)
			if resp.LiveInspection != nil {
				if !s.liveInspectionMatchesRelease(r.Context(), detail, resp.LiveInspection) {
					_, _ = s.runnerSvc.Complete(r.Context(), resp.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "The running release changed before inspection; no model execution was started"})
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
		resp.TaskAgent = &runner.TaskAgentCapability{API: cap.API, Token: cap.Token, TaskID: cap.TaskID, AttemptID: cap.AttemptID, ArtifactsDir: cap.ArtifactsDir}
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

func (s *Server) handleTaskSteer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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

	attemptID := id
	if s.taskSvc != nil {
		if detail, err := s.taskSvc.GetTaskDetail(r.Context(), id); err == nil && detail != nil {
			if detail.LatestAttempt != nil && detail.LatestAttempt.ID != "" {
				attemptID = detail.LatestAttempt.ID
			}
		}
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
	if req.RunnerID == "" {
		req.RunnerID = "cuan-runner"
	}

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

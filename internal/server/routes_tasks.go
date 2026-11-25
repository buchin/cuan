package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// registerTaskRoutes mounts all task, attempt, and runner endpoints on the protected router.
func (s *Server) registerTaskRoutes(pr chi.Router) {
	// Tasks endpoints (T4 & T5)
	pr.Get("/tasks", s.handleListTasks)
	pr.Post("/tasks", s.handleCreateTask)
	pr.Get("/tasks/{id}", s.handleGetTask)
	pr.Get("/tasks/{id}/execution", s.handleTaskExecutionInfo)
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
	r.Get("/api/projects/{id}/git-log", s.handleGetProjectGitLog)
	r.Get("/api/projects/{id}/activity", s.handleGetProjectGitLog)
	r.Get("/api/projects/{id}/commits/{sha}", s.handleGetProjectCommitDetail)
	r.Get("/projects/{id}/git-log", s.handleGetProjectGitLog)
	r.Get("/projects/{id}/activity", s.handleGetProjectGitLog)
	r.Get("/projects/{id}/commits/{sha}", s.handleGetProjectCommitDetail)
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
	r.Get("/api/terminal/active", s.handleGetActiveTerminalStatus)
	r.Delete("/api/terminal/active", s.handleResetActiveTerminalStatus)

	// Project Terminal Engine
	r.Get("/api/projects/{id}/terminal", s.handleProjectTerminalWebSocket)
	r.Delete("/api/projects/{id}/terminal", s.handleCloseProjectTerminalSession)
	r.Get("/api/projects/{id}/terminal/status", s.handleGetProjectTerminalStatus)
	r.Post("/api/projects/{id}/terminal/sync-upstream", s.handleProjectTerminalSyncUpstream)
	r.Post("/api/projects/{id}/terminal/rotate-brain", s.handleRotateProjectTerminalBrain)

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
	r.Get("/projects/{id}/git-log", s.handleGetProjectGitLog)
	r.Get("/projects/{id}/activity", s.handleGetProjectGitLog)
	r.Get("/projects/{id}/commits/{sha}", s.handleGetProjectCommitDetail)
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
	r.Get("/terminal/active", s.handleGetActiveTerminalStatus)
	r.Delete("/terminal/active", s.handleResetActiveTerminalStatus)

	// Project Terminal Engine in v1
	r.Get("/projects/{id}/terminal", s.handleProjectTerminalWebSocket)
	r.Delete("/projects/{id}/terminal", s.handleCloseProjectTerminalSession)
	r.Get("/projects/{id}/terminal/status", s.handleGetProjectTerminalStatus)
	r.Post("/projects/{id}/terminal/sync-upstream", s.handleProjectTerminalSyncUpstream)
	r.Post("/projects/{id}/terminal/rotate-brain", s.handleRotateProjectTerminalBrain)

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
		if err := s.projectTaskReviewFindings(r.Context(), result.Tasks); err != nil {
			respondJSONError(w, 503, "Could not read task review findings")
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

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && len(tasks) > limit {
			tasks = tasks[:limit]
		}
	}

	if err := s.projectTaskReviewFindings(r.Context(), tasks); err != nil {
		respondJSONError(w, 503, "Could not read task review findings")
		return
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
		projects, err := s.projectSvc.List(r.Context())
		if err != nil {
			respondJSONError(w, 503, "Projects are unavailable")
			return
		}
		var active []*project.ProjectResponse
		for _, p := range projects {
			if !p.IsArchived && p.ID != "default" && !strings.EqualFold(p.Name, "default") {
				active = append(active, p)
			}
		}
		if len(active) == 1 {
			req.ProjectID = active[0].ID
			req.ProjectIDs = []string{active[0].ID}
		} else if len(active) > 1 {
			cleanTitle := strings.TrimSpace(req.Title)
			if strings.HasPrefix(cleanTitle, "Terminal ") || strings.HasPrefix(cleanTitle, "Terminal·") || strings.HasPrefix(cleanTitle, "Terminal:") || strings.HasPrefix(cleanTitle, "Terminal-") || strings.Contains(cleanTitle, "Terminal ·") {
				chosen := active[0].ID
				for _, p := range active {
					if strings.EqualFold(p.Name, "cuan") {
						chosen = p.ID
						break
					}
				}
				req.ProjectID = chosen
				req.ProjectIDs = []string{chosen}
			} else {
				respondJSONError(w, 400, "Multiple active projects found. Choose an active project.")
				return
			}
		} else {
			p, err := s.projectSvc.EnsureDefault(r.Context())
			if err != nil {
				respondJSONError(w, 503, "Default project is unavailable")
				return
			}
			req.ProjectID = p.ID
			req.ProjectIDs = []string{p.ID}
		}
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

	s.projectInboxLaunchDiagnostics(r.Context(), detail)
	detail.ReviewFinding = currentTaskReviewFinding(detail)
	detail.ReviewAction = currentTaskReviewAction(detail)

	if detail.SourceRoomID != "" && detail.LatestAttempt != nil && (detail.Status == task.TaskStatusReview || detail.Status == task.TaskStatusBlocked) {
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			requests, err := store.ListInboxRequests(r.Context(), detail.SourceRoomID)
			if err != nil {
				respondJSONError(w, 503, "Could not read task review progress")
				return
			}
			item := &task.TaskItemResponse{Task: detail.Task, LatestAttempt: detail.LatestAttempt, ExecutionReviewState: detail.ExecutionReviewState}
			if err := s.projectInboxTaskReviews(r.Context(), []*task.TaskItemResponse{item}, requests, time.Now().UTC()); err != nil {
				respondJSONError(w, 503, "Could not read task review progress")
				return
			}
			detail.ReviewState = item.ReviewState
		}
	}
	if s.projectStudioRepo != nil && detail.SourceRoomID != "" && detail.LatestAttempt != nil {
		owner, ownerErr := s.taskArtifactOwner(r.Context(), detail)
		deliveries, deliveryErr := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), detail.SourceRoomID)
		byID := make(map[string]db.ProjectMessage, len(deliveries))
		for _, message := range deliveries {
			byID[message.ID] = message
		}
		if ownerErr == nil && deliveryErr == nil {
			seen := map[string]bool{}
			for _, attempt := range detail.Attempts {
				for _, prefix := range []string{"task-visual-delivery:", "task-delivery:"} {
					m, exists := byID[delegationMessageID(prefix+attempt.ID)]
					if !exists || m.RoomID != detail.SourceRoomID || m.AuthorType != "agent" || m.ReplyToID == nil || *m.ReplyToID != detail.SourceMessageID {
						continue
					}
					_, ids := decodeAttachmentMessage(m.Content)
					var delivered []conversationAttachment
					for _, aid := range ids {
						if attachment, _, err := loadAttachment(owner, aid); err == nil {
							delivered = append(delivered, attachment)
							if attempt.ID == detail.LatestAttempt.ID && !seen[aid] {
								detail.Attachments = append(detail.Attachments, attachment)
								seen[aid] = true
							}
						}
					}
					if len(delivered) > 0 {
						raw, _ := json.Marshal(map[string]any{"attachments": delivered})
						attemptID := attempt.ID
						detail.Activities = append(detail.Activities, &task.TaskActivity{ID: "files:" + m.ID, TaskID: detail.ID, AttemptID: &attemptID, ActivityType: "work_files", Author: m.AuthorName, Message: string(raw), CreatedAt: m.CreatedAt})
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
		if updated != nil && (updated.Status == task.TaskStatusDone || updated.Status == task.TaskStatusReview || updated.Status == task.TaskStatusBlocked || updated.Status == "cancelled" || updated.Status == "failed") {
			if s.terminalManager != nil {
				_ = s.terminalManager.CloseSession(id)
			}
			if s.taskSvc != nil {
				_ = s.taskSvc.FinishTaskLaunches(r.Context(), id, "stopped")
			}
		}
		s.kickTaskLaunches()
		if updated != nil && updated.Status == task.TaskStatusDone {
			s.DispatchTaskCompletedPush(r.Context(), updated)
			if updated.SourceRoomID != "" && s.projectStudioRepo != nil {
				s.publishInbox(updated.SourceRoomID)
			}
		}
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

	tail := false
	if raw := r.URL.Query().Get("tail"); raw != "" {
		var parseErr error
		tail, parseErr = strconv.ParseBool(raw)
		if parseErr != nil || tail && r.URL.Query().Get("after_id") != "" && r.URL.Query().Get("after_id") != "0" {
			respondJSONError(w, 400, "Use tail or after_id, not both")
			return
		}
	}
	var logs []*task.LogRecord
	var err error
	if tail {
		logs, err = s.taskSvc.GetRecentAttemptLogs(r.Context(), attemptID, limit)
	} else {
		logs, err = s.taskSvc.GetAttemptLogs(r.Context(), attemptID, afterID, limit)
	}
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

	if s.terminalManager != nil {
		defer s.terminalManager.CloseSession(taskID)
	}

	cancelledLaunches, launchErr := s.taskSvc.CancelLaunches(r.Context(), taskID, time.Now().UTC())
	if launchErr != nil {
		respondJSONError(w, 503, "Could not save cancellation. Check the launch status before retrying.")
		return
	}
	att, err := s.taskSvc.CancelTask(r.Context(), taskID, author)
	if cancelledLaunches > 0 && (errors.Is(err, task.ErrAttemptNotFound) || errors.Is(err, task.ErrInvalidTaskInput)) {
		cancelled := task.TaskStatusCancelled
		_, _ = s.taskSvc.UpdateTask(r.Context(), taskID, author, task.UpdateTaskRequest{Status: &cancelled})
		if s.projectStudioRepo != nil {
			_ = s.projectStudioRepo.UpdateTaskStatus(r.Context(), taskID, "cancelled")
		}
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
		if errors.Is(err, task.ErrInvalidTaskInput) {
			cancelled := task.TaskStatusCancelled
			_, _ = s.taskSvc.UpdateTask(r.Context(), taskID, author, task.UpdateTaskRequest{Status: &cancelled})
			if s.projectStudioRepo != nil {
				_ = s.projectStudioRepo.UpdateTaskStatus(r.Context(), taskID, "cancelled")
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
			respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	cancelled := task.TaskStatusCancelled
	_, _ = s.taskSvc.UpdateTask(r.Context(), taskID, author, task.UpdateTaskRequest{Status: &cancelled})
	if s.projectStudioRepo != nil {
		_ = s.projectStudioRepo.UpdateTaskStatus(r.Context(), taskID, "cancelled")
	}
	if s.runnerSupervisor != nil && att != nil {
		s.runnerSupervisor.CancelAttempt(att.ID)
	}
	if s.runnerSvc != nil && att != nil {
		_ = s.runnerSvc.CancelAttempt(r.Context(), att.ID)
	}
	if s.terminalManager != nil {
		_ = s.terminalManager.CloseSession(taskID)
		if att != nil && att.ID != "" && att.ID != taskID {
			_ = s.terminalManager.CloseSession(att.ID)
		}
	}
	if s.taskSvc != nil {
		_ = s.taskSvc.FinishTaskLaunches(r.Context(), taskID, "cancelled")
		s.kickTaskLaunches()
	}
	if detail, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil && detail.SourceRoomID != "" {
		s.publishInbox(detail.SourceRoomID)
	}
	if att != nil {
		respondJSON(w, http.StatusOK, att)
	} else {
		respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
	}
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
		Reason    string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.RunnerID == "" {
		req.RunnerID = "cuan-runner"
	}

	att, err := s.taskSvc.RetryTask(r.Context(), taskID, author, req.RunnerID, req.AgentName, req.Reason)
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



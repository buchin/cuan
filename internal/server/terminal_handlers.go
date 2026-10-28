package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/buchin/cuan/internal/db"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/agent"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskprocess"
	"github.com/buchin/cuan/internal/terminal"
)

const maxAgentRetries = 3

// resolveTaskID maps attempt IDs (att_...), raw message UUIDs, and room IDs into the canonical task ID.
func (s *Server) resolveTaskID(ctx context.Context, rawID string) string {
	raw := strings.TrimSpace(rawID)
	if raw == "" {
		return ""
	}

	// 1. If it's an attempt ID (att_...)
	if strings.HasPrefix(raw, "att_") && s.taskSvc != nil {
		if att, err := s.taskSvc.GetAttempt(ctx, raw); err == nil && att != nil && att.TaskID != "" {
			return att.TaskID
		}
	}

	// 2. Direct check if raw ID is already a task ID
	if s.taskSvc != nil {
		if _, err := s.taskSvc.GetTaskDetail(ctx, raw); err == nil {
			return raw
		}
	}
	if s.projectStudioRepo != nil {
		if _, err := s.projectStudioRepo.GetTaskByID(ctx, raw); err == nil {
			return raw
		}
	}

	// 3. Message UUID without "msg_" prefix
	if !strings.HasPrefix(raw, "msg_") && !strings.HasPrefix(raw, "task_") {
		msgID := "msg_" + raw
		if s.taskSvc != nil {
			if _, err := s.taskSvc.GetTaskDetail(ctx, msgID); err == nil {
				return msgID
			}
		}
		if s.projectStudioRepo != nil {
			if _, err := s.projectStudioRepo.GetTaskByID(ctx, msgID); err == nil {
				return msgID
			}
		}
	}

	// 4. Room or Conversation ID (conv_... or room UUID)
	if s.taskSvc != nil {
		roomTasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: raw})
		if err == nil && len(roomTasks) > 0 {
			// Find active task or latest task
			for _, t := range roomTasks {
				if t.Status == "doing" || t.Status == "review" || t.Status == "backlog" {
					return t.ID
				}
			}
			return roomTasks[0].ID
		}
	}

	return raw
}

// handleTaskTerminalWebSocket handles bidirectional WebSocket PTY streaming for tasks.
// Supports both Watch Mode (?mode=watch) and Interactive Take-Over Mode (?mode=interactive).
// GET /api/tasks/{id}/terminal or GET /api/v1/tasks/{id}/terminal
func (s *Server) handleTaskTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.terminalHandler == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal engine is not configured")
		return
	}

	if err := s.terminalHandler.ValidateRequestAuth(r); err != nil {
		respondJSONError(w, 401, "Sign in to Cuan to open the terminal")
		return
	}
	rawTaskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if rawTaskID == "" {
		rawTaskID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if rawTaskID == "" {
		rawTaskID = strings.TrimSpace(r.URL.Query().Get("task_id"))
	}

	if rawTaskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	taskID := s.resolveTaskID(r.Context(), rawTaskID)

	found := false
	if s.projectStudioRepo != nil {
		if _, e := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); e == nil {
			found = true
		}
	}
	if !found && s.taskSvc != nil {
		if _, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil {
			found = true
		}
	}
	if !found {
		respondJSONError(w, 404, "Task not found. No terminal was created.")
		return
	}

	if taskID != rawTaskID {
		q := r.URL.Query()
		q.Set("task_id", taskID)
		r.URL.RawQuery = q.Encode()
	}

	s.terminalHandler.ServeHTTP(w, r)
}

// handleGetTerminalSessionsStatus returns status of running terminal sessions.
// GET /api/terminal/sessions or GET /api/v1/terminal/sessions
func (s *Server) handleGetTerminalSessionsStatus(w http.ResponseWriter, r *http.Request) {
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}

	active := s.terminalManager.ActiveSessions()
	type SessionSummary struct {
		ID           string `json:"id"`
		TaskID       string `json:"task_id"`
		Subscribers  int    `json:"subscribers"`
		CreatedAt    string `json:"created_at"`
		LastActivity string `json:"last_activity"`
		Cols         uint16 `json:"cols"`
		Rows         uint16 `json:"rows"`
		PID          int    `json:"pid"`
		WorkDir      string `json:"work_dir"`
	}

	summaries := make([]SessionSummary, len(active))
	for i, sess := range active {
		cols, rows := sess.PTY().GetSize()
		summaries[i] = SessionSummary{
			ID:           sess.ID(),
			TaskID:       sess.TaskID(),
			Subscribers:  sess.SubscriberCount(),
			CreatedAt:    sess.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
			LastActivity: sess.LastActivity().Format("2006-01-02T15:04:05Z07:00"),
			Cols:         cols,
			Rows:         rows,
			PID:          sess.PTY().PID(),
			WorkDir:      sess.PTY().WorkDir(),
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": summaries,
		"count":    len(summaries),
	})
}

// handleCloseTaskTerminalSession terminates an active terminal session for a task.
// DELETE /api/tasks/{id}/terminal or DELETE /api/v1/tasks/{id}/terminal
func (s *Server) handleCloseTaskTerminalSession(w http.ResponseWriter, r *http.Request) {
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}

	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	if rawID == "" {
		rawID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if rawID == "" {
		rawID = strings.TrimSpace(r.URL.Query().Get("task_id"))
	}
	if rawID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}
	taskID := s.resolveTaskID(r.Context(), rawID)

	if err := s.terminalManager.CloseSession(taskID); err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to close terminal session: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "closed",
		"task_id": taskID,
	})
}

type runTaskAgentPayload struct {
	RequestID string `json:"request_id"`
	Revision  string `json:"revision"`
	AgentID   string `json:"agent_id"`
	Prompt    string `json:"prompt"`
	Binary    string `json:"binary"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

// handleRunTaskAgent spawns an autonomous or interactive agent process bound to the task PTY.
// POST /api/v1/tasks/{id}/run-agent
func (s *Server) handleRunTaskAgent(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuthOwner(w, r) {
		return
	}
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("task_id"))
	}
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var payload runTaskAgentPayload
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 40000)).Decode(&payload); err != nil {
			respondJSONError(w, 400, "Invalid execution request")
			return
		}
	}
	ownerID := s.accountOwner(w, r)
	if ownerID == "" {
		return
	}
	s.runTaskAgent(w, r, taskID, payload, ownerID)
}

// Shared launch path: HTTP owner authentication stays in the public wrapper.
// Internal callers must obtain owner and task scope from a persisted inbox request.
func (s *Server) runTaskAgent(w http.ResponseWriter, r *http.Request, taskID string, payload runTaskAgentPayload, ownerID string) {
	if ownerID == "" || s.terminalManager == nil {
		respondJSONError(w, 503, "Execution is unavailable")
		return
	}
	payload.AgentID = strings.TrimSpace(payload.AgentID)
	payload.Prompt = strings.TrimSpace(payload.Prompt)
	payload.Revision = strings.TrimSpace(payload.Revision)
	if len([]rune(payload.Revision)) > 6000 {
		respondJSONError(w, 400, "Revision feedback may contain up to 6000 characters")
		return
	}
	hashData, _ := json.Marshal([]string{payload.AgentID, payload.Prompt, payload.Revision, payload.Binary})
	requestHash := fmt.Sprintf("%x", sha256.Sum256(hashData))
	if s.taskSvc != nil && payload.RequestID != "" {
		previous, err := s.taskSvc.FindRequestAttempt(r.Context(), taskID, payload.RequestID, requestHash)
		if err != nil {
			code := 503
			if errors.Is(err, task.ErrInvalidTaskInput) {
				code = 400
			}
			if errors.Is(err, task.ErrRequestConflict) {
				code = 409
			}
			respondJSONError(w, code, err.Error())
			return
		}
		if previous != nil {
			result := map[string]any{"status": previous.Status, "attempt": previous, "task_id": taskID, "replayed": true}
			for _, h := range s.terminalManager.GetTaskHistory(taskID) {
				if h.AttemptID == previous.ID && h.Status == "running" {
					if live, ok := s.terminalManager.GetSession(taskID); ok && live.ID() == h.SessionID {
						result["session_id"] = live.ID()
						result["terminal_url"] = fmt.Sprintf("/api/v1/tasks/%s/terminal", taskID)
					}
				}
			}
			respondJSON(w, 200, result)
			return
		}
	}

	if _, busy := s.taskLaunches.LoadOrStore(taskID, true); busy {
		respondJSONError(w, 409, "A session is starting for this task. Open the existing session.")
		return
	}
	defer s.taskLaunches.Delete(taskID)
	if active, ownershipErr := s.terminalManager.TaskExecutionActive(taskID); ownershipErr != nil {
		respondJSONError(w, 503, "Could not verify the previous execution. No replacement was started.")
		return
	} else if active {
		respondJSONError(w, 409, "This work still has an active process. No replacement was started.")
		return
	}
	if history := s.terminalManager.GetTaskHistory(taskID); len(history) > 0 && history[len(history)-1].Status == "running" {
		respondJSONError(w, 409, "This task already has an active session. Open its terminal.")
		return
	}
	var projID string
	taskCategory := task.ExecutionCategoryCode
	if s.taskSvc != nil {
		if tDetail, err := s.taskSvc.GetTaskDetail(r.Context(), taskID); err == nil && tDetail != nil {
			projID = tDetail.ProjectID
			if tDetail.ExecutionCategory != "" {
				taskCategory = tDetail.ExecutionCategory
			}
		}
	}
	if projID == "" && s.projectStudioRepo != nil {
		if pt, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); err == nil && pt != nil {
			projID = pt.ProjectID
		}
	}
	if projID != "" && s.isProjectLaneActive(r.Context(), projID, taskCategory, taskID) {
		status := task.TaskStatusQueued
		if s.taskSvc != nil {
			_, _ = s.taskSvc.UpdateTask(r.Context(), taskID, "runner", task.UpdateTaskRequest{Status: &status})
		}
		if s.projectStudioRepo != nil {
			_ = s.projectStudioRepo.UpdateTaskStatus(r.Context(), taskID, "queued")
		}
		respondJSON(w, http.StatusConflict, map[string]string{
			"error": "Another task is actively running in this project. This task is queued to run when it finishes.",
			"code":  "project_concurrency_busy",
		})
		return
	}
	agentID := strings.TrimSpace(payload.AgentID)
	if agentID == "" && s.taskSvc != nil {
		if t, err := s.taskSvc.GetTaskDetail(r.Context(), taskID); err == nil && t != nil {
			if t.AssignedAgent != "" {
				agentID = t.AssignedAgent
			} else if t.LatestAttempt != nil && t.LatestAttempt.AgentName != "" {
				agentID = t.LatestAttempt.AgentName
			}
		}
	}
	if agentID == "" {
		respondJSONError(w, http.StatusBadRequest, "Select a registered agent first")
		return
	}
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Agent registry unavailable")
		return
	}
	registeredAgent, err := s.agentRepo.GetAgentByID(r.Context(), agentID)
	if (err != nil || registeredAgent == nil) && s.agentRepo != nil {
		cleanID := strings.TrimPrefix(agentID, "@")
		if ag, e := s.agentRepo.GetAgentByID(r.Context(), "agent_"+cleanID); e == nil && ag != nil {
			registeredAgent = ag
			err = nil
		} else if ags, e := s.agentRepo.ListAgents(r.Context(), db.AgentFilter{}); e == nil {
			for _, a := range ags {
				if strings.EqualFold(a.ID, agentID) || strings.EqualFold(a.ID, "agent_"+cleanID) || strings.EqualFold(a.Name, agentID) || strings.EqualFold(strings.TrimPrefix(a.Name, "@"), cleanID) {
					agCopy := a
					registeredAgent = &agCopy
					err = nil
					break
				}
			}
		}
	}
	if err != nil || registeredAgent == nil {
		respondJSONError(w, http.StatusNotFound, "Agent is not registered")
		return
	}
	if !registeredAgent.Enabled {
		respondJSONError(w, http.StatusConflict, "Agent is disabled")
		return
	}
	profileHome, err := s.ownedAgentProfileHome(r.Context(), ownerID, registeredAgent)
	if err != nil {
		respondJSONError(w, http.StatusConflict, err.Error())
		return
	}
	expectedBinary := "agy"
	switch registeredAgent.Harness {
	case "claude":
		expectedBinary = "claude"
	case "codex":
		expectedBinary = "codex"
	case "commandcode":
		expectedBinary = "commandcode"
	case "", "agy", "antigravity":
	default:
		respondJSONError(w, 409, "This agent type cannot run tasks")
		return
	}
	if payload.Binary != "" && payload.Binary != expectedBinary && !(expectedBinary == "agy" && payload.Binary == "antigravity") {
		respondJSONError(w, 400, "Use the agent type that matches its connection")
		return
	}
	payload.Binary = expectedBinary
	selectedModel := ""
	selectedAccountID := ""
	var selected *inboxAccountRoute
	routeNote := ""
	routeRelease := func() {}
	routeLaunched := false
	defer func() {
		if !routeLaunched {
			routeRelease()
		}
	}()
	if expectedBinary == "agy" {
		primary, _ := agent.ProviderAccountForConnection(ownerID, registeredAgent.ID, registeredAgent.ConnectionID)
		limited := primary != nil && primary.State == "quota_limited"
		routes := s.providerRoutes(r.Context(), ownerID, registeredAgent, profileHome, limited)
		if len(routes) == 0 {
			respondJSON(w, 409, map[string]string{"error": "No allowed account/model route is available. Check usage and fallback settings in Settings → Agents. The task has not started.", "code": "provider_unavailable"})
			return
		}
		for i := range routes {
			release, err := s.leaseProviderRoute(r.Context(), ownerID, registeredAgent, routes[i])
			if err == nil {
				routeRelease = release
				selected = &routes[i]
				break
			}
		}
		if selected == nil {
			respondJSON(w, 409, map[string]string{"error": "All allowed AI account session slots are in use. The task has not started. Try again when a session finishes.", "code": "provider_capacity"})
			return
		}
		profileHome = selected.home
		selectedModel = selected.model
		selectedAccountID = selected.accountID
		source := "primary account"
		if !selected.primary {
			source = "allowed backup account"
		}
		model := selected.model
		if model == "" {
			model = "provider default"
		}
		routeNote = fmt.Sprintf("Provider preflight selected %s; model: %s. Agent identity and task scope are unchanged.", source, model)
	}
	var commandCodeKey string
	if expectedBinary == "commandcode" {
		var conn *db.AgentConnection
		if registeredAgent.ConnectionID != "" {
			conn, _ = s.agentRepo.GetConnectionByID(r.Context(), registeredAgent.ConnectionID)
		}
		if (conn == nil || conn.CredentialsEncrypted == "") && ownerID != "" {
			if userConn, err := s.agentRepo.GetConnectionByUserAndProvider(r.Context(), ownerID, "commandcode"); err == nil && userConn != nil && userConn.CredentialsEncrypted != "" {
				conn = userConn
			}
		}
		if conn != nil && conn.CredentialsEncrypted != "" {
			if key, err := agent.DecryptCredential(conn.CredentialsEncrypted); err == nil {
				commandCodeKey = key
			}
		}
		selectedModel = registeredAgent.ModelID
		if idx := strings.Index(selectedModel, "#"); idx != -1 {
			selectedModel = selectedModel[idx+1:]
		}
	}
	agentName := registeredAgent.Name

	taskPrompt := strings.TrimSpace(payload.Prompt)
	workDir := ""
	sessionContext := ""
	var attachmentPaths []string
	taskFound := false
	sourceRoom := ""

	// 1. Check project tasks repository
	if s.projectStudioRepo != nil {
		if pt, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); err == nil && pt != nil {
			taskFound = true
			if taskPrompt == "" {
				taskPrompt = pt.Description
				if taskPrompt == "" {
					taskPrompt = pt.Title
				}
			}
			pt.AssigneeID = &agentID
			pt.ExecutionStatus = "running"
			_ = s.projectStudioRepo.UpdateTask(r.Context(), pt)
			_ = s.projectStudioRepo.UpdateTaskExecution(r.Context(), taskID, "running", pt.AgentOutput)
		}
	}

	// Canonical tasks persist an attempt before the process is launched.
	var attempt *task.TaskAttempt
	var detail *task.TaskDetailResponse
	if s.taskSvc != nil {
		var e error
		if detail, e = s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil && detail != nil {
			taskFound = true
			// A review produced by a successful attempt needs concrete feedback
			// before it can become a revision. If the latest execution itself
			// failed or was interrupted, however, the user is retrying that
			// execution and should not have to invent revision instructions.
			revision := strings.TrimSpace(payload.Revision)
			retryableFailure := detail.LatestAttempt != nil &&
				(detail.LatestAttempt.Status == task.AttemptStatusFailed ||
					detail.LatestAttempt.Status == task.AttemptStatusInterrupted ||
					detail.LatestAttempt.Status == task.AttemptStatusCanceled)
			if detail.Status == task.TaskStatusReview && revision == "" && !retryableFailure {
				respondJSONError(w, 400, "Describe the changes needed before starting a revision")
				return
			}
			if detail.Status == task.TaskStatusDone {
				respondJSONError(w, 409, "Reopen the completed task before starting a new session")
				return
			}
			sourceRoom = detail.SourceRoomID
			if detail.SourceRoomID != "" {
				if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
					if requests, e := store.ListInboxRequests(r.Context(), detail.SourceRoomID); e == nil {
						for _, q := range requests {
							if q.MessageID == detail.SourceMessageID && q.OwnerID == ownerID {
								attachmentPaths = s.inboxAttachmentPaths(r.Context(), q)
								break
							}
						}
					}
				}
			}
			if detail.ProjectID != "" && s.projectSvc != nil {
				project, e := s.projectSvc.TaskWorkspace(r.Context(), detail.ProjectID, detail.Instructions)
				if e != nil || project == nil {
					respondJSONError(w, 409, "Task project is unavailable")
					return
				}
				if project.IsArchived {
					respondJSONError(w, 409, "Restore the archived project before starting a task")
					return
				}
				if project.RepoURL != "" || project.LocalPath != "" {
					info, e := os.Stat(project.LocalPath)
					if project.RepoURL != "" && (project.LocalPath == "" || e != nil || !info.IsDir() || project.CloneStatus != "synced") {
						// Auto-sync repository workspace automatically before launching agent session
						if synced, syncErr := s.projectSvc.Sync(r.Context(), detail.ProjectID); syncErr == nil && synced != nil {
							project = synced
							info, e = os.Stat(project.LocalPath)
						}
					}
					if project.RepoURL == "" && (e != nil || !info.IsDir()) && project.LocalPath != "" {
						_ = os.MkdirAll(project.LocalPath, 0755)
						info, e = os.Stat(project.LocalPath)
					}
					if project.LocalPath == "" || e != nil || !info.IsDir() {
						respondJSONError(w, 409, "Project repository is not ready. Sync it before running the task.")
						return
					}
					workDir, e = s.projectSvc.TaskExecutionWorkspace(r.Context(), taskID, detail.Instructions, project)
					if e != nil {
						if errors.Is(e, cuangit.ErrRepositoryDownloadUnavailable) {
							respondJSON(w, 503, map[string]string{"error": e.Error(), "code": "repository_preparing"})
							return
						}
						respondJSONError(w, 409, e.Error())
						return
					}
				}
			}

			if taskPrompt == "" {
				taskPrompt = detail.Instructions
				if taskPrompt == "" {
					taskPrompt = detail.Title
				}
			}
			var criteria strings.Builder
			for _, item := range detail.DoDChecklist {
				fmt.Fprintf(&criteria, "- %s\n", item.Text)
			}
			sessionContext = fmt.Sprintf("## Bound task\nTask: %s\nTitle: %s\n", taskID, detail.Title)
			for _, section := range []struct{ label, text string }{
				{"Human instructions", detail.Instructions}, {"Acceptance criteria", criteria.String()}, {"Previous work summary", detail.Summary},
				{"Previous completion notes", detail.CompleteNote}, {"Revision feedback", revision},
			} {
				if strings.TrimSpace(section.text) != "" {
					sessionContext += "\n## " + section.label + "\n" + section.text + "\n"
				}
			}
			sessionContext += taskRecoveryContext(detail)
			if revision != "" {
				taskPrompt = revision
			}
			attempt, err = s.taskSvc.CreateAttempt(r.Context(), taskID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: agentID, RequestID: payload.RequestID, RequestHash: requestHash})
			if errors.Is(err, task.ErrTaskActive) {
				respondJSONError(w, 409, "This task already has an active attempt. Check the session before retrying.")
				return
			}
			if err != nil {
				if errors.Is(err, task.ErrInvalidTaskInput) {
					respondJSONError(w, 400, err.Error())
					return
				}
				respondJSONError(w, 503, "Could not save the attempt. Execution has not started.")
				return
			}
			if revision != "" {
				if _, e := s.taskSvc.AddComment(r.Context(), taskID, "owner", "Revision requested:\n"+revision); e != nil {
					_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Revision feedback could not be saved. Execution did not start."})
					respondJSONError(w, 503, "Could not save revision feedback. Execution did not start.")
					return
				}
			}
			doing := task.TaskStatusDoing
			emptyReport := ""
			resetCriteria := append([]task.DoDItem{}, detail.DoDChecklist...)
			for i := range resetCriteria {
				resetCriteria[i].Completed = false
			}
			if _, err = s.taskSvc.UpdateTask(r.Context(), taskID, "owner", task.UpdateTaskRequest{Status: &doing, AssignedAgent: &agentID, DoDChecklist: &resetCriteria, Summary: &emptyReport, CompleteNote: &emptyReport}); err != nil {
				_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Could not save task status before starting."})
				respondJSONError(w, 503, "Task status could not be saved")
				return
			}
			_, err = s.taskSvc.UpdateAttemptStatus(r.Context(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
			if err != nil {
				_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Could not save attempt state before launch."})
				_ = s.taskSvc.ReconcileLocalTask(context.Background(), taskID, attempt.ID)
				respondJSONError(w, 503, "Could not save attempt status. The process has not started.")
				return
			}
		}
	}

	if !taskFound {
		respondJSONError(w, 404, "Task not found")
		return
	}

	var reportEnv []string
	reportToken := ""
	reportCleanup := func() {}
	if attempt != nil {
		if workDir == "" {
			workDir, err = s.terminalManager.ResolveTaskWorkspace(taskID)
			if err != nil {
				_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Task workspace could not be prepared."})
				_ = s.taskSvc.ReconcileLocalTask(context.Background(), taskID, attempt.ID)
				respondJSONError(w, 503, "Task workspace unavailable.")
				return
			}
		}
		gitEnv, identityError := s.taskCommitEnvironment(r.Context(), ownerID)
		if identityError != nil {
			_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Task Git identity is unavailable. Execution did not start."})
			_ = s.taskSvc.ReconcileLocalTask(context.Background(), taskID, attempt.ID)
			respondJSONError(w, 409, identityError.Error())
			return
		}
		sessionContext += "\n## Workspace\n" + workDir + "\n"
		var extra string
		reportEnv, extra, reportToken, reportCleanup, err = s.issueTaskAgent(context.WithValue(r.Context(), taskAgentWorkspaceKey{}, workDir), taskID, attempt.ID, agentID, sessionContext)
		if err != nil {
			_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Task reporting tools could not be prepared. Execution did not start."})
			_ = s.taskSvc.ReconcileLocalTask(context.Background(), taskID, attempt.ID)
			respondJSONError(w, 503, "Task reporting tools unavailable. The process has not started.")
			return
		}
		reportEnv = append(reportEnv, gitEnv...)
		if commandCodeKey != "" {
			reportEnv = append(reportEnv, "COMMAND_CODE_API_KEY="+commandCodeKey)
		}
		if expectedBinary == "commandcode" && detail != nil && strings.HasPrefix(detail.Title, "Terminal · ") && strings.TrimSpace(payload.Prompt) == "" {
			taskPrompt = ""
		} else {
			taskPrompt += extra
		}
	}
	quotaHit := false
	retryCount := 0
	launched := false
	defer func() {
		if !launched {
			reportCleanup()
		}
	}()
	for retryCount < maxAgentRetries {
		launchSettled := make(chan struct{})
		var output *taskOutputRecorder
		if attempt != nil {
			output = newTaskOutputRecorder(s.taskSvc, attempt.ID)
			output.secrets = append(output.secrets, reportToken)
			output.onQuota = func() {
				<-launchSettled
				if selectedAccountID != "" {
					_ = agent.RecordProviderModelQuota(ownerID, selectedAccountID, selectedModel)
				}
				_ = s.terminalManager.StopQuotaAttempt(taskID, attempt.ID)
				quotaHit = true
			}
			output.onRegion = func() {
				<-launchSettled
				if selectedAccountID != "" {
					_ = agent.RecordProviderModelRestriction(ownerID, selectedAccountID, selectedModel)
				}
				_ = s.terminalManager.StopRestrictedAttempt(taskID, attempt.ID)
			}
		}

		var attemptID string
		if attempt != nil {
			attemptID = attempt.ID
		}

		// 3. Spawn PTY process via Manager
		runCfg := terminal.TaskAgentRunConfig{
			Model:           selectedModel,
			AttachmentPaths: attachmentPaths,
			AttemptID:         attemptID,
			TaskID:            taskID,
			ProjectID:         projID,
			ExecutionCategory: taskCategory,
			AgentID:           agentID,
			AgentName:       agentName,
			Command:         payload.Binary,
			Prompt:          taskPrompt,
			WorkDir:         workDir,
			Cols:            payload.Cols,
			Rows:            payload.Rows,
			Mode:            terminal.ModeInteractive,
			OnOutput: func(chunk []byte) {
				if output != nil {
					output.append(chunk)
				}
			},
			OnExit: func(h *terminal.TaskRunHistory) {
				defer routeRelease()
				defer reportCleanup()
				defer func() {
					if sourceRoom != "" {
						s.publishInbox(sourceRoom)
					}
				}()
				if output != nil {
					output.flush()
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if s.projectStudioRepo != nil {
					_ = s.projectStudioRepo.UpdateTaskExecution(ctx, taskID, h.Status, fmt.Sprintf("Runner exited with code %d (%s). Review the result and verification evidence.", h.ExitCode, h.Status))
				}
				if attempt != nil && s.taskSvc != nil {
					state := task.AttemptStatusFailed
					if (h.ExitCode == 0 && h.EndReason != "provider_quota" && h.EndReason != "provider_restricted") || h.EndReason == "agent_report" {
						state = task.AttemptStatusSucceeded
					}
					note := output.failureNote()
					if h.EndReason == "agent_report" {
						note = ""
					}
					if h.ExitCode == 0 && h.EndReason != "provider_quota" && h.EndReason != "provider_restricted" {
						ready, e := s.taskSvc.AgentReviewReady(ctx, taskID, attempt.ID)
						if e != nil || !ready {
							state = task.AttemptStatusFailed
							note = "No review-ready result was recorded. Check the agent report and saved output."
						}
					}
					var e error
					if h.EndReason == "agent_report" {
						_, e = s.taskSvc.FinishReportedAttempt(ctx, attempt.ID, h.ExitCode)
					} else {
						_, e = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: state, ExitCode: &h.ExitCode, ErrorMessage: note})
					}
					if e == nil {
						_ = s.taskSvc.ReconcileLocalTask(r.Context(), taskID, attempt.ID)
					}
					_ = s.taskSvc.FinishTaskLaunches(context.Background(), taskID, "stopped")
					s.kickTaskLaunches()
				}
			},
		}

		if attempt != nil {
			runCfg.AttemptID = attempt.ID
		}
		if profileHome != "" {
			if payload.Binary == "agy" || payload.Binary == "antigravity" || payload.Binary == "" {
				_ = agent.PrepareAntigravitySettings(profileHome, workDir)
				_ = agent.EnsureAntigravityOnboarding(profileHome)
			}
			runCfg.Env = agent.AntigravityProfileEnvironment(profileHome)
		}
		runCfg.Env = append(runCfg.Env, reportEnv...)
		if attempt != nil && routeNote != "" {
			if _, err := s.taskSvc.AddComment(r.Context(), taskID, "runner", routeNote); err != nil {
				_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Provider route could not be recorded. Execution did not start."})
				_ = s.taskSvc.ReconcileLocalTask(r.Context(), taskID, attempt.ID)
				respondJSONError(w, 503, "Provider route could not be recorded. The task has not started.")
				return
			}
		}
		close(launchSettled)
		sess, err := s.terminalManager.RunTaskAgent(r.Context(), runCfg)
		if err != nil {
			if errors.Is(err, taskprocess.ErrActive) {
				status := task.TaskStatusQueued
				if attempt != nil {
					_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusQueued})
				}
				if s.taskSvc != nil {
					_, _ = s.taskSvc.UpdateTask(context.Background(), taskID, "runner", task.UpdateTaskRequest{Status: &status})
				}
				if s.projectStudioRepo != nil {
					_ = s.projectStudioRepo.UpdateTaskStatus(context.Background(), taskID, "queued")
				}
				respondJSON(w, http.StatusConflict, map[string]string{
					"error": "Another task is actively running in this project. This task is queued to run when it finishes.",
					"code":  "project_concurrency_busy",
				})
				return
			}
			if attempt != nil {
				_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Could not start the CLI. Check its installation and the agent connection."})
				status := task.TaskStatusBlocked
				_, _ = s.taskSvc.UpdateTask(context.Background(), taskID, "runner", task.UpdateTaskRequest{Status: &status})
			}
			if s.taskSvc != nil {
				_ = s.taskSvc.FinishTaskLaunches(context.Background(), taskID, "stopped")
				s.kickTaskLaunches()
			}
			if s.projectStudioRepo != nil {
				_ = s.projectStudioRepo.UpdateTaskExecution(context.Background(), taskID, "failed", "Could not start the CLI.")
			}
			respondJSONError(w, http.StatusInternalServerError, "Could not start the CLI. Check its installation and the agent connection.")
			return
		}

		// Wait briefly to detect quota hit during early execution
		<-launchSettled

		if !quotaHit {
			if sourceRoom != "" {
				s.publishInbox(sourceRoom)
			}
			launched = true
			routeLaunched = true

			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":       "running",
				"session_id":   sess.ID(),
				"task_id":      taskID,
				"pid":          sess.PTY().PID(),
				"terminal_url": fmt.Sprintf("/api/v1/tasks/%s/terminal", taskID),
				"agent_id":     agentID,
				"agent_name":   agentName,
				"attempt":      attempt,
			})
			return
		}

		// Quota hit - retry with next available agent
		retryCount++
		quotaHit = false
		if retryCount >= maxAgentRetries {
			break
		}

		// Select next available route for retry
		primary, _ := agent.ProviderAccountForConnection(ownerID, registeredAgent.ID, registeredAgent.ConnectionID)
		limited := primary != nil && primary.State == "quota_limited"
		routes := s.providerRoutes(r.Context(), ownerID, registeredAgent, profileHome, limited)
		var nextSelected *inboxAccountRoute
		for i := range routes {
			release, err := s.leaseProviderRoute(r.Context(), ownerID, registeredAgent, routes[i])
			if err == nil {
				routeRelease = release
				nextSelected = &routes[i]
				break
			}
		}
		if nextSelected == nil {
			break
		}
		selected = nextSelected
		profileHome = selected.home
		selectedModel = selected.model
		selectedAccountID = selected.accountID
		source := "primary account"
		if !selected.primary {
			source = "allowed backup account"
		}
		model := selected.model
		if model == "" {
			model = "provider default"
		}
		routeNote = fmt.Sprintf("Auto-retry: provider preflight selected %s; model: %s. Agent identity and task scope are unchanged.", source, model)
	}

	// All retries exhausted - show informative error
	if attempt != nil {
		_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Provider quota reached on all available accounts. Your message is saved. Try again after the limit resets."})
		status := task.TaskStatusBlocked
		_, _ = s.taskSvc.UpdateTask(context.Background(), taskID, "runner", task.UpdateTaskRequest{Status: &status})
	}
	if s.projectStudioRepo != nil {
		_ = s.projectStudioRepo.UpdateTaskExecution(context.Background(), taskID, "failed", "Provider quota reached on all available accounts.")
	}
	respondJSON(w, 409, map[string]string{"error": "All available agents reached usage limits. Your message is saved. Try again after the limit resets.", "code": "provider_quota_exhausted"})
}


// handleGetTaskRunHistory returns execution history of an agent runner on a task.
// GET /api/v1/tasks/{id}/run-agent/history
func (s *Server) handleGetTaskRunHistory(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}

	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	if rawID == "" {
		rawID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if rawID == "" {
		rawID = strings.TrimSpace(r.URL.Query().Get("task_id"))
	}
	if rawID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}
	taskID := s.resolveTaskID(r.Context(), rawID)

	history := s.terminalManager.GetTaskHistory(taskID)
	var live map[string]any
	if session, ok := s.terminalManager.GetSession(taskID); ok {
		for _, h := range history {
			if h.SessionID == session.ID() && h.Status == "running" {
				live = map[string]any{"session_id": session.ID(), "attempt_id": h.AttemptID, "agent_id": h.AgentID, "started_at": h.StartedAt}
				break
			}
		}
		if live == nil {
			live = map[string]any{
				"session_id": session.ID(),
				"attempt_id": "",
				"started_at": time.Now(),
			}
		}
		if live["attempt_id"] == "" && s.taskSvc != nil {
			if t, err := s.taskSvc.GetTaskDetail(r.Context(), taskID); err == nil && t != nil && t.LatestAttempt != nil {
				live["attempt_id"] = t.LatestAttempt.ID
				if live["agent_id"] == nil || live["agent_id"] == "" {
					live["agent_id"] = t.LatestAttempt.AgentName
				}
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"task_id":      taskID,
		"history":      history,
		"live_session": live,
		"count":        len(history),
	})
}

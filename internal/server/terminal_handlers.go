package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	cuangit "github.com/buchin/cuan/internal/git"
	projectpkg "github.com/buchin/cuan/internal/project"
	cuanPrompt "github.com/buchin/cuan/internal/prompt"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskprocess"
	"github.com/buchin/cuan/internal/terminal"
)

const maxAgentRetries = 3

// resolveTaskID maps attempt IDs (att_...), raw message UUIDs, and room IDs into the canonical task ID.
func (s *Server) resolveTaskID(ctx context.Context, rawID string) string {
	raw := strings.TrimSpace(rawID)
	if raw == "" || strings.HasPrefix(raw, "project:") {
		return raw
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

	// Attempt-specific attaches may never fall through to another task's PTY.
	expected := strings.TrimPrefix(r.URL.Query().Get("session_id"), "runner-sess-")

	// Project terminal auto-route & activation:
	// Dedicated project terminals (title "Terminal · ...") run independently without task queue delays.
	var tDetail *task.TaskDetailResponse
	if s.taskSvc != nil {
		if td, err := s.taskSvc.GetTaskDetail(r.Context(), taskID); err == nil && td != nil {
			tDetail = td
		}
	}
	targetProjID := ""
	if tDetail != nil {
		targetProjID = tDetail.ProjectID
		if targetProjID == "" && len(tDetail.ProjectIDs) > 0 {
			targetProjID = tDetail.ProjectIDs[0]
		}
	}
	isDedicatedTerminalTask := tDetail != nil && strings.HasPrefix(tDetail.Title, "Terminal ·") && targetProjID != ""

	if isDedicatedTerminalTask {
		sessionKey := terminal.ProjectSessionKey(targetProjID)
		if s.terminalManager != nil {
			s.terminalManager.RegisterTaskAlias(taskID, sessionKey)
		}
		if s.projectSvc != nil {
			if proj, err := s.projectSvc.Get(r.Context(), targetProjID); err == nil && proj != nil {
				doing := task.TaskStatusDoing
				_, _ = s.taskSvc.UpdateTask(r.Context(), taskID, "runner", task.UpdateTaskRequest{Status: &doing})
				var activeAttID string
				if tDetail.LatestAttempt != nil && (tDetail.LatestAttempt.Status == task.AttemptStatusRunning || tDetail.LatestAttempt.Status == task.AttemptStatusQueued) {
					activeAttID = tDetail.LatestAttempt.ID
					if tDetail.LatestAttempt.Status != task.AttemptStatusRunning {
						_, _ = s.taskSvc.UpdateAttemptStatus(r.Context(), tDetail.LatestAttempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
					}
				} else {
					if att, err := s.taskSvc.CreateAttempt(r.Context(), taskID, "runner", task.CreateAttemptRequest{
						RunnerID:  "local_terminal",
						AgentName: tDetail.AssignedAgent,
					}); err == nil && att != nil {
						activeAttID = att.ID
						_, _ = s.taskSvc.UpdateAttemptStatus(r.Context(), att.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
					}
				}
				if activeAttID != "" {
					_ = s.taskSvc.RenewLocalAttemptLease(r.Context(), activeAttID)
				}
				q := r.URL.Query()
				q.Set("project_id", targetProjID)
				q.Set("id", targetProjID)
				if strings.HasPrefix(q.Get("session_id"), "att_") || q.Get("session_id") == "live" {
					q.Del("session_id")
				}
				r.URL.RawQuery = q.Encode()
				if rctx := chi.RouteContext(r.Context()); rctx != nil {
					for i := range rctx.URLParams.Keys {
						if rctx.URLParams.Keys[i] == "id" {
							rctx.URLParams.Values[i] = targetProjID
						}
					}
				}
				s.handleProjectTerminalWebSocket(w, r)
				return
			}
		}
	}

	if strings.HasPrefix(expected, "att_") {
		if s.taskSvc == nil {
			respondJSONError(w, 503, "Task attempt lookup is unavailable")
			return
		}
		attempt, err := s.taskSvc.AttemptForTask(r.Context(), taskID, expected)
		if err != nil || attempt.Status != task.AttemptStatusRunning {
			respondJSONError(w, 409, "The selected attempt has no live terminal")
			return
		}
		if s.terminalManager != nil {
			if session, ok := s.terminalManager.GetSession(taskID); ok {
				matched := false
				for _, history := range s.terminalManager.GetTaskHistory(taskID) {
					if history.SessionID == session.ID() && history.AttemptID == expected {
						matched = true
					}
				}
				if !matched {
					respondJSONError(w, 409, "The selected attempt is no longer live")
					return
				}
				query := r.URL.Query()
				query.Set("session_id", session.ID())
				r.URL.RawQuery = query.Encode()
			}
		}
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

// handleProjectTerminalWebSocket handles bidirectional WebSocket PTY streaming for dedicated project terminals.
// Supports background persistence and re-attach without exit/timeout when client/browser closes.
// GET /api/projects/{id}/terminal or GET /api/v1/projects/{id}/terminal
func (s *Server) handleProjectTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.terminalHandler == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal engine is not configured")
		return
	}

	if err := s.terminalHandler.ValidateRequestAuth(r); err != nil {
		respondJSONError(w, 401, "Sign in to Cuan to open the terminal")
		return
	}

	rawProjID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "project_id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if rawProjID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	if s.projectSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project service is unavailable")
		return
	}

	proj, err := s.projectSvc.Get(r.Context(), rawProjID)
	if err != nil || proj == nil {
		respondJSONError(w, 404, "Project not found. No terminal was created.")
		return
	}

	workDir, err := s.projectSvc.ProjectTerminalWorkspace(r.Context(), proj.ID, proj)
	if err != nil || workDir == "" {
		workDir = proj.LocalPath
	}

	var env []string
	settings := projectpkg.SettingsFromMetadata(proj.Metadata)
	if settings.Environment != "" {
		for k, v := range projectpkg.ParseEnvironment(settings.Environment) {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	ownerID := ""
	if s.authSvc != nil {
		token := auth.ExtractToken(r)
		if token == "" {
			token = strings.TrimSpace(r.URL.Query().Get("token"))
		}
		if token == "" {
			for _, p := range websocket.Subprotocols(r) {
				if strings.HasPrefix(p, "token.") {
					token = strings.TrimPrefix(p, "token.")
					break
				}
			}
		}
		if token != "" {
			if owner, err := s.authSvc.ValidateToken(r.Context(), token); err == nil && owner != nil {
				ownerID = owner.ID
			}
		}
	}

	sessionKey := terminal.ProjectSessionKey(proj.ID)

	// If session already exists, re-attach directly without leasing a new brain or advancing brainCursor!
	if existing, ok := s.terminalManager.GetSession(sessionKey); ok && existing != nil && !existing.IsClosed() {
		q := r.URL.Query()
		q.Set("task_id", sessionKey)
		q.Set("session_id", existing.ID())
		q.Set("attach", "1")
		r.URL.RawQuery = q.Encode()

		s.terminalHandler.ServeHTTP(w, r)
		return
	}

	if ownerID != "" {
		if home, brain, release, err := agent.LeaseHealthyBrainForProject(ownerID, proj.ID, nil); err == nil && home != "" {
			env = append(env, agent.AntigravityProfileEnvironment(home)...)
			if brain != nil && brain.ModelID != "" {
				env = append(env, "CUAN_AGENT_MODEL="+brain.ModelID)
			}
			release()
		}
	}

	sess, err := s.terminalManager.GetOrCreateProjectSession(context.WithoutCancel(r.Context()), proj.ID, workDir, "", 80, 24, env...)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start project terminal: %v", err))
		return
	}

	q := r.URL.Query()
	q.Set("task_id", sessionKey)
	q.Set("session_id", sess.ID())
	q.Set("attach", "1")
	r.URL.RawQuery = q.Encode()

	s.terminalHandler.ServeHTTP(w, r)
}

// handleGetProjectTerminalStatus returns status and upstream tracking for the project terminal.
// GET /api/projects/{id}/terminal/status or GET /api/v1/projects/{id}/terminal/status
func (s *Server) handleGetProjectTerminalStatus(w http.ResponseWriter, r *http.Request) {
	rawProjID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "project_id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if rawProjID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}
	if s.projectSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project service is unavailable")
		return
	}
	proj, err := s.projectSvc.Get(r.Context(), rawProjID)
	if err != nil || proj == nil {
		respondJSONError(w, 404, "Project not found")
		return
	}

	workDir, err := s.projectSvc.ProjectTerminalWorkspace(r.Context(), proj.ID, proj)
	if err != nil || workDir == "" {
		workDir = proj.LocalPath
	}

	shouldFetch := r.URL.Query().Get("fetch") == "1" || r.URL.Query().Get("fetch") == "true"
	var integrity GitIntegrityReport
	if shouldFetch {
		integrity = fetchAndInspectUpstream(r.Context(), workDir, "")
	} else {
		integrity = inspectGitIntegrity(r.Context(), workDir, "")
	}

	var sessSummary map[string]interface{}
	if s.terminalManager != nil {
		if sess, ok := s.terminalManager.GetProjectSession(proj.ID); ok && !sess.IsClosed() {
			cols, rows := sess.PTY().GetSize()
			sessSummary = map[string]interface{}{
				"active":        true,
				"session_id":    sess.ID(),
				"subscribers":   sess.SubscriberCount(),
				"created_at":    sess.CreatedAt().Format(time.RFC3339),
				"last_activity": sess.LastActivity().Format(time.RFC3339),
				"cols":          cols,
				"rows":          rows,
				"pid":           sess.PTY().PID(),
				"work_dir":      sess.PTY().WorkDir(),
			}
		}
	}
	if sessSummary == nil {
		sessSummary = map[string]interface{}{
			"active": false,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"project_id":   proj.ID,
		"project_name": proj.Name,
		"work_dir":     workDir,
		"session":      sessSummary,
		"upstream":     integrity,
	})
}

// handleProjectTerminalSyncUpstream triggers a non-destructive git fetch to check upstream commits.
// POST /api/projects/{id}/terminal/sync-upstream or POST /api/v1/projects/{id}/terminal/sync-upstream
func (s *Server) handleProjectTerminalSyncUpstream(w http.ResponseWriter, r *http.Request) {
	rawProjID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "project_id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if rawProjID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}
	if s.projectSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project service is unavailable")
		return
	}
	proj, err := s.projectSvc.Get(r.Context(), rawProjID)
	if err != nil || proj == nil {
		respondJSONError(w, 404, "Project not found")
		return
	}

	workDir, err := s.projectSvc.ProjectTerminalWorkspace(r.Context(), proj.ID, proj)
	if err != nil || workDir == "" {
		workDir = proj.LocalPath
	}

	// Non-destructive fetch without touching uncommitted files or HEAD
	integrity := fetchAndInspectUpstream(r.Context(), workDir, "")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "fetched",
		"upstream": integrity,
	})
}

// handleCloseProjectTerminalSession closes an active persistent project terminal session.
// DELETE /api/projects/{id}/terminal or DELETE /api/v1/projects/{id}/terminal
func (s *Server) handleCloseProjectTerminalSession(w http.ResponseWriter, r *http.Request) {
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}
	rawProjID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "project_id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if rawProjID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	_ = s.terminalManager.CloseProjectSession(rawProjID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "closed",
		"project_id": rawProjID,
	})
}

// handleRotateProjectTerminalBrain rotates the AI brain for an active project terminal session upon quota limits.
// POST /api/projects/{id}/terminal/rotate-brain or POST /api/v1/projects/{id}/terminal/rotate-brain
func (s *Server) handleRotateProjectTerminalBrain(w http.ResponseWriter, r *http.Request) {
	rawProjID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "project_id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(chi.URLParam(r, "id"))
	}
	if rawProjID == "" {
		rawProjID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if rawProjID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	ownerID := s.accountOwner(w, r)
	if ownerID == "" {
		return
	}

	sess, ok := s.terminalManager.GetProjectSession(rawProjID)
	if !ok || sess == nil || sess.IsClosed() {
		respondJSONError(w, http.StatusNotFound, "No active terminal session for project")
		return
	}

	// Lease healthy backup brain
	home, brain, release, err := agent.LeaseHealthyBrainForTask(ownerID, nil)
	if err != nil || brain == nil {
		respondJSON(w, http.StatusConflict, map[string]string{
			"error": "No healthy backup AI model available",
			"code":  "provider_capacity",
		})
		return
	}
	defer release()

	notice := fmt.Sprintf("\r\n\x1b[32m[cuan]\x1b[0m Rotated to healthy brain %s (%s). Terminal session remains active.\r\n", brain.ModelID, brain.ID)
	sess.BroadcastBytes([]byte(notice))

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "rotated",
		"account_id": brain.ID,
		"model_id":   brain.ModelID,
		"home":       home,
	})
}

type runTaskAgentPayload struct {
	RecoverySourceAttemptID string `json:"-"`
	// OwnerAuthorized is set only by orchestration that verified a later user
	// turn authorized this continuation.
	OwnerAuthorized bool   `json:"-"`
	RequestID       string `json:"request_id"`
	Revision        string `json:"revision"`
	AgentID         string `json:"agent_id"`
	Prompt          string `json:"prompt"`
	Binary          string `json:"binary"`
	RunnerID        string `json:"runner_id,omitempty"`
	Cols            uint16 `json:"cols"`
	Rows            uint16 `json:"rows"`
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
	if s.agentRepo != nil {
		switch payload.AgentID {
		case "", "@cuan", "cuan", "agent_sm", "@sm", "agent_dev", "@dev", "agent_support", "@support":
			if public, err := s.agentRepo.GetAgentByID(r.Context(), db.AgentIDCuan); err == nil && public != nil {
				payload.AgentID = public.ID
			}
		}
	}
	payload.Prompt = strings.TrimSpace(payload.Prompt)
	payload.Revision = strings.TrimSpace(payload.Revision)
	if len([]rune(payload.Revision)) > task.MaxTaskFeedbackRunes && !s.allowsSavedExecutionRevision(r.Context(), taskID, ownerID, payload.RequestID, payload.Revision) {
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
	var tDetail *task.TaskDetailResponse
	if s.taskSvc != nil {
		if td, err := s.taskSvc.GetTaskDetail(r.Context(), taskID); err == nil && td != nil {
			tDetail = td
			projID = td.ProjectID
			if td.ExecutionCategory != "" {
				taskCategory = td.ExecutionCategory
			}
		}
	}
	if projID == "" && s.projectStudioRepo != nil {
		if pt, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); err == nil && pt != nil {
			projID = pt.ProjectID
		}
	}
	isDedicatedTerminalTask := (tDetail != nil && strings.HasPrefix(tDetail.Title, "Terminal ·")) || strings.HasPrefix(payload.Prompt, "Terminal ·")
	if !isDedicatedTerminalTask && projID != "" && s.isProjectLaneActive(r.Context(), projID, taskCategory, taskID) {
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
	registeredAgent, err := s.runtimeAgent(r.Context(), agentID)
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
	harness := db.NormalizeHarness(registeredAgent.Harness, registeredAgent.Provider)
	switch harness {
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
	needsVision := s.taskRequiresVision(r.Context(), ownerID, tDetail, payload.Prompt, payload.Revision)
	if tDetail != nil && payload.RequestID != "" && s.taskSvc != nil {
		if launch, err := s.taskSvc.GetLaunch(r.Context(), task.LaunchID(taskID, payload.RequestID)); err == nil && launch != nil {
			if inspection, err := task.LiveInspectionForLaunch(&tDetail.Task, launch); err == nil && inspection != nil {
				needsVision = s.runnerClaimRequiresVision(r.Context(), &runner.ClaimTaskResponse{OwnerID: ownerID, LiveInspection: inspection}, tDetail, payload.Revision)
			}
		}
	}
	if !needsVision && tDetail != nil && tDetail.SourceRoomID != "" && tDetail.SourceMessageID != "" && s.projectStudioRepo != nil {
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			if requests, e := store.ListInboxRequests(r.Context(), tDetail.SourceRoomID); e == nil {
				for _, q := range requests {
					if q.MessageID == tDetail.SourceMessageID && q.OwnerID == ownerID {
						for _, p := range s.inboxAttachmentPaths(r.Context(), q) {
							if isImageAttachmentNameOrType(p, "") {
								needsVision = true
								break
							}
						}
						break
					}
				}
			}
		}
	}
	nativePublicTask := expectedBinary == "agy" && registeredAgent.ID == db.AgentIDCuan && (payload.RunnerID == "" || payload.RunnerID == "cuan-runner")
	if nativePublicTask {
		// Admission must use the worker's selected-model and task-slot policy,
		// not the separate chat router's legacy model fallbacks. Release this
		// observation before publishing work so it cannot block the real worker.
		home, _, release, err := agent.LeaseHealthyBrainForTask(ownerID, nil, needsVision)
		if err != nil {
			respondJSON(w, 409, map[string]string{"error": agent.ErrBrainCapacity.Error(), "code": "provider_capacity"})
			return
		}
		release()
		profileHome = home
	} else if expectedBinary == "agy" {
		primary, _ := agent.ProviderAccountForConnection(ownerID, registeredAgent.ID, registeredAgent.ConnectionID)
		limited := primary != nil && primary.State == "quota_limited"
		routes := s.providerRoutes(r.Context(), ownerID, registeredAgent, profileHome, limited)
		if needsVision {
			visionRoutes := make([]inboxAccountRoute, 0, len(routes))
			for _, r := range routes {
				if r.model == "" || agent.IsModelVisionCapable("agy", r.model) {
					visionRoutes = append(visionRoutes, r)
				}
			}
			routes = visionRoutes
		}
		if len(routes) == 0 {
			msg := "No allowed account/model route is available. Check usage and fallback settings in Settings → Agents. The task has not started."
			code := "provider_unavailable"
			if needsVision {
				msg = fmt.Sprintf("Task requires image/visual inspection, but no vision-capable model route is available for agent %s.", registeredAgent.Name)
				code = "provider_vision_unsupported"
			}
			respondJSON(w, 409, map[string]string{"error": msg, "code": code})
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
	if expectedBinary != "agy" && needsVision {
		if !agent.IsModelVisionCapable(expectedBinary, selectedModel) {
			fallbackID := strings.TrimSpace(registeredAgent.FallbackModelID)
			if idx := strings.Index(fallbackID, "#"); idx != -1 {
				fallbackID = fallbackID[idx+1:]
			}
			if fallbackID != "" && agent.IsModelVisionCapable(expectedBinary, fallbackID) {
				selectedModel = fallbackID
			} else {
				respondJSON(w, 409, map[string]string{
					"error": fmt.Sprintf("Task requires image/visual inspection, but agent %s (%s) does not support vision.", registeredAgent.Name, selectedModel),
					"code":  "provider_vision_unsupported",
				})
				return
			}
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
				inspectionPath := ""
				var inspection *task.LiveInspectionIntent
				var inspectionErr error
				if strings.Contains(payload.Revision, "Cuan live-result inspection") {
					inspectionPath, inspection, inspectionErr = s.prepareLiveInspection(r.Context(), detail, payload.RequestID)
				}
				if inspectionErr != nil {
					code, status := "inspection_unavailable", http.StatusConflict
					if errors.Is(inspectionErr, cuangit.ErrRepositoryDownloadUnavailable) || errors.Is(inspectionErr, context.DeadlineExceeded) || errors.Is(inspectionErr, context.Canceled) {
						code, status = "repository_preparing", http.StatusServiceUnavailable
					}
					respondJSON(w, status, map[string]string{"error": inspectionErr.Error(), "code": code})
					return
				}
				if inspection != nil {
					workDir = inspectionPath
				} else {
					project, e := s.projectSvc.TaskContext(r.Context(), detail.ProjectID, detail.Instructions)
					if e != nil || project == nil {
						if e != nil {
							log.Printf("[RunTaskAgent] task=%s project=%s task context unavailable: %v", taskID, detail.ProjectID, e)
						} else {
							log.Printf("[RunTaskAgent] task=%s project=%s task context returned no project", taskID, detail.ProjectID)
						}
						respondJSONError(w, 409, "Task project is unavailable")
						return
					}
					if project.IsArchived {
						respondJSONError(w, 409, "Restore the archived project before starting a task")
						return
					}
					if executionpolicy.StandaloneArtifactWork(detail.Instructions) {
						workDir, e = s.projectSvc.TaskExecutionWorkspace(r.Context(), taskID, detail.Instructions, project)
						if e != nil {
							respondJSONError(w, 503, "The task output workspace could not be prepared")
							return
						}
					} else if project.RepoURL != "" || project.LocalPath != "" {
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
						if detail.LatestAttempt == nil || detail.LatestAttempt.ErrorMessage == "Project verification failed" {
							// A synced cache may predate later pushes. Refresh the bound
							// branch before the first attempt, and when recovering an old
							// verifier. This only moves refs; retained files stay intact.
							if e = s.projectSvc.RefreshExecutionBase(r.Context(), project); e != nil {
								respondJSON(w, 503, map[string]string{"error": e.Error(), "code": "repository_preparing"})
								return
							}
						}
						workDir, e = s.projectSvc.TaskExecutionWorkspace(r.Context(), taskID, detail.Instructions, project)
						if errors.Is(e, projectpkg.ErrExecutionWorkspaceMissing) && revision != "" && detail.LatestAttempt != nil {
							workDir, e = s.projectSvc.ReconstructTaskExecutionWorkspace(r.Context(), taskID, project)
						}
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
			}

			if taskPrompt == "" {
				taskPrompt = executionpolicy.NormalizeManagedInstructions(detail.Instructions)
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
				{"Human instructions", executionpolicy.NormalizeManagedInstructions(detail.Instructions)}, {"Acceptance criteria", criteria.String()}, {"Previous work summary", detail.Summary},
				{"Previous completion notes", detail.CompleteNote}, {"Revision feedback", revision},
			} {
				if strings.TrimSpace(section.text) != "" {
					sessionContext += "\n## " + section.label + "\n" + section.text + "\n"
				}
			}
			sessionContext += taskRecoveryContext(detail)
			sessionContext += projectpkg.WorkspaceReconstructionContext(workDir)
			if revision != "" {
				taskPrompt = revision
			}
			runnerID := strings.TrimSpace(payload.RunnerID)
			storagePath := workDir
			if storagePath == "" && s.cfg != nil {
				storagePath = s.cfg.WorkspacesDir
			}
			if taskStorageCritical(getSystemResources(storagePath)) {
				s.requestStorageMaintenance()
				respondJSON(w, 503, map[string]string{"code": "execution_storage", "error": "Server storage is temporarily full. Saved work will continue after space is available."})
				return
			}
			if runnerID == "" {
				if agentID == "agent_lifecycle" || agentID == "agent_queued" || (detail != nil && strings.HasPrefix(detail.Title, "Terminal ·")) {
					runnerID = "local_terminal"
				} else {
					runnerID = "cuan-runner"
				}
			}
			// Save feedback before exposing work to the queue. Once published,
			// an attempt belongs to its worker and must not be reset by this handler.
			if revision != "" {
				if _, e := s.taskSvc.AddComment(r.Context(), taskID, "owner", "Revision requested:\n"+revision); e != nil {
					respondJSONError(w, 503, "Could not save revision feedback. Execution did not start.")
					return
				}
			}
			attempt, err = s.taskSvc.CreateAttempt(r.Context(), taskID, "owner", task.CreateAttemptRequest{RunnerID: runnerID, AgentName: agentID, RequestID: payload.RequestID, RequestHash: requestHash, ExpectedPredecessorID: payload.RecoverySourceAttemptID, InitializeQueuedTask: runnerID == "cuan-runner", PreserveQueuedReport: revision != "", OwnerAuthorized: payload.OwnerAuthorized})
			if errors.Is(err, task.ErrTaskActive) {
				respondJSONError(w, 409, task.ActiveAttemptLaunchError)
				return
			}
			if errors.Is(err, task.ErrRequestConflict) && payload.RecoverySourceAttemptID != "" {
				respondJSON(w, 409, map[string]string{"code": "recovery_superseded", "error": supersededRecoveryLaunchReason})
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
			var updateSummary *string
			var updateCompleteNote *string
			resetCriteria := append([]task.DoDItem{}, detail.DoDChecklist...)
			if revision == "" {
				emptyReport := ""
				updateSummary = &emptyReport
				updateCompleteNote = &emptyReport
				for i := range resetCriteria {
					resetCriteria[i].Completed = false
				}
			}

			// If delegated to cuan-runner, attempt stays in AttemptStatusQueued and task is queued for worker daemon
			if runnerID == "cuan-runner" {
				// Metadata and queue visibility were committed atomically. Only
				// observe worker state after publication; never overwrite it.
				if s.acknowledgeClaimedWorkerAttempt(w, r.Context(), taskID, attempt.ID) {
					return
				}
				respondJSON(w, http.StatusOK, map[string]interface{}{
					"status":     task.AttemptStatusQueued,
					"attempt_id": attempt.ID,
					"task_id":    taskID,
					"runner_id":  "cuan-runner",
				})
				return
			}

			doing := task.TaskStatusDoing
			if _, err = s.taskSvc.UpdateTask(r.Context(), taskID, "owner", task.UpdateTaskRequest{Status: &doing, AssignedAgent: &agentID, DoDChecklist: &resetCriteria, Summary: updateSummary, CompleteNote: updateCompleteNote}); err != nil {
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
			// Native runners add guidance in BuildRunnerPromptWithDoD. Local
			// terminal launches use this entry point and need the target copy.
			if detail != nil && !executionpolicy.StandaloneArtifactWork(detail.Instructions) {
				taskPrompt = cuanPrompt.GetRunnerPromptAdditions(workDir) + "\n\n" + taskPrompt
			}
		}
	}
	var repositoryBaseline string
	if attempt != nil && detail != nil {
		limits, policyErr := s.taskSvc.AttemptPublicationPolicy(r.Context(), taskID, attempt.ID)
		if policyErr == nil && limits.ReadOnlyRepository {
			repositoryBaseline, policyErr = runner.ObserveRepositoryScope(r.Context(), workDir)
			taskPrompt += limits.Prompt()
		}
		if policyErr != nil {
			_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Read-only repository baseline could not be verified. Execution did not start."})
			_ = s.taskSvc.ReconcileLocalTask(context.Background(), taskID, attempt.ID)
			reportCleanup()
			respondJSONError(w, 409, "Read-only repository baseline could not be verified. Existing files were preserved.")
			return
		}
	}
	var quotaHit atomic.Bool
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
					_ = output.recordQuotaCooldown(ownerID, selectedAccountID, selectedModel)
				}
				sess, ok := s.terminalManager.GetSession(taskID)
				if ok && sess != nil && sess.IsPersistent() {
					quotaHit.Store(true)
					nextHome, nextBrain, nextRelease, err := agent.LeaseHealthyBrainForTask(ownerID, map[string]bool{selectedAccountID: true})
					if err == nil && nextBrain != nil {
						defer nextRelease()
						selectedAccountID = nextBrain.ID
						selectedModel = nextBrain.ModelID
						profileHome = nextHome
						_ = agent.PrepareAntigravitySettings(profileHome, workDir)
						notice := fmt.Sprintf("\r\n\x1b[33m[cuan]\x1b[0m AI model quota limited (429). Rotated automatically to healthy brain %s (%s). Active terminal session preserved.\r\n", selectedModel, selectedAccountID)
						sess.BroadcastBytes([]byte(notice))
					}
					return
				}
				_ = s.terminalManager.StopQuotaAttempt(taskID, attempt.ID)
				quotaHit.Store(true)
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
			Model:             selectedModel,
			AttachmentPaths:   attachmentPaths,
			AttemptID:         attemptID,
			TaskID:            taskID,
			ProjectID:         projID,
			ExecutionCategory: taskCategory,
			AgentID:           agentID,
			AgentName:         agentName,
			Command:           payload.Binary,
			Prompt:            taskPrompt,
			WorkDir:           workDir,
			Cols:              payload.Cols,
			Rows:              payload.Rows,
			Mode:              terminal.ModeInteractive,
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
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if s.projectStudioRepo != nil {
					_ = s.projectStudioRepo.UpdateTaskExecution(ctx, taskID, h.Status, fmt.Sprintf("Runner exited with code %d (%s). Review the result and verification evidence.", h.ExitCode, h.Status))
				}
				if attempt != nil && s.taskSvc != nil {
					state := task.AttemptStatusFailed
					note := output.failureNote()
					ready := false
					if h.EndReason != "provider_quota" && h.EndReason != "provider_restricted" {
						isReady, e := s.taskSvc.AgentReviewReady(ctx, taskID, attempt.ID)
						ready = e == nil && isReady
					}
					if h.EndReason == "agent_report" || (ready && h.ExitCode == 0) {
						state = task.AttemptStatusSucceeded
						note = ""
					} else if h.ExitCode == 0 && h.EndReason != "provider_quota" && h.EndReason != "provider_restricted" {
						state = task.AttemptStatusFailed
						note = "No review-ready result was recorded. Check the agent report and saved output."
					}
					var updatedAttempt *task.TaskAttempt
					var e error
					if repositoryBaseline != "" {
						final, observeErr := runner.ObserveRepositoryScope(ctx, workDir)
						if observeErr == nil && final == repositoryBaseline {
							observeErr = s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "repository-scope:" + attempt.ID, TaskID: taskID, AttemptID: &attempt.ID, Author: attempt.RunnerID, ActivityType: "repository_scope_verified", NewStatus: "read_only", Message: final, CreatedAt: time.Now().UTC()})
						}
						if observeErr != nil || final != repositoryBaseline {
							_, _ = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Read-only repository state changed or could not be verified. Existing files were preserved."})
							_ = s.taskSvc.ReconcileLocalTask(ctx, taskID, attempt.ID)
							return
						}
					}
					if h.EndReason == "agent_report" || (ready && h.ExitCode == 0) {
						updatedAttempt, e = s.taskSvc.FinishReportedAttempt(ctx, attempt.ID, h.ExitCode)
					} else {
						updatedAttempt, e = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: state, ExitCode: &h.ExitCode, ErrorMessage: note})
					}
					if e == nil {
						// Completion outlives the HTTP request that launched the terminal.
						_ = s.taskSvc.ReconcileLocalTask(ctx, taskID, attempt.ID)
					}
					_ = s.taskSvc.FinishTaskLaunches(context.Background(), taskID, "stopped")
					s.kickTaskLaunches()
					if e == nil && updatedAttempt != nil && (updatedAttempt.Status == task.AttemptStatusSucceeded || updatedAttempt.Status == task.AttemptStatusFailed) {
						s.onRunnerAttemptCompleted(ctx, updatedAttempt)
					}
					s.kickInboxTaskUpdates(context.Background())
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
		if projID != "" && s.projectSvc != nil {
			if p, pErr := s.projectSvc.Get(r.Context(), projID); pErr == nil && p != nil {
				settings := projectpkg.SettingsFromMetadata(p.Metadata)
				if settings.Environment != "" {
					runCfg.DelegatedEnv = projectpkg.ParseEnvironment(settings.Environment)
					for k, v := range projectpkg.ParseEnvironment(settings.Environment) {
						runCfg.Env = append(runCfg.Env, fmt.Sprintf("%s=%s", k, v))
					}
				}
			}
		}
		if attempt != nil && routeNote != "" {
			if err := s.taskSvc.Store().CreateActivity(r.Context(), &task.TaskActivity{ID: "provider-route:" + attempt.ID, TaskID: taskID, AttemptID: &attempt.ID, ActivityType: "provider_route", Author: "runner", Message: routeNote, CreatedAt: time.Now().UTC()}); err != nil {
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

		if !quotaHit.Load() {
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
		quotaHit.Store(false)
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
	// The decoupled runner owns its PTY; only expose the current attempt socket.
	if live == nil && s.taskSvc != nil {
		if detail, err := s.taskSvc.GetTaskDetail(r.Context(), taskID); err == nil && detail != nil && detail.LatestAttempt != nil {
			att := detail.LatestAttempt
			if att.Status == task.AttemptStatusRunning && terminal.SocketExists(att.ID) {
				live = map[string]any{"session_id": "runner-sess-" + att.ID, "attempt_id": att.ID, "agent_id": att.AgentName, "started_at": att.StartedAt}
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

func (s *Server) taskRequiresVision(ctx context.Context, ownerID string, detail *task.TaskDetailResponse, prompt, revision string) bool {
	checkText := func(text string) bool {
		if text == "" {
			return false
		}
		if strings.Contains(text, "Images require visual inspection") {
			return true
		}
		// A ban on producing screenshots is not a request for image tools or
		// frontend setup. Keep later positive requirements and attachments.
		text = executionpolicy.WithoutVisualExclusions(text)
		text = visualCaptureProhibited.ReplaceAllString(text, "")
		lower := strings.ToLower(text)
		if strings.Contains(lower, "screenshot") || strings.Contains(lower, "tangkapan layar") {
			return true
		}
		for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
			if strings.Contains(lower, ext) {
				return true
			}
		}
		return false
	}

	if checkText(prompt) || checkText(revision) {
		return true
	}

	if detail != nil {
		for _, criterion := range detail.DoDChecklist {
			if checkText(criterion.Text) {
				return true
			}
		}
		if checkText(projectpkg.TaskScopeInstructions(detail.Instructions)) || checkText(detail.Title) {
			return true
		}
		delivered := taskDeliveredAttachmentIDs(detail)
		for _, att := range detail.Attachments {
			if ca, ok := att.(conversationAttachment); ok {
				if delivered[ca.ID] {
					continue
				}
				if isImageAttachmentNameOrType(ca.Name, ca.Type) {
					return true
				}
			} else if m, ok := att.(map[string]any); ok {
				if id, _ := m["id"].(string); delivered[id] {
					continue
				}
				name, _ := m["name"].(string)
				mime, _ := m["type"].(string)
				if isImageAttachmentNameOrType(name, mime) {
					return true
				}
			}
		}
		if s.projectStudioRepo != nil && detail.SourceRoomID != "" && detail.SourceMessageID != "" {
			if msg, err := s.projectStudioRepo.GetMessageByID(ctx, detail.SourceMessageID); err == nil && msg != nil {
				_, attIDs := decodeAttachmentMessage(msg.Content)
				for _, aid := range attIDs {
					if a, _, err := loadAttachment(ownerID, aid); err == nil {
						if isImageAttachmentNameOrType(a.Name, a.Type) {
							return true
						}
					} else if a, _, err := findAttachmentAcrossOwners(aid); err == nil {
						if isImageAttachmentNameOrType(a.Name, a.Type) {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

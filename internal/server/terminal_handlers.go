package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

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

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

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
	if history := s.terminalManager.GetTaskHistory(taskID); len(history) > 0 && history[len(history)-1].Status == "running" {
		respondJSONError(w, 409, "This task already has an active session. Open its terminal.")
		return
	}
	agentID := strings.TrimSpace(payload.AgentID)
	if agentID == "" {
		respondJSONError(w, http.StatusBadRequest, "Select a registered agent first")
		return
	}
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Agent registry unavailable")
		return
	}
	registeredAgent, err := s.agentRepo.GetAgentByID(r.Context(), agentID)
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
	agentName := registeredAgent.Name

	taskPrompt := strings.TrimSpace(payload.Prompt)
	workDir := ""
	sessionContext := ""
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
	if s.taskSvc != nil {
		if detail, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil && detail != nil {
			taskFound = true
			if detail.Status == task.TaskStatusReview && payload.Revision == "" {
				respondJSONError(w, 400, "Describe the changes needed before starting a revision")
				return
			}
			if detail.Status == task.TaskStatusDone {
				respondJSONError(w, 409, "Reopen the completed task before starting a new session")
				return
			}
			sourceRoom = detail.SourceRoomID
			if detail.ProjectID != "" && s.projectSvc != nil {
				project, e := s.projectSvc.Get(r.Context(), detail.ProjectID)
				if e != nil || project == nil {
					respondJSONError(w, 409, "Task project is unavailable")
					return
				}
				if project.IsArchived {
					respondJSONError(w, 409, "Restore the archived project before starting a task")
					return
				}
				if project.RepoURL != "" {
					info, e := os.Stat(project.LocalPath)
					if project.LocalPath == "" || e != nil || !info.IsDir() {
						respondJSONError(w, 409, "Project repository is not ready. Sync it before running the task.")
						return
					}
					workDir = project.LocalPath
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
				{"Previous completion notes", detail.CompleteNote}, {"Revision feedback", payload.Revision},
			} {
				if strings.TrimSpace(section.text) != "" {
					sessionContext += "\n## " + section.label + "\n" + section.text + "\n"
				}
			}
			if strings.TrimSpace(payload.Revision) != "" {
				taskPrompt = payload.Revision
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
			if payload.Revision != "" {
				if _, e := s.taskSvc.AddComment(r.Context(), taskID, "owner", "Revision requested:\n"+payload.Revision); e != nil {
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
		sessionContext += "\n## Workspace\n" + workDir + "\n"
		var extra string
		reportEnv, extra, reportToken, reportCleanup, err = s.issueTaskAgent(r.Context(), taskID, attempt.ID, agentID, sessionContext)
		if err != nil {
			_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Task reporting tools could not be prepared. Execution did not start."})
			_ = s.taskSvc.ReconcileLocalTask(context.Background(), taskID, attempt.ID)
			respondJSONError(w, 503, "Task reporting tools unavailable. The process has not started.")
			return
		}
		taskPrompt += extra
	}
	launched := false
	defer func() {
		if !launched {
			reportCleanup()
		}
	}()
	var output *taskOutputRecorder
	if attempt != nil {
		output = newTaskOutputRecorder(s.taskSvc, attempt.ID)
		output.secrets = append(output.secrets, reportToken)
	}
	// 3. Spawn PTY process via Manager
	runCfg := terminal.TaskAgentRunConfig{
		TaskID:    taskID,
		AgentID:   agentID,
		AgentName: agentName,
		Command:   payload.Binary,
		Prompt:    taskPrompt,
		WorkDir:   workDir,
		Cols:      payload.Cols,
		Rows:      payload.Rows,
		Mode:      terminal.ModeInteractive,
		OnOutput: func(chunk []byte) {
			if output != nil {
				output.append(chunk)
			}
		},
		OnExit: func(h *terminal.TaskRunHistory) {
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
				if h.ExitCode == 0 || h.EndReason == "agent_report" {
					state = task.AttemptStatusSucceeded
				}
				note := output.failureNote()
				if h.EndReason == "agent_report" {
					note = ""
				}
				if h.ExitCode == 0 {
					ready, e := s.taskSvc.AgentReviewReady(ctx, taskID, attempt.ID)
					if e != nil || !ready {
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
					_ = s.taskSvc.ReconcileLocalTask(ctx, taskID, attempt.ID)
				}
			}
		},
	}

	if attempt != nil {
		runCfg.AttemptID = attempt.ID
	}
	if profileHome != "" {
		runCfg.Env = agent.AntigravityProfileEnvironment(profileHome)
	}
	runCfg.Env = append(runCfg.Env, reportEnv...)
	sess, err := s.terminalManager.RunTaskAgent(r.Context(), runCfg)
	if err != nil {
		if attempt != nil {
			_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Could not start the CLI. Check its installation and the agent connection."})
			status := task.TaskStatusBlocked
			_, _ = s.taskSvc.UpdateTask(context.Background(), taskID, "runner", task.UpdateTaskRequest{Status: &status})
		}
		if s.projectStudioRepo != nil {
			_ = s.projectStudioRepo.UpdateTaskExecution(context.Background(), taskID, "failed", "Could not start the CLI.")
		}
		respondJSONError(w, http.StatusInternalServerError, "Could not start the CLI. Check its installation and the agent connection.")
		return
	}

	if sourceRoom != "" {
		s.publishInbox(sourceRoom)
	}
	launched = true

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

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	history := s.terminalManager.GetTaskHistory(taskID)
	var live map[string]any
	if session, ok := s.terminalManager.GetSession(taskID); ok {
		for _, h := range history {
			if h.SessionID == session.ID() && h.Status == "running" {
				live = map[string]any{"session_id": session.ID(), "attempt_id": h.AttemptID, "agent_id": h.AgentID, "started_at": h.StartedAt}
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

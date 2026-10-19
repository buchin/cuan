package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

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

	// Verify task exists if repositories are configured
	if s.projectStudioRepo != nil {
		// If valid UUID format, check project tasks repository
		if _, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); err == nil {
			// Found in projectStudioRepo
		} else if s.taskSvc != nil {
			// Check runner tasks
			if _, err := s.taskSvc.GetTaskDetail(r.Context(), taskID); err == nil {
				// Found in taskSvc
			}
		}
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
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
	Binary  string `json:"binary"`
	Cols    uint16 `json:"cols"`
	Rows    uint16 `json:"rows"`
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
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	agentID := strings.TrimSpace(payload.AgentID)
	if agentID == "" {
		respondJSONError(w, http.StatusBadRequest, "Pilih agen terdaftar terlebih dahulu")
		return
	}
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Agent registry unavailable")
		return
	}
	registeredAgent, err := s.agentRepo.GetAgentByID(r.Context(), agentID)
	if err != nil || registeredAgent == nil {
		respondJSONError(w, http.StatusNotFound, "Agen tidak terdaftar")
		return
	}
	if !registeredAgent.Enabled {
		respondJSONError(w, http.StatusConflict, "Agen dinonaktifkan")
		return
	}
	agentName := registeredAgent.Name
	if payload.Binary == "" {
		switch registeredAgent.Harness {
		case "claude":
			payload.Binary = "claude"
		case "codex":
			payload.Binary = "codex"
		default:
			payload.Binary = "agy"
		}
	}

	taskPrompt := strings.TrimSpace(payload.Prompt)
	workDir := ""

	// 1. Check project tasks repository
	if s.projectStudioRepo != nil {
		if pt, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); err == nil && pt != nil {
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

	// 2. Check runner task service
	if s.taskSvc != nil {
		if tDetail, err := s.taskSvc.GetTaskDetail(r.Context(), taskID); err == nil && tDetail != nil {
			if taskPrompt == "" {
				taskPrompt = tDetail.Instructions
			}
			statusDoing := task.TaskStatusDoing
			_, _ = s.taskSvc.UpdateTask(r.Context(), taskID, "system", task.UpdateTaskRequest{
				Status:        &statusDoing,
				AssignedAgent: &agentID,
			})
		}
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
		OnExit: func(h *terminal.TaskRunHistory) {
			if s.projectStudioRepo != nil {
				_ = s.projectStudioRepo.UpdateTaskExecution(context.Background(), taskID, h.Status, fmt.Sprintf("Runner exited with code %d (%s)", h.ExitCode, h.Status))
			}
			if s.taskSvc != nil {
				if h.Status == "completed" {
					statusReview := task.TaskStatusReview
					_, _ = s.taskSvc.UpdateTask(context.Background(), taskID, "runner", task.UpdateTaskRequest{
						Status: &statusReview,
					})
				}
			}
		},
	}

	sess, err := s.terminalManager.RunTaskAgent(r.Context(), runCfg)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to spawn agent runner: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "running",
		"session_id":   sess.ID(),
		"task_id":      taskID,
		"pid":          sess.PTY().PID(),
		"terminal_url": fmt.Sprintf("/api/v1/tasks/%s/terminal", taskID),
		"agent_id":     agentID,
		"agent_name":   agentName,
	})
}

// handleGetTaskRunHistory returns execution history of an agent runner on a task.
// GET /api/v1/tasks/{id}/run-agent/history
func (s *Server) handleGetTaskRunHistory(w http.ResponseWriter, r *http.Request) {
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
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"task_id": taskID,
		"history": history,
		"count":   len(history),
	})
}

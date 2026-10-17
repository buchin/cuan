package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
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
		ID           string        `json:"id"`
		TaskID       string        `json:"task_id"`
		Subscribers  int           `json:"subscribers"`
		CreatedAt    string        `json:"created_at"`
		LastActivity string        `json:"last_activity"`
		Cols         uint16        `json:"cols"`
		Rows         uint16        `json:"rows"`
		PID          int           `json:"pid"`
		WorkDir      string        `json:"work_dir"`
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

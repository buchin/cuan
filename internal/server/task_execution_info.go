package server

import (
	"errors"
	"net/http"

	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleTaskExecutionInfo(w http.ResponseWriter, r *http.Request) {
	info, err := s.taskSvc.ExecutionInfo(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, 404, "Task not found")
			return
		}
		respondJSONError(w, 503, "Saved execution context could not be read")
		return
	}
	respondJSON(w, 200, info)
}

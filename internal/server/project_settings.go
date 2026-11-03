package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleProjectSettings(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	var request project.UpdateSettingsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		respondJSONError(w, 400, "Enter valid project settings.")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		respondJSONError(w, 400, "Submit one project settings object.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	result, err := s.projectSvc.UpdateSettings(ctx, chi.URLParam(r, "id"), request)
	if err != nil {
		switch {
		case errors.Is(err, project.ErrSettingsConflict):
			respondJSONError(w, 409, err.Error())
		case errors.Is(err, project.ErrProjectNotFound):
			respondJSONError(w, 404, "Project not found.")
		default:
			respondJSONError(w, 400, err.Error())
		}
		return
	}
	respondJSON(w, 200, result)
}

type liveProjectFinder struct{ server *Server }

func (f liveProjectFinder) Get(ctx context.Context, id string) (*project.ProjectResponse, error) {
	if f.server.projectSvc == nil {
		return nil, project.ErrProjectNotFound
	}
	return f.server.projectSvc.Get(ctx, id)
}

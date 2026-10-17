package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/publisher"
)

// SetupWordPressIntegrationPayload defines the request body for configuring WordPress.
type SetupWordPressIntegrationPayload struct {
	SiteURL             string `json:"site_url"`
	Username            string `json:"username"`
	ApplicationPassword string `json:"application_password"`
	TestConnection      *bool  `json:"test_connection,omitempty"`
}

// PublishTaskPayload defines the request body for publishing a task to WordPress.
type PublishTaskPayload struct {
	Title           string   `json:"title,omitempty"`
	Content         string   `json:"content,omitempty"`
	Status          string   `json:"status,omitempty"` // "draft", "publish", "pending"
	Tags            []string `json:"tags,omitempty"`
	ConvertMarkdown *bool    `json:"convert_markdown,omitempty"`
}

// handleSetupWordPressIntegration configures and tests the WordPress REST API connection for a project.
// POST /api/v1/projects/{id}/integrations/wordpress
func (s *Server) handleSetupWordPressIntegration(w http.ResponseWriter, r *http.Request) {
	if s.publisherEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "publisher engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var payload SetupWordPressIntegrationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	testConn := true
	if payload.TestConnection != nil {
		testConn = *payload.TestConnection
	}

	cfg := publisher.WordPressConfig{
		SiteURL:             payload.SiteURL,
		Username:            payload.Username,
		ApplicationPassword: payload.ApplicationPassword,
	}

	dto, err := s.publisherEngine.SetupWordPressIntegration(r.Context(), projectID, cfg, testConn)
	if err != nil {
		// Zero credential leakage
		errMsg := err.Error()
		if payload.ApplicationPassword != "" {
			errMsg = strings.ReplaceAll(errMsg, payload.ApplicationPassword, "[REDACTED]")
		}
		respondJSONError(w, http.StatusBadRequest, errMsg)
		return
	}

	respondJSON(w, http.StatusOK, dto)
}

// handleGetWordPressIntegration retrieves the WordPress integration status for a project.
// GET /api/v1/projects/{id}/integrations/wordpress
func (s *Server) handleGetWordPressIntegration(w http.ResponseWriter, r *http.Request) {
	if s.publisherEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "publisher engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dto, err := s.publisherEngine.GetWordPressIntegration(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, db.ErrIntegrationNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress integration not configured for this project")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get wordpress integration: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, dto)
}

// handlePublishTask publishes a task deliverable (agent_output) directly to WordPress.
// POST /api/v1/tasks/{id}/publish
func (s *Server) handlePublishTask(w http.ResponseWriter, r *http.Request) {
	if s.publisherEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "publisher engine is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var payload PublishTaskPayload
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	convertMD := true
	if payload.ConvertMarkdown != nil {
		convertMD = *payload.ConvertMarkdown
	}

	opts := publisher.PublishTaskOptions{
		Title:           payload.Title,
		Content:         payload.Content,
		Status:          payload.Status,
		Tags:            payload.Tags,
		ConvertMarkdown: convertMD,
	}

	res, err := s.publisherEngine.PublishTask(r.Context(), taskID, opts)
	if err != nil {
		if errors.Is(err, publisher.ErrIntegrationNotFound) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task not found")
			return
		}
		if errors.Is(err, publisher.ErrEmptyTaskContent) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusBadRequest, "failed to publish task to wordpress: "+err.Error())
		return
	}

	// Broadcast task.published event to room if roomEventBroker is available
	if s.roomEventBroker != nil && s.projectStudioRepo != nil {
		if task, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); err == nil && task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
			s.roomEventBroker.Broadcast(*task.RoomID, "task.published", task.ToDTO())
		}
	}

	respondJSON(w, http.StatusOK, res)
}

// handleExportTask exports a task in the requested format (md, html, txt).
// GET /api/v1/tasks/{id}/export?format={md|html|txt}
func (s *Server) handleExportTask(w http.ResponseWriter, r *http.Request) {
	if s.publisherEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "publisher engine is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = "md"
	}

	res, err := s.publisherEngine.ExportTask(r.Context(), taskID, format)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task not found")
			return
		}
		if errors.Is(err, publisher.ErrUnsupportedExportFormat) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to export task: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", res.Filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(res.Content))
}

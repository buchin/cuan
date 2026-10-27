package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/creator"
	"github.com/buchin/cuan/internal/db"
)

type instantiateRequestPayload struct {
	TemplateID    string          `json:"template_id"`
	BlueprintID   string          `json:"blueprint_id"`
	Type          string          `json:"type"`
	ProjectName   string          `json:"project_name"`
	Name          string          `json:"name"`
	RepoURL       string          `json:"repo_url,omitempty"`
	DefaultBranch string          `json:"default_branch,omitempty"`
	Token         string          `json:"token,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// handleListCreatorTemplates returns the catalog of 4 creator blueprint templates.
// Routes: GET /api/creator/templates, GET /api/v1/creator/templates
func (s *Server) handleListCreatorTemplates(w http.ResponseWriter, r *http.Request) {
	var templates []creator.Template
	if s.blueprintEngine != nil {
		templates = s.blueprintEngine.ListTemplates()
	} else {
		templates = creator.DefaultTemplates()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "success",
		"templates": templates,
		"count":     len(templates),
	})
}

// handleGetCreatorTemplate retrieves a single blueprint template by ID or synonym.
// Routes: GET /api/creator/templates/{id}, GET /api/v1/creator/templates/{id}
func (s *Server) handleGetCreatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "template id is required")
		return
	}

	var tmpl *creator.Template
	var err error

	if s.blueprintEngine != nil {
		tmpl, err = s.blueprintEngine.GetTemplate(id)
	} else {
		var ok bool
		tmpl, ok = creator.LookupTemplate(id)
		if !ok {
			err = fmt.Errorf("%w: %s", creator.ErrInvalidTemplateID, id)
		}
	}

	if err != nil {
		if errors.Is(err, creator.ErrInvalidTemplateID) {
			respondJSONError(w, http.StatusNotFound, fmt.Sprintf("creator blueprint template '%s' not found", id))
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get creator template: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"template": tmpl,
	})
}

// handleInstantiateCreatorProject atomically creates a project, room, tasks pipeline, and agent persona.
// Routes: POST /api/creator/projects/instantiate, POST /api/v1/creator/projects/instantiate
func (s *Server) handleInstantiateCreatorProject(w http.ResponseWriter, r *http.Request) {
	if s.blueprintEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "creator blueprint engine is not configured")
		return
	}

	var payload instantiateRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	// Resolve template ID across synonyms
	templateID := strings.TrimSpace(payload.TemplateID)
	if templateID == "" {
		templateID = strings.TrimSpace(payload.BlueprintID)
	}
	if templateID == "" {
		templateID = strings.TrimSpace(payload.Type)
	}

	if templateID == "" {
		respondJSONError(w, http.StatusBadRequest, "template_id is required")
		return
	}

	// Resolve project name
	projectName := strings.TrimSpace(payload.ProjectName)
	if projectName == "" {
		projectName = strings.TrimSpace(payload.Name)
	}

	req := creator.InstantiateRequest{
		TemplateID:    templateID,
		ProjectName:   projectName,
		RepoURL:       strings.TrimSpace(payload.RepoURL),
		DefaultBranch: strings.TrimSpace(payload.DefaultBranch),
		Token:         strings.TrimSpace(payload.Token),
		Metadata:      payload.Metadata,
	}

	resp, err := s.blueprintEngine.Instantiate(r.Context(), req)
	if err != nil {
		if errors.Is(err, creator.ErrInvalidTemplateID) {
			respondJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid creator blueprint template ID: %s", templateID))
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to instantiate creator project: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

type executeTaskAgentRequestPayload struct {
	Instructions string `json:"instructions,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	ModelID      string `json:"model_id,omitempty"`
	Provider     string `json:"provider,omitempty"`
}

// handleExecuteTaskAgent executes an AI agent for a specific project task.
// Routes: POST /api/v1/tasks/{id}/execute-agent, POST /api/tasks/{id}/execute-agent
// Aliases: POST /api/projects/tasks/{id}/execute-agent, POST /api/v1/projects/tasks/{id}/execute-agent
func (s *Server) handleExecuteTaskAgent(w http.ResponseWriter, r *http.Request) {
	if s.taskExecutionEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "task execution engine is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}
	if !db.IsValidUUID(taskID) {
		respondJSONError(w, http.StatusBadRequest, "invalid task uuid format")
		return
	}

	var payload executeTaskAgentRequestPayload
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	req := creator.ExecuteTaskRequest{
		TaskID:       taskID,
		UserID:       strings.TrimSpace(payload.UserID),
		Instructions: strings.TrimSpace(payload.Instructions),
		ModelID:      strings.TrimSpace(payload.ModelID),
		Provider:     strings.TrimSpace(payload.Provider),
	}

	resp, err := s.taskExecutionEngine.ExecuteTask(r.Context(), req)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "project task not found")
			return
		}
		if errors.Is(err, creator.ErrMissingGeminiKey) {
			respondJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
				"status":   "error",
				"code":     "MISSING_GEMINI_KEY",
				"error":    err.Error(),
				"help_url": "/studio/agents",
			})
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to execute task with agent: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, resp)
}


package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/wordpress"
)

// WordPressDeployRequestBody defines the JSON payload for deploying a WordPress site.
type WordPressDeployRequestBody struct {
	SiteTitle          string  `json:"site_title"`
	AdminEmail         string  `json:"admin_email"`
	AdminUsername      string  `json:"admin_username"`
	AdminPassword      string  `json:"admin_password,omitempty"`
	Domain             string  `json:"domain,omitempty"`
	DBName             string  `json:"db_name,omitempty"`
	DBUser             string  `json:"db_user,omitempty"`
	DBPassword         string  `json:"db_password,omitempty"`
	TablePrefix        string  `json:"table_prefix,omitempty"`
	WPVersion          string  `json:"wp_version,omitempty"`
	PermalinkStructure string  `json:"permalink_structure,omitempty"`
	ServerID           *string `json:"server_id,omitempty"`
	SkipHealthWait     *bool   `json:"skip_health_wait,omitempty"`
}

// handleWordPressDeploy launches a turnkey WordPress deployment.
// POST /api/projects/{id}/wordpress/deploy or POST /api/v1/projects/{id}/wordpress/deploy
func (s *Server) handleWordPressDeploy(w http.ResponseWriter, r *http.Request) {
	if s.wordpressOrchestrator == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress orchestrator engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var reqBody WordPressDeployRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	skipHealth := false
	if reqBody.SkipHealthWait != nil {
		skipHealth = *reqBody.SkipHealthWait
	}

	deployReq := wordpress.DeployRequest{
		ProjectID:          projectID,
		SiteTitle:          reqBody.SiteTitle,
		AdminEmail:         reqBody.AdminEmail,
		AdminUsername:      reqBody.AdminUsername,
		AdminPassword:      reqBody.AdminPassword,
		Domain:             reqBody.Domain,
		DBName:             reqBody.DBName,
		DBUser:             reqBody.DBUser,
		DBPassword:         reqBody.DBPassword,
		TablePrefix:        reqBody.TablePrefix,
		WPVersion:          reqBody.WPVersion,
		PermalinkStructure: reqBody.PermalinkStructure,
		ServerID:           reqBody.ServerID,
		SkipHealthWait:     skipHealth,
	}

	res, err := s.wordpressOrchestrator.Deploy(r.Context(), deployReq)
	if err != nil {
		if errors.Is(err, wordpress.ErrInvalidProjectID) ||
			errors.Is(err, wordpress.ErrInvalidSiteTitle) ||
			errors.Is(err, wordpress.ErrInvalidAdminEmail) ||
			errors.Is(err, wordpress.ErrInvalidAdminUsername) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to deploy wordpress: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

// handleGetWordPressDeployment retrieves the deployment details for a project.
// GET /api/projects/{id}/wordpress or GET /api/v1/projects/{id}/wordpress
func (s *Server) handleGetWordPressDeployment(w http.ResponseWriter, r *http.Request) {
	if s.wordpressOrchestrator == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress orchestrator engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.wordpressOrchestrator.GetDeploymentByProjectID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, dep.ToDTO())
}

// handleRestartWordPressDeployment restarts the WordPress runtime container/process.
// POST /api/projects/{id}/wordpress/restart or POST /api/v1/projects/{id}/wordpress/restart
func (s *Server) handleRestartWordPressDeployment(w http.ResponseWriter, r *http.Request) {
	if s.wordpressOrchestrator == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress orchestrator engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.wordpressOrchestrator.GetDeploymentByProjectID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to find deployment: "+err.Error())
		return
	}

	res, err := s.wordpressOrchestrator.RestartDeployment(r.Context(), dep.ID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to restart wordpress: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, res)
}

// handleHealthCheckWordPressDeployment probes the active WordPress site and updates health status.
// GET /api/projects/{id}/wordpress/health or GET /api/v1/projects/{id}/wordpress/health
func (s *Server) handleHealthCheckWordPressDeployment(w http.ResponseWriter, r *http.Request) {
	if s.wordpressOrchestrator == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress orchestrator engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.wordpressOrchestrator.GetDeploymentByProjectID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to find deployment: "+err.Error())
		return
	}

	hcRes, err := s.wordpressOrchestrator.HealthCheck(r.Context(), dep.ID)
	if err != nil && hcRes == nil {
		respondJSONError(w, http.StatusInternalServerError, "health check probe failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"project_id":    projectID,
		"deployment_id": dep.ID,
		"health_status": dep.HealthStatus,
		"probe":         hcRes,
	})
}

// handleStopWordPressDeployment terminates the running WordPress instance and releases port.
// POST /api/projects/{id}/wordpress/stop or POST /api/v1/projects/{id}/wordpress/stop
func (s *Server) handleStopWordPressDeployment(w http.ResponseWriter, r *http.Request) {
	if s.wordpressOrchestrator == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress orchestrator engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.wordpressOrchestrator.GetDeploymentByProjectID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to find deployment: "+err.Error())
		return
	}

	if err := s.wordpressOrchestrator.StopDeployment(r.Context(), dep.ID); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to stop wordpress: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "wordpress deployment stopped successfully",
		"project_id":    projectID,
		"deployment_id": dep.ID,
		"status":        db.WPStatusStopped,
	})
}

// handleDeleteWordPressDeployment deletes the deployment record and releases resources.
// DELETE /api/projects/{id}/wordpress or DELETE /api/v1/projects/{id}/wordpress
func (s *Server) handleDeleteWordPressDeployment(w http.ResponseWriter, r *http.Request) {
	if s.wordpressOrchestrator == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress orchestrator engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.wordpressOrchestrator.GetDeploymentByProjectID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to find deployment: "+err.Error())
		return
	}

	if err := s.wordpressOrchestrator.DeleteDeployment(r.Context(), dep.ID); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to delete wordpress deployment: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "wordpress deployment deleted successfully",
		"project_id":    projectID,
		"deployment_id": dep.ID,
	})
}

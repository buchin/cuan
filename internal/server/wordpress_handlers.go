package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
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

// -----------------------------------------------------------------------------
// AI Webmaster Engine Endpoints (Sprint 13 S13.3)
// -----------------------------------------------------------------------------

// WordPressAssetUploadRequestBody holds options for uploading theme/plugin via JSON.
type WordPressAssetUploadRequestBody struct {
	Kind       string `json:"kind,omitempty"`
	Slug       string `json:"slug,omitempty"`
	Activate   bool   `json:"activate,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
	ZipBase64  string `json:"zip_base64,omitempty"`
}

// WordPressAssetStatusRequestBody holds status payload for toggling an installed asset.
type WordPressAssetStatusRequestBody struct {
	IsActive bool `json:"is_active"`
}

// WordPressScriptInjectRequestBody defines the payload to inject a tracking script.
type WordPressScriptInjectRequestBody struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Position string `json:"position,omitempty"` // head or footer
	Content  string `json:"content"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

// WordPressScriptToggleRequestBody defines payload to toggle active status of a script.
type WordPressScriptToggleRequestBody struct {
	Enabled bool `json:"enabled"`
}

// WordPressConfigTweakRequestBody defines directives to tweak in wp-config.php.
type WordPressConfigTweakRequestBody struct {
	Tweaks        map[string]interface{} `json:"tweaks"`
	RawDirectives []string               `json:"raw_directives,omitempty"`
}

// Helper to resolve deployment by project ID for webmaster operations
func (s *Server) getDeploymentForProject(r *http.Request, projectID string) (*db.WordPressDeployment, error) {
	if s.wordpressOrchestrator == nil {
		return nil, errors.New("wordpress orchestrator engine is not configured")
	}
	return s.wordpressOrchestrator.GetDeploymentByProjectID(r.Context(), projectID)
}

// handleUploadWordPressAsset uploads and extracts a theme or plugin ZIP safely.
// POST /api/projects/{id}/wordpress/assets/upload or POST /api/v1/projects/{id}/wordpress/assets/upload
func (s *Server) handleUploadWordPressAsset(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	var installParams wordpress.InstallAssetParams
	installParams.DeploymentID = dep.ID

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// 1. Handle multipart form file upload (max 50 MB)
		if err := r.ParseMultipartForm(50 << 20); err != nil {
			respondJSONError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
			return
		}

		file, handler, err := r.FormFile("file")
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "missing 'file' field in multipart form upload")
			return
		}
		defer file.Close()

		zipBytes, err := io.ReadAll(file)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "failed to read uploaded file: "+err.Error())
			return
		}

		installParams.ZipBytes = zipBytes
		installParams.Kind = r.FormValue("kind")
		installParams.CustomSlug = r.FormValue("slug")
		actStr := strings.ToLower(strings.TrimSpace(r.FormValue("activate")))
		installParams.Activate = actStr == "true" || actStr == "1"
		installParams.SourceZipPath = handler.Filename
	} else {
		// 2. Handle JSON payload
		var reqBody WordPressAssetUploadRequestBody
		if r.Body != nil && r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
				return
			}
		}

		installParams.Kind = reqBody.Kind
		installParams.CustomSlug = reqBody.Slug
		installParams.Activate = reqBody.Activate
		installParams.SourceZipPath = reqBody.SourcePath

		if reqBody.ZipBase64 != "" {
			decoded, err := base64.StdEncoding.DecodeString(reqBody.ZipBase64)
			if err != nil {
				respondJSONError(w, http.StatusBadRequest, "invalid zip_base64: "+err.Error())
				return
			}
			installParams.ZipBytes = decoded
		}
	}

	res, err := s.wordpressWebmaster.InstallAsset(r.Context(), installParams)
	if err != nil {
		if errors.Is(err, wordpress.ErrInvalidAssetZip) ||
			errors.Is(err, wordpress.ErrInvalidWordPressAsset) ||
			errors.Is(err, wordpress.ErrZipSlipDetected) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to install asset: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

// handleListWordPressAssets returns installed plugins and themes for a deployment.
// GET /api/projects/{id}/wordpress/assets or GET /api/v1/projects/{id}/wordpress/assets
func (s *Server) handleListWordPressAssets(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	kind := r.URL.Query().Get("kind")
	assets, err := s.wordpressWebmaster.ListAssets(r.Context(), dep.ID, kind)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list assets: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, assets)
}

// handleToggleWordPressAssetStatus toggles active state of an installed plugin or theme.
// POST /api/projects/{id}/wordpress/assets/{asset_id}/status or POST /api/v1/projects/{id}/wordpress/assets/{asset_id}/status
func (s *Server) handleToggleWordPressAssetStatus(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	assetID := strings.TrimSpace(chi.URLParam(r, "asset_id"))
	if assetID == "" {
		respondJSONError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	var reqBody WordPressAssetStatusRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	if err := s.wordpressWebmaster.ToggleAssetStatus(r.Context(), assetID, reqBody.IsActive); err != nil {
		if errors.Is(err, db.ErrWordPressPluginThemeNotFound) {
			respondJSONError(w, http.StatusNotFound, "asset not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to update asset status: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"asset_id":  assetID,
		"is_active": reqBody.IsActive,
		"message":   "asset status updated successfully",
	})
}

// handleDeleteWordPressAsset removes an installed plugin or theme.
// DELETE /api/projects/{id}/wordpress/assets/{asset_id} or DELETE /api/v1/projects/{id}/wordpress/assets/{asset_id}
func (s *Server) handleDeleteWordPressAsset(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	assetID := strings.TrimSpace(chi.URLParam(r, "asset_id"))
	if projectID == "" || assetID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id and asset id are required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	if err := s.wordpressWebmaster.DeleteAsset(r.Context(), dep.ID, assetID); err != nil {
		if errors.Is(err, db.ErrWordPressPluginThemeNotFound) {
			respondJSONError(w, http.StatusNotFound, "asset not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete asset: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"asset_id": assetID,
		"message":  "asset deleted successfully",
	})
}

// handleInjectWordPressScript injects a tracking script into wp_head or wp_footer via mu-plugins.
// POST /api/projects/{id}/wordpress/scripts or POST /api/v1/projects/{id}/wordpress/scripts
func (s *Server) handleInjectWordPressScript(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	var reqBody WordPressScriptInjectRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	params := wordpress.InjectScriptParams{
		DeploymentID: dep.ID,
		ID:           reqBody.ID,
		Name:         reqBody.Name,
		Type:         reqBody.Type,
		Position:     reqBody.Position,
		Content:      reqBody.Content,
		Enabled:      reqBody.Enabled,
	}

	res, err := s.wordpressWebmaster.InjectScript(r.Context(), params)
	if err != nil {
		if errors.Is(err, wordpress.ErrEmptyScriptContent) || errors.Is(err, wordpress.ErrInvalidScriptPosition) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to inject script: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

// handleListWordPressScripts retrieves all injected tracking scripts for a project.
// GET /api/projects/{id}/wordpress/scripts or GET /api/v1/projects/{id}/wordpress/scripts
func (s *Server) handleListWordPressScripts(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	scripts, err := s.wordpressWebmaster.ListScripts(r.Context(), dep.ID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list scripts: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, scripts)
}

// handleToggleWordPressScript toggles active status of an injected script.
// POST /api/projects/{id}/wordpress/scripts/{script_id}/toggle or POST /api/v1/projects/{id}/wordpress/scripts/{script_id}/toggle
func (s *Server) handleToggleWordPressScript(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	scriptID := strings.TrimSpace(chi.URLParam(r, "script_id"))
	if projectID == "" || scriptID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id and script id are required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	var reqBody WordPressScriptToggleRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	updated, err := s.wordpressWebmaster.ToggleScript(r.Context(), dep.ID, scriptID, reqBody.Enabled)
	if err != nil {
		if errors.Is(err, wordpress.ErrScriptNotFound) {
			respondJSONError(w, http.StatusNotFound, "script not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to toggle script: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// handleDeleteWordPressScript removes an injected tracking script.
// DELETE /api/projects/{id}/wordpress/scripts/{script_id} or DELETE /api/v1/projects/{id}/wordpress/scripts/{script_id}
func (s *Server) handleDeleteWordPressScript(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	scriptID := strings.TrimSpace(chi.URLParam(r, "script_id"))
	if projectID == "" || scriptID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id and script id are required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	if err := s.wordpressWebmaster.DeleteScript(r.Context(), dep.ID, scriptID); err != nil {
		if errors.Is(err, wordpress.ErrScriptNotFound) {
			respondJSONError(w, http.StatusNotFound, "script not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete script: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"script_id": scriptID,
		"message":   "script deleted successfully",
	})
}

// handleTweakWordPressConfig modifies wp-config.php constants safely with pre-save PHP syntax verification.
// POST /api/projects/{id}/wordpress/config or POST /api/v1/projects/{id}/wordpress/config
func (s *Server) handleTweakWordPressConfig(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	var reqBody WordPressConfigTweakRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	res, err := s.wordpressWebmaster.TweakConfig(r.Context(), dep.ID, reqBody.Tweaks, reqBody.RawDirectives)
	if err != nil {
		if errors.Is(err, wordpress.ErrInvalidPHPSyntax) || errors.Is(err, wordpress.ErrEmptyConfigTweaks) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, wordpress.ErrWPConfigNotFound) {
			respondJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to tweak wp-config.php: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, res)
}

// handleGetWordPressConfig inspects active wp-config.php constant definitions.
// GET /api/projects/{id}/wordpress/config or GET /api/v1/projects/{id}/wordpress/config
func (s *Server) handleGetWordPressConfig(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	tweaks, err := s.wordpressWebmaster.GetConfigTweaks(r.Context(), dep.ID)
	if err != nil {
		if errors.Is(err, wordpress.ErrWPConfigNotFound) {
			respondJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to read wp-config.php: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, tweaks)
}

// handleListWordPressWebmasterActions returns audit action history for a WordPress deployment.
// GET /api/projects/{id}/wordpress/actions or GET /api/v1/projects/{id}/wordpress/actions
func (s *Server) handleListWordPressWebmasterActions(w http.ResponseWriter, r *http.Request) {
	if s.wordpressWebmaster == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "wordpress webmaster engine is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	dep, err := s.getDeploymentForProject(r, projectID)
	if err != nil {
		if errors.Is(err, db.ErrWordPressDeploymentNotFound) || errors.Is(err, wordpress.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "wordpress deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	actions, err := s.wordpressWebmaster.ListActions(r.Context(), dep.ID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list actions: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, actions)
}


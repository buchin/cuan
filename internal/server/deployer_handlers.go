package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/deployer"
)

// handleListDeployments returns all deployments ordered by creation timestamp descending.
func (s *Server) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	deployments, err := s.deployerDriver.List(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list deployments: "+err.Error())
		return
	}

	if deployments == nil {
		deployments = []deployer.Deployment{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deployments": deployments,
		"count":       len(deployments),
	})
}

// handleGetDeployment retrieves metadata and live health for an individual deployment.
func (s *Server) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	dep, err := s.deployerDriver.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, deployer.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve deployment: "+err.Error())
		return
	}

	health, _ := s.deployerDriver.Health(r.Context(), id)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deployment": dep,
		"health":     health,
	})
}

// handleUploadDeploymentZip accepts a multipart/form-data ZIP upload, validates anti-ZipSlip,
// runs automatic runtime detection, and registers the deployment.
func (s *Server) handleUploadDeploymentZip(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	// 100 MB max file size
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		respondJSONError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "file field is required in multipart form")
		return
	}
	defer file.Close()

	projectName := strings.TrimSpace(r.FormValue("project_name"))
	if projectName == "" {
		filename := header.Filename
		if ext := strings.LastIndex(filename, "."); ext != -1 {
			projectName = filename[:ext]
		} else {
			projectName = filename
		}
	}

	slug := strings.TrimSpace(r.FormValue("slug"))
	runtime := strings.TrimSpace(r.FormValue("runtime"))
	serverID := strings.TrimSpace(r.FormValue("server_id"))

	var envVars map[string]interface{}
	envVarsStr := strings.TrimSpace(r.FormValue("env_vars"))
	if envVarsStr != "" {
		if err := json.Unmarshal([]byte(envVarsStr), &envVars); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid env_vars json: "+err.Error())
			return
		}
	}

	req := deployer.DeploymentRequest{
		ServerID:    serverID,
		ProjectName: projectName,
		Slug:        slug,
		SourceType:  deployer.SourceTypeZip,
		SourceRef:   header.Filename,
		Runtime:     runtime,
		EnvVars:     envVars,
		ZipReader:   file,
		ZipSize:     header.Size,
	}

	res, err := s.deployerDriver.Deploy(r.Context(), req)
	if err != nil {
		if errors.Is(err, deployer.ErrZipSlip) {
			respondJSONError(w, http.StatusBadRequest, "security violation: "+err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to deploy zip: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

// CreateDeploymentPayload models JSON request for Git or custom deployment creation.
type CreateDeploymentPayload struct {
	ServerID    string                 `json:"server_id,omitempty"`
	ProjectName string                 `json:"project_name"`
	Slug        string                 `json:"slug"`
	SourceType  string                 `json:"source_type"` // 'git' or 'zip'
	SourceRef   string                 `json:"source_ref"`
	RepoURL     string                 `json:"repo_url"`
	Branch      string                 `json:"branch"`
	Runtime     string                 `json:"runtime"`
	EnvVars     map[string]interface{} `json:"env_vars"`
}

// handleCreateDeployment handles JSON deployment requests (Git clone or custom metadata).
func (s *Server) handleCreateDeployment(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	var payload CreateDeploymentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	if payload.ProjectName == "" {
		respondJSONError(w, http.StatusBadRequest, "project_name is required")
		return
	}

	sourceType := payload.SourceType
	if sourceType == "" {
		if payload.RepoURL != "" || payload.SourceRef != "" {
			sourceType = deployer.SourceTypeGit
		} else {
			sourceType = deployer.SourceTypeZip
		}
	}

	sourceRef := payload.SourceRef
	if sourceRef == "" {
		sourceRef = payload.RepoURL
	}

	req := deployer.DeploymentRequest{
		ServerID:    payload.ServerID,
		ProjectName: payload.ProjectName,
		Slug:        payload.Slug,
		SourceType:  sourceType,
		SourceRef:   sourceRef,
		Branch:      payload.Branch,
		Runtime:     payload.Runtime,
		EnvVars:     payload.EnvVars,
	}

	res, err := s.deployerDriver.Deploy(r.Context(), req)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to create deployment: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

// handleCreateDeploymentGit is a dedicated endpoint for Git repository intake.
func (s *Server) handleCreateDeploymentGit(w http.ResponseWriter, r *http.Request) {
	s.handleCreateDeployment(w, r)
}

// handleStartDeployment triggers the dynamic port allocation, child process launch,
// and virtual host route registration.
func (s *Server) handleStartDeployment(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	if err := s.deployerDriver.Start(r.Context(), id); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to start deployment: "+err.Error())
		return
	}

	dep, err := s.deployerDriver.Get(r.Context(), id)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "process started but failed to read record: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "running",
		"deployment": dep,
	})
}

// handleStopDeployment gracefully terminates the sandboxed process and unregisters Caddy routes.
func (s *Server) handleStopDeployment(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	if err := s.deployerDriver.Stop(r.Context(), id); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to stop deployment: "+err.Error())
		return
	}

	dep, err := s.deployerDriver.Get(r.Context(), id)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "process stopped but failed to read record: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "stopped",
		"deployment": dep,
	})
}

// handleRestartDeployment executes Stop then Start sequence.
func (s *Server) handleRestartDeployment(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	if err := s.deployerDriver.Restart(r.Context(), id); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to restart deployment: "+err.Error())
		return
	}

	dep, err := s.deployerDriver.Get(r.Context(), id)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "process restarted but failed to read record: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "running",
		"deployment": dep,
	})
}

// handleDeleteDeployment stops any active process, revokes routes, cleans up files,
// and deletes the database record.
func (s *Server) handleDeleteDeployment(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	if err := s.deployerDriver.Delete(r.Context(), id); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to delete deployment: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": true,
		"id":      id,
	})
}

// handleGetDeploymentLogs fetches logs or streams them via Server-Sent Events (SSE).
func (s *Server) handleGetDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			if parsed > 2000 {
				parsed = 2000
			}
			limit = parsed
		}
	}

	isSSE := r.Header.Get("Accept") == "text/event-stream" ||
		r.URL.Query().Get("stream") == "true" ||
		r.URL.Query().Get("follow") == "true"

	if !isSSE {
		logs, err := s.deployerDriver.GetLogs(r.Context(), id, limit)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to get logs: "+err.Error())
			return
		}
		if logs == nil {
			logs = []deployer.LogEntry{}
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"logs":  logs,
			"count": len(logs),
		})
		return
	}

	// Server-Sent Events (SSE) streaming mode
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondJSONError(w, http.StatusBadRequest, "streaming unsupported by client")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send initial log entries
	initialLogs, _ := s.deployerDriver.GetLogs(r.Context(), id, limit)
	var lastLogID int64
	for _, l := range initialLogs {
		if l.ID > lastLogID {
			lastLogID = l.ID
		}
		data, _ := json.Marshal(l)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	// If driver exposes active in-memory LogStreamer, subscribe for instant event push
	var subCh <-chan deployer.LogEntry
	if streamerProvider, ok := s.deployerDriver.(interface{ GetStreamer(string) *deployer.LogStreamer }); ok {
		if st := streamerProvider.GetStreamer(id); st != nil {
			var unsub func()
			subCh, unsub = st.Subscribe()
			defer unsub()
		}
	}

	pollInterval := 2 * time.Second
	if subCh != nil {
		pollInterval = 5 * time.Second
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case entry, ok := <-subCh:
			if !ok {
				subCh = nil
				ticker.Reset(2 * time.Second)
				continue
			}
			if entry.ID > lastLogID || lastLogID == 0 {
				lastLogID = entry.ID
				data, _ := json.Marshal(entry)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		case <-ticker.C:
			currentLogs, err := s.deployerDriver.GetLogs(r.Context(), id, 50)
			if err != nil {
				continue
			}
			for _, l := range currentLogs {
				if l.ID > lastLogID {
					lastLogID = l.ID
					data, _ := json.Marshal(l)
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				}
			}
		}
	}
}

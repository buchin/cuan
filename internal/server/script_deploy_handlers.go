package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

var scriptDeployWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for dev/reverse proxy
	},
}

// ScriptDeployRequestBody specifies parameters for deploying a script.
type ScriptDeployRequestBody struct {
	ServerID        *string           `json:"server_id,omitempty"`
	Domain          string            `json:"domain,omitempty"`
	Port            int               `json:"port,omitempty"`
	Stack           string            `json:"stack,omitempty"`
	EnvVars         map[string]string `json:"env_vars,omitempty"`
	RunInstall      *bool             `json:"run_install,omitempty"`
	RunBuild        *bool             `json:"run_build,omitempty"`
	RunMigrate      *bool             `json:"run_migrate,omitempty"`
	PostInstallCmds []string          `json:"post_install_cmds,omitempty"`
	TimeoutSeconds  int               `json:"timeout_seconds,omitempty"`
	HealthPath      string            `json:"health_path,omitempty"`
	SkipHealthWait  *bool             `json:"skip_health_wait,omitempty"`
}

// handleScriptDeploy builds and launches an isolated script application.
// POST /api/projects/{id}/script-deploy or POST /api/v1/projects/{id}/script-deploy
func (s *Server) handleScriptDeploy(w http.ResponseWriter, r *http.Request) {
	if s.processSupervisor == nil || s.scriptRunner == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "script build runner or process supervisor is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	// 1. Resolve project directory
	projectDir := filepath.Join("data", "projects", projectID)
	if s.projectSvc != nil {
		if proj, err := s.projectSvc.Get(r.Context(), projectID); err == nil && proj != nil && proj.LocalPath != "" {
			projectDir = proj.LocalPath
		}
	}

	// 2. Parse request body
	var reqBody ScriptDeployRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	runInstall := true
	if reqBody.RunInstall != nil {
		runInstall = *reqBody.RunInstall
	}
	runBuild := true
	if reqBody.RunBuild != nil {
		runBuild = *reqBody.RunBuild
	}
	runMigrate := true
	if reqBody.RunMigrate != nil {
		runMigrate = *reqBody.RunMigrate
	}

	timeout := 10 * time.Minute
	if reqBody.TimeoutSeconds > 0 {
		timeout = time.Duration(reqBody.TimeoutSeconds) * time.Second
	}

	// 3. Find or initialize ScriptDeployment record
	var dep *db.ScriptDeployment
	if s.scriptDeployRepo != nil {
		existing, err := s.scriptDeployRepo.GetDeploymentByProjectID(r.Context(), projectID)
		if err == nil && existing != nil {
			dep = existing
		}
	}

	vaultSecret := s.cfg.AuthSecret
	if vaultSecret == "" {
		vaultSecret = "cuan-default-database-provisioner-vault-key"
	}

	if dep == nil {
		randBytes := make([]byte, 8)
		_, _ = rand.Read(randBytes)
		depID := fmt.Sprintf("sd_%s_%s", projectID, hex.EncodeToString(randBytes))

		dep = &db.ScriptDeployment{
			ID:          depID,
			ProjectID:   projectID,
			ServerID:    reqBody.ServerID,
			Port:        reqBody.Port,
			Domain:      reqBody.Domain,
			BuildStatus: db.BuildStatusBuilding,
			Health:      db.HealthStatusUnknown,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		if s.scriptDeployRepo != nil {
			_ = s.scriptDeployRepo.CreateDeployment(r.Context(), dep)
		}
	} else {
		dep.BuildStatus = db.BuildStatusBuilding
		dep.UpdatedAt = time.Now().UTC()
		if reqBody.ServerID != nil {
			dep.ServerID = reqBody.ServerID
		}
		if reqBody.Domain != "" {
			dep.Domain = reqBody.Domain
		}
		if reqBody.Port > 0 {
			dep.Port = reqBody.Port
		}
		if s.scriptDeployRepo != nil {
			_ = s.scriptDeployRepo.UpdateDeployment(r.Context(), dep)
		}
	}

	// 4. Merge environment variables from vault and provisioned DB
	mergedEnv := make(map[string]string)
	if dep.EnvEncrypted != "" {
		if decrypted, err := dep.DecryptEnvMap(vaultSecret); err == nil {
			for k, v := range decrypted {
				mergedEnv[k] = v
			}
		}
	}

	// Check provisioned database if present
	if s.provisionedDBRepo != nil {
		if pdb, err := s.provisionedDBRepo.GetDatabaseByProjectID(r.Context(), projectID); err == nil && pdb != nil {
			plainPassword := ""
			if pdb.DBPasswordEncrypted != "" {
				plainPassword, _ = pdb.DecryptPassword(vaultSecret)
			}
			switch pdb.Engine {
			case db.DBEngineSQLite:
				if _, exists := mergedEnv["DB_CONNECTION"]; !exists {
					mergedEnv["DB_CONNECTION"] = "sqlite"
					mergedEnv["DB_DATABASE"] = pdb.DBName
				}
			case db.DBEngineMySQL:
				if _, exists := mergedEnv["DB_CONNECTION"]; !exists {
					mergedEnv["DB_CONNECTION"] = "mysql"
					mergedEnv["DB_HOST"] = "127.0.0.1"
					mergedEnv["DB_PORT"] = "3306"
					mergedEnv["DB_DATABASE"] = pdb.DBName
					mergedEnv["DB_USERNAME"] = pdb.DBUser
					mergedEnv["DB_PASSWORD"] = plainPassword
				}
			case db.DBEnginePostgres:
				if _, exists := mergedEnv["DB_CONNECTION"]; !exists {
					mergedEnv["DB_CONNECTION"] = "pgsql"
					mergedEnv["DB_HOST"] = "127.0.0.1"
					mergedEnv["DB_PORT"] = "5432"
					mergedEnv["DB_DATABASE"] = pdb.DBName
					mergedEnv["DB_USERNAME"] = pdb.DBUser
					mergedEnv["DB_PASSWORD"] = plainPassword
				}
			}
		}
	}

	// Overlay explicit request env vars
	for k, v := range reqBody.EnvVars {
		mergedEnv[k] = v
	}

	// Save back encrypted env
	_ = dep.EncryptEnvMap(mergedEnv, vaultSecret)
	if s.scriptDeployRepo != nil {
		_ = s.scriptDeployRepo.UpdateDeployment(r.Context(), dep)
	}

	// 5. Setup live log streamer
	streamer := s.processSupervisor.GetOrCreateStreamer(projectID)

	// 6. Execute Dependency Build Runner Pipeline
	buildReq := deployer.ScriptBuildRequest{
		ProjectID:       projectID,
		ProjectDir:      projectDir,
		Stack:           reqBody.Stack,
		EnvVars:         mergedEnv,
		InstallDeps:     runInstall,
		RunBuild:        runBuild,
		RunMigrate:      runMigrate,
		PostInstallCmds: reqBody.PostInstallCmds,
		Timeout:         timeout,
		Streamer:        streamer,
	}

	buildResult, err := s.scriptRunner.RunPipeline(r.Context(), buildReq)
	if err != nil {
		dep.BuildStatus = db.BuildStatusFailed
		dep.Health = db.HealthStatusUnhealthy
		dep.UpdatedAt = time.Now().UTC()
		if s.scriptDeployRepo != nil {
			_ = s.scriptDeployRepo.UpdateDeployment(r.Context(), dep)
		}

		respondJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
			"error":        "build pipeline failed",
			"details":      err.Error(),
			"build_result": buildResult,
			"deployment":   dep.ToDTO(),
		})
		return
	}

	skipHealthWait := false
	if reqBody.SkipHealthWait != nil {
		skipHealthWait = *reqBody.SkipHealthWait
	}

	// 7. Start Supervised Process & Route via Caddy & Verify Health
	supReq := deployer.SupervisorStartRequest{
		ProjectID:      projectID,
		DeploymentID:   dep.ID,
		ProjectDir:     projectDir,
		ServerID:       dep.ServerID,
		Port:           dep.Port,
		Domain:         dep.Domain,
		Stack:          buildResult.Stack,
		EnvVars:        mergedEnv,
		HealthPath:     reqBody.HealthPath,
		SkipHealthWait: skipHealthWait,
	}

	proc, err := s.processSupervisor.StartProcess(r.Context(), supReq)
	if err != nil {
		dep.BuildStatus = db.BuildStatusFailed
		dep.Health = db.HealthStatusUnhealthy
		dep.UpdatedAt = time.Now().UTC()
		if s.scriptDeployRepo != nil {
			_ = s.scriptDeployRepo.UpdateDeployment(r.Context(), dep)
		}

		respondJSONError(w, http.StatusInternalServerError, "failed to start supervised process: "+err.Error())
		return
	}

	// Synchronize final deployment state
	if s.scriptDeployRepo != nil {
		if updated, err := s.scriptDeployRepo.GetDeploymentByProjectID(r.Context(), projectID); err == nil && updated != nil {
			dep = updated
		}
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"deployment":   dep.ToDTO(),
		"build_result": buildResult,
		"process": map[string]interface{}{
			"pid":    proc.PID,
			"port":   proc.Port,
			"domain": proc.Domain,
			"status": proc.Status,
			"health": proc.Health,
		},
		"message": "Script deployed and running successfully",
	})
}

// handleScriptDeployLogs streams or returns logs via SSE, WebSocket, or JSON.
// GET /api/projects/{id}/script-deploy/logs or GET /api/v1/projects/{id}/script-deploy/logs
func (s *Server) handleScriptDeployLogs(w http.ResponseWriter, r *http.Request) {
	if s.processSupervisor == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "process supervisor is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	streamer := s.processSupervisor.GetOrCreateStreamer(projectID)

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			if parsed > 2000 {
				parsed = 2000
			}
			limit = parsed
		}
	}

	// Check if WebSocket upgrade requested
	if websocket.IsWebSocketUpgrade(r) || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		conn, err := scriptDeployWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send initial buffered logs
		initialLogs := streamer.GetRecent(limit)
		initMsg, _ := json.Marshal(map[string]interface{}{
			"type": "init",
			"logs": initialLogs,
		})
		_ = conn.WriteMessage(websocket.TextMessage, initMsg)

		// Subscribe to live log events
		subCh, unsub := streamer.Subscribe()
		defer unsub()

		doneCh := make(chan struct{})
		go func() {
			defer close(doneCh)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-doneCh:
				return
			case entry, ok := <-subCh:
				if !ok {
					return
				}
				data, err := json.Marshal(map[string]interface{}{
					"type":  "log",
					"entry": entry,
				})
				if err != nil {
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					return
				}
			}
		}
	}

	// Check if SSE streaming requested
	isSSE := r.Header.Get("Accept") == "text/event-stream" ||
		r.URL.Query().Get("stream") == "true" ||
		r.URL.Query().Get("follow") == "true"

	if !isSSE {
		// Plain JSON snapshot
		logs := streamer.GetRecent(limit)
		if logs == nil {
			logs = []deployer.LogEntry{}
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"logs":  logs,
			"count": len(logs),
		})
		return
	}

	// SSE Mode
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondJSONError(w, http.StatusBadRequest, "streaming unsupported by client")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	initialLogs := streamer.GetRecent(limit)
	for _, l := range initialLogs {
		data, _ := json.Marshal(l)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	subCh, unsub := streamer.Subscribe()
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case entry, ok := <-subCh:
			if !ok {
				return
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// handleScriptDeployRestart restarts the project's background supervisor process.
// POST /api/projects/{id}/script-deploy/restart or POST /api/v1/projects/{id}/script-deploy/restart
func (s *Server) handleScriptDeployRestart(w http.ResponseWriter, r *http.Request) {
	if s.processSupervisor == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "process supervisor is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	proc, err := s.processSupervisor.RestartProcess(r.Context(), projectID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to restart service: "+err.Error())
		return
	}

	var depDTO *db.ScriptDeploymentDTO
	if s.scriptDeployRepo != nil {
		if dep, err := s.scriptDeployRepo.GetDeploymentByProjectID(r.Context(), projectID); err == nil && dep != nil {
			dto := dep.ToDTO()
			depDTO = &dto
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deployment": depDTO,
		"process": map[string]interface{}{
			"pid":    proc.PID,
			"port":   proc.Port,
			"domain": proc.Domain,
			"status": proc.Status,
			"health": proc.Health,
		},
		"message": "Service restarted successfully",
	})
}

// handleScriptDeployStatus returns the live process status and deployment info.
// GET /api/projects/{id}/script-deploy/status or GET /api/v1/projects/{id}/script-deploy/status
func (s *Server) handleScriptDeployStatus(w http.ResponseWriter, r *http.Request) {
	if s.processSupervisor == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "process supervisor is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var depDTO *db.ScriptDeploymentDTO
	if s.scriptDeployRepo != nil {
		dep, err := s.scriptDeployRepo.GetDeploymentByProjectID(r.Context(), projectID)
		if err != nil && !errors.Is(err, db.ErrScriptDeploymentNotFound) {
			respondJSONError(w, http.StatusInternalServerError, "failed to query deployment: "+err.Error())
			return
		}
		if dep != nil {
			dto := dep.ToDTO()
			depDTO = &dto
		}
	}

	procStatus, err := s.processSupervisor.GetStatus(r.Context(), projectID)
	if err != nil && depDTO == nil {
		respondJSONError(w, http.StatusNotFound, "script deployment not found for project")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deployment": depDTO,
		"process":    procStatus,
	})
}

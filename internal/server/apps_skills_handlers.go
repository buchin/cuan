package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/crypto"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/mcp"
)

// MCPRegistry returns the server's MCP registry, initializing with default if nil.
func (s *Server) MCPRegistry() *mcp.Registry {
	if s.mcpRegistry == nil {
		s.mcpRegistry = mcp.GetDefaultRegistry()
	}
	return s.mcpRegistry
}

// ============================================================================
// Apps Endpoints
// ============================================================================

// handleListApps returns catalog of integrated external third-party apps.
// Route: GET /api/v1/apps
// Alias: GET /api/apps
func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	apps := reg.ListApps()

	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category != "" {
		filtered := make([]*mcp.App, 0, len(apps))
		for _, a := range apps {
			if strings.EqualFold(a.Category, category) {
				filtered = append(filtered, a)
			}
		}
		apps = filtered
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"apps":   apps,
		"total":  len(apps),
	})
}

// handleConnectApp connects an external app via 1-click user action.
// Route: POST /api/v1/apps/{id}/connect
// Alias: POST /api/apps/{id}/connect
func (s *Server) handleConnectApp(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "app id is required")
		return
	}

	var req struct {
		Account string `json:"account"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	app, err := reg.ConnectApp(id, req.Account)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"app":    app,
	})
}

// handleDisconnectApp disconnects an external app.
// Route: POST /api/v1/apps/{id}/disconnect
// Alias: POST /api/apps/{id}/disconnect
func (s *Server) handleDisconnectApp(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "app id is required")
		return
	}

	app, err := reg.DisconnectApp(id)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"app":    app,
	})
}

// handleConnectRemoteApp registers and connects an external MCP server (e.g. tebar.app).
// Route: POST /api/v1/apps/connect-remote
// Alias: POST /api/apps/connect-remote
func (s *Server) handleConnectRemoteApp(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		BaseURL     string `json:"base_url"`
		Transport   string `json:"transport"`
		APIKey      string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.BaseURL) == "" {
		respondJSONError(w, http.StatusBadRequest, "base_url is required")
		return
	}

	cfg := mcp.ServerConfig{
		ID:          strings.TrimSpace(req.ID),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		BaseURL:     strings.TrimSpace(req.BaseURL),
		Transport:   mcp.TransportType(strings.TrimSpace(req.Transport)),
		APIKey:      strings.TrimSpace(req.APIKey),
		Enabled:     true,
	}

	app, tools, err := reg.RegisterRemoteMCP(r.Context(), cfg)
	if err != nil {
		if errors.Is(err, mcp.ErrAuthRequired) {
			msg := "Server MCP ini memerlukan autentikasi. Silakan masukkan API key atau Bearer token."
			if cfg.APIKey != "" {
				msg = "Autentikasi ditolak oleh server MCP. Periksa kembali API key atau Bearer token Anda."
			}
			respondJSON(w, http.StatusBadRequest, map[string]any{
				"error":         msg,
				"auth_required": true,
			})
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.persistRemoteMCP(r.Context(), cfg, app); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to persist remote MCP connector: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"app":    app,
		"tools":  tools,
	})
}

// handleUnregisterRemoteApp disconnects and removes a custom remote MCP server.
// Route: DELETE /api/v1/apps/{id}/custom
// Alias: DELETE /api/apps/{id}/custom
func (s *Server) handleUnregisterRemoteApp(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "app id is required")
		return
	}

	if err := reg.UnregisterRemoteMCP(r.Context(), id); err != nil {
		respondJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := s.deletePersistedRemoteMCP(r.Context(), id); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to delete persisted remote MCP connector: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"app_id": id,
	})
}

func (s *Server) remoteMCPSecret() string {
	if s.cfg != nil && strings.TrimSpace(s.cfg.AuthSecret) != "" {
		return s.cfg.AuthSecret
	}
	return "cuan-default-remote-mcp-secret-32b"
}

func (s *Server) persistRemoteMCP(ctx context.Context, cfg mcp.ServerConfig, app *mcp.App) error {
	if s.remoteMCPRepo == nil || app == nil {
		return nil
	}
	var encKey string
	if strings.TrimSpace(cfg.APIKey) != "" {
		var err error
		encKey, err = crypto.EncryptStringAES256GCM(strings.TrimSpace(cfg.APIKey), s.remoteMCPSecret())
		if err != nil {
			return err
		}
	}
	return s.remoteMCPRepo.UpsertRemoteMCPConnector(ctx, &cuanDB.RemoteMCPConnector{
		ID:              app.ID,
		Name:            app.Name,
		Description:     app.Description,
		BaseURL:         app.BaseURL,
		Transport:       app.Transport,
		APIKeyEncrypted: encKey,
		Enabled:         true,
	})
}

func (s *Server) deletePersistedRemoteMCP(ctx context.Context, appID string) error {
	if s.remoteMCPRepo == nil {
		return nil
	}
	return s.remoteMCPRepo.DeleteRemoteMCPConnector(ctx, appID)
}

// RestoreRemoteMCPConnectors reloads saved Remote MCP connectors from the database into the registry.
// Unreachable endpoints are recorded with status "error" without blocking startup.
func (s *Server) RestoreRemoteMCPConnectors(ctx context.Context) {
	if s.remoteMCPRepo == nil {
		return
	}
	connectors, err := s.remoteMCPRepo.ListRemoteMCPConnectors(ctx)
	if err != nil {
		log.Printf("[Startup] ListRemoteMCPConnectors error: %v", err)
		return
	}
	if len(connectors) == 0 {
		return
	}

	reg := s.MCPRegistry()
	secret := s.remoteMCPSecret()

	var wg sync.WaitGroup
	for _, c := range connectors {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			var apiKey string
			if c.APIKeyEncrypted != "" {
				decrypted, decErr := crypto.DecryptStringAES256GCM(c.APIKeyEncrypted, secret)
				if decErr != nil {
					log.Printf("[Startup] Failed to decrypt API key for remote MCP %s: %v", c.ID, decErr)
				} else {
					apiKey = decrypted
				}
			}

			connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			cfg := mcp.ServerConfig{
				ID:          c.ID,
				Name:        c.Name,
				Description: c.Description,
				BaseURL:     c.BaseURL,
				Transport:   mcp.TransportType(c.Transport),
				APIKey:      apiKey,
				Enabled:     c.Enabled,
				Timeout:     5 * time.Second,
			}
			if _, _, restoreErr := reg.RestoreRemoteMCP(connCtx, cfg); restoreErr != nil {
				log.Printf("[Startup] Remote MCP %s (%s) unavailable at boot: %v", c.ID, c.BaseURL, restoreErr)
			}
		}()
	}
	wg.Wait()
}

// ============================================================================
// Skills Endpoints
// ============================================================================

// handleListSkills returns catalog of modular intelligence capabilities.
// Route: GET /api/v1/skills
// Alias: GET /api/skills
func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	skills := reg.ListSkills()

	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category != "" {
		filtered := make([]*mcp.Skill, 0, len(skills))
		for _, sk := range skills {
			if strings.EqualFold(sk.Category, category) {
				filtered = append(filtered, sk)
			}
		}
		skills = filtered
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"skills": skills,
		"total":  len(skills),
	})
}

// handleGetAgentSkills returns the skill enablement configuration for a specific agent.
// Route: GET /api/v1/agents/{id}/skills
// Alias: GET /api/agents/{id}/skills
func (s *Server) handleGetAgentSkills(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	agentID := strings.TrimSpace(chi.URLParam(r, "id"))
	if agentID == "" {
		respondJSONError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	views, err := reg.GetAgentSkills(agentID)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"agent_id": agentID,
		"skills":   views,
		"total":    len(views),
	})
}

// handleUpdateAgentSkills updates the skill enablement configuration for a specific agent.
// Route: PUT /api/v1/agents/{id}/skills
// Alias: PUT /api/agents/{id}/skills
func (s *Server) handleUpdateAgentSkills(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	agentID := strings.TrimSpace(chi.URLParam(r, "id"))
	if agentID == "" {
		respondJSONError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	var inputs []mcp.AgentSkillInput

	// Accept either {"skills": [...]} or raw array [...]
	var payload struct {
		Skills []mcp.AgentSkillInput `json:"skills"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&payload); err == nil && len(payload.Skills) > 0 {
		inputs = payload.Skills
	}

	views, err := reg.UpdateAgentSkills(agentID, inputs)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"agent_id": agentID,
		"skills":   views,
		"total":    len(views),
	})
}

// handleInstallSkill installs a modular skill from URL or raw markdown content.
// Route: POST /api/v1/skills/install
// Alias: POST /api/skills/install
func (s *Server) handleInstallSkill(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	var req struct {
		URL         string   `json:"url"`
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Category    string   `json:"category"`
		Content     string   `json:"content"`
		BoundAgents []string `json:"bound_agents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var skill *mcp.Skill
	var err error

	if strings.TrimSpace(req.URL) != "" {
		skill, err = reg.InstallSkill(r.Context(), req.URL, req.BoundAgents)
	} else if strings.TrimSpace(req.Content) != "" || strings.TrimSpace(req.Name) != "" {
		skill, err = reg.InstallSkillFromContent(r.Context(), req.ID, req.Name, req.Description, req.Category, req.Content, req.BoundAgents)
	} else {
		respondJSONError(w, http.StatusBadRequest, "provide a skill URL or markdown content")
		return
	}

	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"status": "success",
		"skill":  skill,
	})
}

// handleUninstallSkill uninstalls a custom modular skill.
// Route: DELETE /api/v1/skills/{id}
// Alias: DELETE /api/skills/{id}
func (s *Server) handleUninstallSkill(w http.ResponseWriter, r *http.Request) {
	reg := s.MCPRegistry()
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "skill id is required")
		return
	}

	if err := reg.UninstallSkill(r.Context(), id); err != nil {
		respondJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"skill_id": id,
	})
}

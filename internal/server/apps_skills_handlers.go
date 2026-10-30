package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

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

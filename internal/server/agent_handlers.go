package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

// ============================================================================
// Agent Personas Handlers
// ============================================================================

// handleListAgents lists agents with optional filters (?provider=, ?status=, ?role=, ?search=, ?limit=, ?offset=).
// Route: GET /api/agents
// Aliases: GET /api/v1/agents
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	q := r.URL.Query()
	filter := db.AgentFilter{
		Provider: strings.TrimSpace(q.Get("provider")),
		Status:   strings.TrimSpace(q.Get("status")),
		Role:     strings.TrimSpace(q.Get("role")),
		Search:   strings.TrimSpace(q.Get("search")),
	}

	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if offsetStr := strings.TrimSpace(q.Get("offset")); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	agents, err := s.agentRepo.ListAgents(r.Context(), filter)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Agent list is unavailable. Try again later")
		return
	}

	total, err := s.agentRepo.CountAgents(r.Context(), filter)
	if err != nil {
		total = len(agents)
	}

	type rosterAgent struct {
		db.AgentDTO
		ConnectionState string     `json:"connection_state"`
		LastCheckedAt   *time.Time `json:"last_checked_at,omitempty"`
		VerifiedAt      *time.Time `json:"verified_at,omitempty"`
	}
	dtos := make([]rosterAgent, len(agents))
	ownerID := ""
	if s.authSvc != nil {
		if owner, err := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r)); err == nil {
			ownerID = owner.ID
		}
	}
	for i, a := range agents {
		status := agent.LegacyConnectionVerification(ownerID, a.ID, a.ConnectionID)
		state := status.State
		checkedAt, verifiedAt := status.LastCheckedAt, status.VerifiedAt
		dto := a.ToDTO()
		if a.Harness == "commandcode" || a.Provider == "commandcode" {
			var conn *db.AgentConnection
			if a.ConnectionID != "" {
				conn, _ = s.agentRepo.GetConnectionByID(r.Context(), a.ConnectionID)
			}
			if (conn == nil || conn.CredentialsEncrypted == "") && ownerID != "" {
				if userConn, err := s.agentRepo.GetConnectionByUserAndProvider(r.Context(), ownerID, "commandcode"); err == nil && userConn != nil && userConn.CredentialsEncrypted != "" {
					conn = userConn
				}
			}
			if conn != nil && conn.CredentialsEncrypted != "" {
				dto.AuthType = "byok"
			} else {
				dto.AuthType = "subscription"
			}
			if conn != nil && conn.Status == db.ConnectionStatusActive {
				state = "verified"
				checkedAt = &conn.UpdatedAt
				verifiedAt = &conn.UpdatedAt
			}
		} else if strings.HasPrefix(a.ConnectionID, "conn_profile_") {
			state = "login_required"
			if conn, err := s.agentRepo.GetConnectionByID(r.Context(), a.ConnectionID); err == nil && conn.UserID == ownerID {
				_, bindingErr := s.agentProfileHome(r.Context(), &a)
				if profile, err := agent.ProviderAccountForConnection(ownerID, a.ID, conn.ID); err == nil && bindingErr == nil {
					state = profile.State
					checkedAt, verifiedAt = profile.LastCheckedAt, profile.VerifiedAt
					if state == "quota_limited" {
						if profiles, err := agent.ListProviderAccounts(ownerID); err == nil {
							for _, p := range profiles {
								if p.ID != conn.ID && p.AllowFailover && p.CredentialsPresent && p.State != "quota_limited" && p.State != "login_required" && p.State != "unavailable" {
									state = "verified"
									break
								}
							}
						}
					}
				}
			}
		}
		dtos[i] = rosterAgent{AgentDTO: dto, ConnectionState: state, LastCheckedAt: checkedAt, VerifiedAt: verifiedAt}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"agents": dtos,
		"count":  len(dtos),
		"total":  total,
	})
}

// handleCreateAgent registers a new agent persona.
// Route: POST /api/agents
// Aliases: POST /api/v1/agents
func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	var input db.CreateAgentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	if err := input.Validate(); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if input.Harness == "agy" || input.Provider == "agy" || input.Provider == "antigravity" {
		if !s.requireOAuthOwner(w, r) {
			return
		}
		respondJSONError(w, http.StatusConflict, "Use Add agent to connect this agent account")
		return
	}
	entity, err := input.ToEntity()
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.agentRepo.CreateAgent(r.Context(), entity); err != nil {
		if errors.Is(err, db.ErrDuplicateAgent) {
			respondJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, db.ErrEmptyAgentName) || errors.Is(err, db.ErrInvalidAgentStatus) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to create agent: "+err.Error())
		return
	}

	// S23: Securely store credentials / connection configuration
	connID := entity.ConnectionID
	if connID == "" {
		connID = "conn_" + entity.ID
		entity.ConnectionID = connID
		_ = s.agentRepo.UpdateAgent(r.Context(), entity)
	}

	var encryptedCred string

	planHint := "Host server session (no API key)"
	switch entity.Harness {
	case "agy":
		planHint = "Antigravity CLI session"
	case "claude":
		planHint = "Claude CLI session"
	case "codex":
		planHint = "Codex CLI session"
	case "custom":
		planHint = "Custom model session"
	}

	conn := &db.AgentConnection{
		ID:                   connID,
		UserID:               "default",
		Provider:             entity.Harness,
		CredentialsEncrypted: encryptedCred,
		PlanHint:             planHint,
		Status:               db.ConnectionStatusActive,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	_ = s.agentRepo.CreateConnection(r.Context(), conn)

	// Ensure direct chat room exists for this agent
	if s.projectStudioRepo != nil {
		roomExists := false
		if allRooms, err := s.projectStudioRepo.ListAllRooms(r.Context()); err == nil {
			for _, rm := range allRooms {
				if rm.Topic == "agent:"+entity.ID || rm.Title == entity.Name {
					roomExists = true
					break
				}
			}
		}
		if !roomExists {
			roomID := db.GenerateUUID()
			now := time.Now().UTC()
			_ = s.projectStudioRepo.CreateRoom(r.Context(), &db.ProjectRoom{
				ID:        roomID,
				ProjectID: "default",
				Title:     entity.Name,
				Topic:     "agent:" + entity.ID,
				CreatedAt: now,
				UpdatedAt: now,
			})
			_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
				ID:         db.GenerateUUID(),
				RoomID:     roomID,
				AuthorType: "agent",
				AuthorID:   entity.ID,
				AuthorName: entity.Name,
				Content:    fmt.Sprintf("Hello! I am %s. My agent session is connected through the UI (%s) and ready for instructions.", entity.Name, entity.Harness),
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "success",
		"agent":  entity.ToDTO(),
	})
}

// handleGetAgent retrieves a single agent by ID.
// Route: GET /api/agents/{id}
// Aliases: GET /api/v1/agents/{id}
func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	entity, err := s.agentRepo.GetAgentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get agent: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"agent":  entity.ToDTO(),
	})
}

// handleUpdateAgent modifies an existing agent.
// Route: PUT /api/agents/{id}
// Aliases: PUT /api/v1/agents/{id}
func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	entity, err := s.agentRepo.GetAgentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get agent: "+err.Error())
		return
	}

	var input db.UpdateAgentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	if err := input.ApplyTo(entity); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.agentRepo.UpdateAgent(r.Context(), entity); err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		if errors.Is(err, db.ErrEmptyAgentName) || errors.Is(err, db.ErrInvalidAgentStatus) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to update agent: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"agent":  entity.ToDTO(),
	})
}

// handlePatchAgent modifies an existing agent partially (Universal Agent format { id, name, harness, connection_id, enabled }).
// Route: PATCH /api/agents/{id}
// Aliases: PATCH /api/v1/agents/{id}
func (s *Server) handlePatchAgent(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	entity, err := s.agentRepo.GetAgentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get agent: "+err.Error())
		return
	}

	var input db.PatchAgentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	if err := input.ApplyTo(entity); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.agentRepo.UpdateAgent(r.Context(), entity); err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		if errors.Is(err, db.ErrEmptyAgentName) || errors.Is(err, db.ErrInvalidAgentStatus) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to update agent: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"agent":  entity.ToDTO(),
	})
}

// handleDeleteAgent deletes an agent persona by ID.
// Route: DELETE /api/agents/{id}
// Aliases: DELETE /api/v1/agents/{id}
func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	if err := s.agentRepo.DeleteAgent(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete agent: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "agent deleted successfully",
	})
}

// ============================================================================
// Agent Connections Handlers
// ============================================================================

// CreateAgentConnectionRequest defines the JSON payload for creating an AgentConnection.
type CreateAgentConnectionRequest struct {
	ID                   string `json:"id,omitempty"`
	UserID               string `json:"user_id,omitempty"`
	Provider             string `json:"provider,omitempty"`
	APIKey               string `json:"api_key,omitempty"`
	Credentials          string `json:"credentials,omitempty"`
	CredentialsEncrypted string `json:"credentials_encrypted,omitempty"`
	AuthType             string `json:"auth_type,omitempty"`
	SessionToken         string `json:"session_token,omitempty"`
	Handle               string `json:"handle,omitempty"`
	PlanHint             string `json:"plan_hint,omitempty"`
	Status               string `json:"status,omitempty"`
	VerifyNow            bool   `json:"verify_now,omitempty"`
}

// handleListAgentConnections lists agent connections (?user_id=, ?provider=, ?status=, ?limit=, ?offset=).
// Route: GET /api/agent-connections
// Aliases: GET /api/v1/agent-connections, GET /api/agents/connections, GET /api/v1/agents/connections
func (s *Server) handleListAgentConnections(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	q := r.URL.Query()
	filter := db.AgentConnectionFilter{
		UserID:   strings.TrimSpace(q.Get("user_id")),
		Provider: strings.TrimSpace(q.Get("provider")),
		Status:   strings.TrimSpace(q.Get("status")),
	}

	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if offsetStr := strings.TrimSpace(q.Get("offset")); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	connections, err := s.agentRepo.ListConnections(r.Context(), filter)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list connections: "+err.Error())
		return
	}

	total, err := s.agentRepo.CountConnections(r.Context(), filter)
	if err != nil {
		total = len(connections)
	}

	dtos := make([]db.AgentConnectionDTO, len(connections))
	for i, c := range connections {
		dtos[i] = c.ToDTO()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"connections": dtos,
		"count":       len(dtos),
		"total":       total,
	})
}

// handleCreateAgentConnection creates a new agent provider connection with AES-256-GCM encrypted credentials.
// Route: POST /api/agent-connections
// Aliases: POST /api/v1/agent-connections, POST /api/agents/connections, POST /api/v1/agents/connections
func (s *Server) handleCreateAgentConnection(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	var req CreateAgentConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = "default"
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = db.DefaultConnectionProvider
	}

	rawSecret := strings.TrimSpace(req.APIKey)
	if rawSecret == "" {
		rawSecret = strings.TrimSpace(req.Credentials)
	}
	if rawSecret == "" && (req.AuthType == "cli_login" || req.AuthType == "oauth" || provider == "antigravity" || provider == "gemini-cli" || provider == "claude-cli" || req.SessionToken != "") {
		if req.SessionToken != "" {
			rawSecret = req.SessionToken
		} else {
			rawSecret = "cuan_cli_session_" + db.GenerateUUID()
		}
		if req.PlanHint == "" {
			req.PlanHint = "Host server session (no API key)"
		}
	}

	encryptedCredentials := strings.TrimSpace(req.CredentialsEncrypted)
	if encryptedCredentials == "" && rawSecret != "" {
		encrypted, err := agent.EncryptCredential(rawSecret)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to encrypt credentials: "+err.Error())
			return
		}
		encryptedCredentials = encrypted
	}

	if encryptedCredentials == "" {
		respondJSONError(w, http.StatusBadRequest, "api_key or credentials are required")
		return
	}

	status := db.ConnectionStatusActive
	if req.Status != "" {
		norm, err := db.NormalizeConnectionStatus(req.Status)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		status = norm
	}

	planHint := strings.TrimSpace(req.PlanHint)

	// If verify_now requested, probe connection before persistence
	var probeResult *agent.ConnectionProbeResult
	if req.VerifyNow && rawSecret != "" {
		reg := s.ProviderRegistry()
		if reg == nil {
			reg = agent.DefaultRegistry()
		}
		prov, err := reg.Get(provider)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "unsupported provider: "+provider)
			return
		}

		probe, err := prov.VerifyConnection(r.Context(), rawSecret)
		probeResult = probe
		if err != nil || (probe != nil && !probe.Valid) {
			status = db.ConnectionStatusInvalid
			errMsg := "connection verification failed"
			if probe != nil && probe.ErrorMessage != "" {
				errMsg = probe.ErrorMessage
			} else if err != nil {
				errMsg = err.Error()
			}
			respondJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
				"status":   "failed",
				"verified": false,
				"error":    errMsg,
				"probe":    probeResult,
			})
			return
		}

		if probe != nil && probe.PlanHint != "" && planHint == "" {
			planHint = probe.PlanHint
		}
	}

	connID := strings.TrimSpace(req.ID)
	if connID == "" {
		connID = db.GenerateUUID()
	}

	connEntity := &db.AgentConnection{
		ID:                   connID,
		UserID:               userID,
		Provider:             provider,
		CredentialsEncrypted: encryptedCredentials,
		PlanHint:             planHint,
		Status:               status,
	}

	if err := s.agentRepo.CreateConnection(r.Context(), connEntity); err != nil {
		if errors.Is(err, db.ErrDuplicateConnection) {
			respondJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, db.ErrEmptyUserID) || errors.Is(err, db.ErrInvalidConnectionStatus) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to create connection: "+err.Error())
		return
	}

	resp := map[string]interface{}{
		"status":     "success",
		"connection": connEntity.ToDTO(),
	}
	if probeResult != nil {
		resp["probe"] = probeResult
		resp["verified"] = probeResult.Valid
	}

	respondJSON(w, http.StatusCreated, resp)
}

// handleGetAgentConnection retrieves a single agent connection by ID.
// Route: GET /api/agent-connections/{id}
// Aliases: GET /api/v1/agent-connections/{id}, GET /api/agents/connections/{id}, GET /api/v1/agents/connections/{id}
func (s *Server) handleGetAgentConnection(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "connection id is required")
		return
	}

	conn, err := s.agentRepo.GetConnectionByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrAgentConnectionNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent connection not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get connection: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "success",
		"connection": conn.ToDTO(),
	})
}

// handleDeleteAgentConnection deletes an agent connection by ID.
// Route: DELETE /api/agent-connections/{id}
// Aliases: DELETE /api/v1/agent-connections/{id}, DELETE /api/agents/connections/{id}, DELETE /api/v1/agents/connections/{id}
func (s *Server) handleDeleteAgentConnection(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "connection id is required")
		return
	}

	if err := s.agentRepo.DeleteConnection(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrAgentConnectionNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent connection not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete connection: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "agent connection deleted successfully",
	})
}

// handleVerifyAgentConnection tests and verifies an existing agent connection.
// Route: POST /api/agent-connections/{id}/verify
// Aliases: POST /api/v1/agent-connections/{id}/verify, POST /api/agents/connections/{id}/verify, POST /api/v1/agents/connections/{id}/verify
func (s *Server) handleVerifyAgentConnection(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "connection id is required")
		return
	}

	conn, err := s.agentRepo.GetConnectionByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrAgentConnectionNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent connection not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get connection: "+err.Error())
		return
	}

	if conn.CredentialsEncrypted == "" {
		respondJSONError(w, http.StatusBadRequest, "connection does not contain encrypted credentials")
		return
	}

	decryptedKey, err := agent.DecryptCredential(conn.CredentialsEncrypted)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to decrypt connection credentials: "+err.Error())
		return
	}

	reg := s.ProviderRegistry()
	if reg == nil {
		reg = agent.DefaultRegistry()
	}

	prov, err := reg.Get(conn.Provider)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "unsupported provider: "+conn.Provider)
		return
	}

	probe, err := prov.VerifyConnection(r.Context(), decryptedKey)
	if err != nil || (probe != nil && !probe.Valid) {
		conn.Status = db.ConnectionStatusInvalid
		_ = s.agentRepo.UpdateConnection(r.Context(), conn)

		errMsg := "verification failed"
		if probe != nil && probe.ErrorMessage != "" {
			errMsg = probe.ErrorMessage
		} else if err != nil {
			errMsg = err.Error()
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "failed",
			"verified":   false,
			"error":      errMsg,
			"probe":      probe,
			"connection": conn.ToDTO(),
		})
		return
	}

	// Update connection to active and apply plan hint if discovered
	conn.Status = db.ConnectionStatusActive
	if probe != nil && probe.PlanHint != "" {
		conn.PlanHint = probe.PlanHint
	}
	if err := s.agentRepo.UpdateConnection(r.Context(), conn); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to update connection status: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "success",
		"verified":   true,
		"probe":      probe,
		"connection": conn.ToDTO(),
	})
}

// handleConnectCLIAgent registers an agent and its connection without a raw API Key.
// Route: POST /api/v1/agents/connect-cli, POST /api/agents/connect-cli
func (s *Server) handleConnectCLIAgent(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuthOwner(w, r) {
		return
	}
	if s.agentRepo == nil || s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Agent registry or Inbox storage is unavailable")
		return
	}

	var req struct {
		Name         string `json:"name"`
		Handle       string `json:"handle"`
		Provider     string `json:"provider"`
		ModelID      string `json:"model_id"`
		Role         string `json:"role"`
		SystemPrompt string `json:"system_prompt"`
		AuthType     string `json:"auth_type"`
		SessionToken string `json:"session_token"`
		AccountID    string `json:"account_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondJSONError(w, http.StatusBadRequest, "agent name is required")
		return
	}

	handle := strings.TrimSpace(req.Handle)
	if handle == "" {
		handle = "@" + strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	}
	if !strings.HasPrefix(handle, "@") {
		handle = "@" + handle
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "antigravity"
	}

	ownerID := "default"
	var profile *agent.ProviderAccount
	isCommandCode := provider == "commandcode"
	if provider == "agy" || provider == "antigravity" || isCommandCode {
		ownerID = s.accountOwner(w, r)
		if ownerID == "" {
			return
		}
		if isCommandCode && (req.AuthType == "byok" || req.SessionToken != "") {
			// Custom provider BYOK: store the user's API key encrypted in ProviderAccount.
			// No CLI OAuth flow is involved; credentials belong to the user.
		} else if isCommandCode && (req.AuthType == "subscription" || req.SessionToken == "") {
			// Subscription connect: CommandCode CLI handles its own authentication.
			// No provider account / Google OAuth flow is involved.
		} else {
			var err error
			profile, err = agent.GetProviderAccount(ownerID, req.AccountID)
			if err != nil || profile.Legacy || !profile.CredentialsPresent {
				respondJSONError(w, http.StatusConflict, "Complete CommandCode sign-in for this agent first")
				return
			}
		}
	}
	modelID := strings.TrimSpace(req.ModelID)
	if modelID == "" {
		modelID = "default"
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "Development Team (Worker)"
	}

	// OAuth credentials remain in the CLI-owned credential store; do not invent a token.
	var encryptedToken string
	if isCommandCode && (req.AuthType == "byok" || req.SessionToken != "") {
		apiKey := strings.TrimSpace(req.SessionToken)
		if len(apiKey) < 8 {
			respondJSONError(w, http.StatusBadRequest, "CommandCode API key is required for custom provider BYOK (at least 8 characters)")
			return
		}
		var err error
		encryptedToken, err = agent.EncryptCredential(apiKey)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "Failed to protect credential")
			return
		}
	} else if isCommandCode && (req.AuthType == "subscription" || req.SessionToken == "") {
		// Subscription connect: no token to encrypt; CommandCode CLI handles auth.
	} else if req.SessionToken != "" && provider != "agy" && provider != "antigravity" {
		var err error
		encryptedToken, err = agent.EncryptCredential(strings.TrimSpace(req.SessionToken))
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "Failed to protect credential")
			return
		}
	}

	// 1. Create / Persist Agent
	cleanHandle := strings.TrimPrefix(handle, "@")
	cleanHandle = strings.ReplaceAll(cleanHandle, "-", "_")
	agentID := "agent_" + cleanHandle
	// A deleted persona may leave a retained connection/history. Do not reuse its ID.
	connID := "conn_" + db.GenerateUUID()
	if profile != nil {
		connID = "conn_profile_" + db.GenerateUUID()
	}
	// The existing 19-migration schema persists the CLI connection link in model_id for agy.
	// For CommandCode, store the selected model ID (or default) so model selection & vision check work.
	if provider == "agy" || provider == "antigravity" {
		modelID = connID
	} else if isCommandCode {
		if req.ModelID != "" && req.ModelID != "default" {
			modelID = req.ModelID
		} else {
			modelID = "commandcode-default"
		}
	}
	agentDisplayName := handle
	if name != handle {
		agentDisplayName = fmt.Sprintf("%s (%s)", name, handle)
	}
	if existing, err := s.agentRepo.GetAgentByID(r.Context(), agentID); err == nil {
		// A client may retry after the successful response was lost. Reconcile
		// only the same dedicated binding, never another agent or account.
		if profile != nil && profile.AgentID == agentID && profile.ConnectionID == existing.ConnectionID {
			if conn, err := s.agentRepo.GetConnectionByID(r.Context(), existing.ConnectionID); err == nil && conn.UserID == ownerID {
				rooms, err := s.projectStudioRepo.ListAllRooms(r.Context())
				if err == nil {
					for _, room := range rooms {
						if room.Topic == "agent:"+agentID {
							respondJSON(w, http.StatusOK, map[string]any{"status": "success", "agent": existing.ToDTO(), "connection": conn.ToDTO(), "conversation_id": room.ID, "room_id": room.ID, "provider": provider, "handle": handle, "message": "Agent already saved"})
							return
						}
					}
				}
			}
		}
		respondJSONError(w, http.StatusConflict, "Agent name is already in use. Choose another name.")
		return
	}

	committed := false
	if profile != nil {
		if err := agent.BindProviderAccount(ownerID, profile.ID, agentID, connID); err != nil {
			respondJSONError(w, http.StatusConflict, err.Error())
			return
		}
		// Secondary or newly connected accounts default to active backup & model fallback
		// so quota failover works seamlessly without requiring non-developer users
		// to manually find and toggle checkboxes.
		if accounts, err := agent.ListProviderAccounts(ownerID); err == nil && len(accounts) > 1 {
			_, _ = agent.SetProviderAccountFailover(ownerID, profile.ID, true)
			_, _ = agent.SetProviderAccountModelFallback(ownerID, profile.ID, true)
		}
		defer func() {
			if !committed {
				agent.UnbindProviderAccount(ownerID, profile.ID, agentID, connID)
			}
		}()
	}

	agentEntity := &db.Agent{
		ID:           agentID,
		Name:         agentDisplayName,
		Harness:      provider,
		ConnectionID: connID,
		Enabled:      true,
		Avatar:       "🤖",
		Role:         role,
		SystemPrompt: req.SystemPrompt,
		ModelID:      modelID,
		Provider:     provider,
		Status:       db.AgentStatusActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.agentRepo.CreateAgent(r.Context(), agentEntity); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to create agent: "+err.Error())
		return
	}

	// 2. Create / Persist Connection
	planHint := "Host server session (no API key)"
	if isCommandCode {
		if encryptedToken != "" {
			planHint = "CommandCode custom provider (BYOK)"
		} else {
			planHint = "CommandCode subscription"
		}
	}
	connEntity := &db.AgentConnection{
		ID:                   connID,
		UserID:               ownerID,
		Provider:             provider,
		CredentialsEncrypted: encryptedToken,
		PlanHint:             planHint,
		Status:               db.ConnectionStatusActive,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	if err := s.agentRepo.CreateConnection(r.Context(), connEntity); err != nil {
		s.rollbackAgentRegistration(agentID, connID, false)
		respondJSONError(w, http.StatusInternalServerError, "Could not save the agent. Try again.")
		return
	}

	// 3. Ensure a room in projectStudioRepo for direct chat with this agent
	var roomID string
	if s.projectStudioRepo != nil {
		if allRooms, err := s.projectStudioRepo.ListAllRooms(r.Context()); err == nil {
			for _, rm := range allRooms {
				if rm.Topic == "agent:"+agentID || rm.Title == agentDisplayName {
					roomID = rm.ID
					break
				}
			}
		}
		if roomID == "" {
			roomID = db.GenerateUUID()
			now := time.Now().UTC()
			if err := s.projectStudioRepo.CreateRoom(r.Context(), &db.ProjectRoom{
				ID:        roomID,
				ProjectID: "default",
				Title:     agentDisplayName,
				Topic:     "agent:" + agentID,
				CreatedAt: now,
				UpdatedAt: now,
			}); err != nil {
				s.rollbackAgentRegistration(agentID, connID, true)
				respondJSONError(w, http.StatusInternalServerError, "Could not save the agent to Inbox. Try again.")
				return
			}
		}
	} else {
		roomID = db.GenerateUUID()
	}

	committed = true
	dto := agentEntity.ToDTO()
	if isCommandCode {
		if encryptedToken != "" {
			dto.AuthType = "byok"
		} else {
			dto.AuthType = "subscription"
		}
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status":          "success",
		"agent":           dto,
		"connection":      connEntity.ToDTO(),
		"handle":          handle,
		"provider":        provider,
		"room_id":         roomID,
		"conversation_id": roomID,
		"auth_model":      "cuan_cli_session",
		"message":         "Agent connected successfully via CLI login without API Key",
	})
}

// requireOAuthOwner protects account login and provider probes with the owner session.
func (s *Server) requireOAuthOwner(w http.ResponseWriter, r *http.Request) bool {
	if s.authSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Owner authentication unavailable")
		return false
	}
	if _, err := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r)); err != nil {
		respondJSONError(w, http.StatusUnauthorized, "Masuk ke Cuan terlebih dahulu")
		return false
	}
	return true
}

// handleGetHarnessAuthStatus checks whether the server environment is authenticated for a harness.
// Route: GET /api/v1/agents/auth-status, GET /api/agents/auth-status
func (s *Server) handleGetHarnessAuthStatus(w http.ResponseWriter, r *http.Request) {
	harness := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("harness")))
	if harness == "" || harness == "antigravity" {
		harness = "agy"
	}

	if harness == "agy" {
		status := agent.GetAntigravityAuthStatus()
		status["runtime_prerequisites"] = onboardingRuntimePrerequisites()
		if r.URL.Query().Get("verify") == "true" {
			if !s.requireOAuthOwner(w, r) {
				return
			}
			err := agent.VerifyAntigravityConnection(r.Context())
			status["verified"] = err == nil
			status["authenticated"] = err == nil
			if err != nil {
				status["message"] = err.Error()
			} else {
				status["message"] = "Antigravity provider connection verified"
			}
		}
		respondJSON(w, http.StatusOK, status)
		return
	}

	switch harness {
	case "commandcode":
		cliAvailable := func() bool {
			if _, err := exec.LookPath("commandcode"); err == nil {
				return true
			}
			for _, candidate := range []string{
				"/home/dev/.local/bin/commandcode",
				"/home/ubuntu/.local/bin/commandcode",
				"/usr/local/bin/commandcode",
			} {
				if _, err := os.Stat(candidate); err == nil {
					return true
				}
			}
			return false
		}()
		status := map[string]interface{}{
			"status":              "success",
			"harness":             "commandcode",
			"authenticated":       cliAvailable,
			"message":             "CommandCode CLI is ready",
			"credentials_present": cliAvailable,
			"verified":            cliAvailable,
			"cli_available":       cliAvailable,
		}
		if !cliAvailable {
			status["authenticated"] = false
			status["credentials_present"] = false
			status["verified"] = false
			status["message"] = "CommandCode CLI is not installed on this server"
		} else if r.URL.Query().Get("verify") == "true" {
			reg := s.ProviderRegistry()
			if reg == nil {
				reg = agent.DefaultRegistry()
			}
			if prov, err := reg.Get("commandcode"); err == nil {
				if probe, err := prov.VerifyConnection(r.Context(), "subscription"); err == nil && probe != nil && probe.Valid {
					status["verified"] = true
					status["message"] = "CommandCode provider connection verified"
				}
			}
		}
		status["runtime_prerequisites"] = onboardingRuntimePrerequisites()
		respondJSON(w, http.StatusOK, status)
		return
	case "claude", "codex":
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":        "success",
			"harness":       harness,
			"authenticated": false,
			"message":       "Harness authentication is not verified by this integration",
		})
		return
	default:
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":        "success",
			"harness":       harness,
			"authenticated": false,
			"message":       "Harness authentication is not verified by this integration",
		})
		return
	}
}

// handleInitHarnessAuth initiates authentication flow for a CLI harness (e.g. agy Google OAuth).
// Route: POST /api/v1/agents/auth-init, POST /api/agents/auth-init
func (s *Server) handleInitHarnessAuth(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuthOwner(w, r) {
		return
	}
	var req struct {
		Harness string `json:"harness"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	harness := strings.ToLower(strings.TrimSpace(req.Harness))
	if harness == "" {
		harness = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("harness")))
	}
	if harness == "" || harness == "antigravity" {
		harness = "agy"
	}

	if harness == "agy" {
		if agent.IsAntigravityLoggedIn() {
			if err := agent.VerifyAntigravityConnection(r.Context()); err != nil {
				// A stored account can hit provider quota while remaining signed in.
				// Starting its interactive CLI does not produce a new OAuth URL.
				respondJSONError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":        "success",
				"harness":       "agy",
				"authenticated": true,
				"message":       "Antigravity server session is active and ready",
				"auth_url":      "",
				"verified":      true,
			})
			return
		}

		session, err := agent.StartAntigravityAuthSession(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to initiate Antigravity authentication: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":        "success",
			"harness":       "agy",
			"session_id":    session.ID,
			"expires_at":    session.CreatedAt.Add(5 * time.Minute),
			"auth_url":      session.AuthURL,
			"authenticated": false,
			"message":       "Buka link Google OAuth dan tempel kode otorisasi dari browser",
		})
		return
	}

	switch harness {
	case "commandcode":
		owner, err := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r))
		if err != nil {
			respondJSONError(w, http.StatusUnauthorized, "Masuk ke Cuan terlebih dahulu")
			return
		}
		cc := agent.StartCommandCodeAuth(owner.ID)
		if cc.AuthURL != "" {
			base := "http://" + r.Host
			if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				base = "https://" + r.Host
			}
			if rewritten, e := agent.CommandCodeAuthURL(cc.ID, cc.AuthURL, base); e == nil {
				cc.AuthURL = rewritten
			}
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "success",
			"harness":    "commandcode",
			"session_id": cc.ID, "expires_at": cc.ExpiresAt, "authenticated": cc.Authenticated, "verified": cc.Verified,
			"message": cc.Message, "auth_url": cc.AuthURL,
		})
		return
	default:
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":        "success",
			"harness":       harness,
			"authenticated": false,
			"message":       "Harness authentication is not verified by this integration",
		})
		return
	}
}

// handleCommandCodeAuthCallback bridges the provider redirect from the
// user's browser to the localhost callback listener owned by commandcode.
func (s *Server) handleCommandCodeAuthCallback(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if id == "" {
		http.Error(w, "missing auth session", http.StatusBadRequest)
		return
	}
	if err := agent.ForwardCommandCodeCallback(id, r.URL.Query()); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<p>CommandCode sign-in received. Return to Cuan to finish connecting.</p>"))
}

// handleListProviderModels returns models supported by a provider along with vision capability.
// Route: GET /api/v1/agents/models, GET /api/agents/models
func (s *Server) handleListProviderModels(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	if provider == "" {
		provider = "commandcode"
	}
	reg := s.ProviderRegistry()
	if reg == nil {
		reg = agent.DefaultRegistry()
	}
	prov, err := reg.Get(provider)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "unsupported provider: "+provider)
		return
	}
	models, err := prov.ListModels(r.Context(), "")
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list models: "+err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"provider": provider,
		"models":   models,
	})
}

// handleSubmitHarnessAuthCode exchanges the authorization code from browser Google OAuth.
// Route: POST /api/v1/agents/auth-code, POST /api/agents/auth-code
func (s *Server) handleSubmitHarnessAuthCode(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuthOwner(w, r) {
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Code      string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}
	if r.URL.Query().Get("cancel") == "true" && strings.HasPrefix(req.SessionID, "ccauth") {
		owner, ownerErr := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r))
		if ownerErr != nil {
			respondJSONError(w, 401, "Masuk ke Cuan terlebih dahulu")
			return
		}
		if err := agent.CancelCommandCodeAuth(req.SessionID, owner.ID); err != nil {
			respondJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"status": "cancelled", "authenticated": false})
		return
	}
	if strings.HasPrefix(req.SessionID, "ccauth") {
		owner, ownerErr := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r))
		if ownerErr != nil {
			respondJSONError(w, 401, "Masuk ke Cuan terlebih dahulu")
			return
		}
		if strings.TrimSpace(req.Code) == "" {
			respondJSONError(w, http.StatusBadRequest, "CommandCode sign-in confirmation is required")
			return
		}
		cc, err := agent.CompleteCommandCodeAuth(req.SessionID, owner.ID, true, true, "CommandCode subscription is connected")
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"status": "success", "authenticated": cc.Authenticated, "verified": cc.Verified, "session_id": cc.ID, "expires_at": cc.ExpiresAt, "message": cc.Message})
		return
	}

	if req.Code == "" && req.SessionID != "" && r.URL.Query().Get("cancel") == "true" {
		if !agent.CancelLegacyAntigravityAuthSession(req.SessionID) {
			respondJSONError(w, http.StatusNotFound, "Sign-in session is unavailable")
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"status": "cancelled", "authenticated": false})
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		respondJSONError(w, http.StatusBadRequest, "authorization code is required")
		return
	}

	if err := agent.SubmitAntigravityAuthCode(req.SessionID, code); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := agent.VerifyAntigravityConnection(r.Context()); err != nil {
		respondJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"verified":      true,
		"status":        "success",
		"authenticated": true,
		"message":       "Signed in successfully. Antigravity server session is active and ready.",
	})
}

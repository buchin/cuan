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

	"github.com/buchin/cuan/internal/agent"
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
		respondJSONError(w, http.StatusInternalServerError, "failed to list agents: "+err.Error())
		return
	}

	total, err := s.agentRepo.CountAgents(r.Context(), filter)
	if err != nil {
		total = len(agents)
	}

	dtos := make([]db.AgentDTO, len(agents))
	for i, a := range agents {
		dtos[i] = a.ToDTO()
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
	if enc, err := agent.EncryptCredential("host_session_active"); err == nil {
		encryptedCred = enc
	}

	planHint := "Sesi Host Server (Tanpa API Key)"
	switch entity.Harness {
	case "agy":
		planHint = "Sesi Antigravity CLI"
	case "claude":
		planHint = "Sesi Claude CLI"
	case "codex":
		planHint = "Sesi Codex CLI"
	case "custom":
		planHint = "Sesi Custom Model"
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
	_ = s.agentRepo.DeleteConnection(r.Context(), connID)
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
				Content:    fmt.Sprintf("Halo Buchin! Saya %s. Sesi agen telah terhubung via UI (%s). Saya siap menerima instruksi.", entity.Name, entity.Harness),
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

// handlePatchAgent modifies an existing agent partially (Fural format { id, name, harness, connection_id, enabled }).
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
	if rawSecret == "" && (req.AuthType == "cli_login" || req.AuthType == "oauth" || provider == "antigravity" || provider == "fural" || provider == "gemini-cli" || provider == "claude-cli" || req.SessionToken != "") {
		if req.SessionToken != "" {
			rawSecret = req.SessionToken
		} else {
			rawSecret = "cuan_cli_session_" + db.GenerateUUID()
		}
		if req.PlanHint == "" {
			req.PlanHint = "Sesi Host Server (Tanpa API Key)"
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
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
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

	modelID := strings.TrimSpace(req.ModelID)
	if modelID == "" {
		modelID = "Gemini 3.8 Flash (High-Speed)"
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "Development Team (Worker)"
	}

	sessionToken := strings.TrimSpace(req.SessionToken)
	if sessionToken == "" {
		sessionToken = fmt.Sprintf("cuan_cli_token_%s_%s", strings.TrimPrefix(handle, "@"), db.GenerateUUID())
	}

	encryptedToken, err := agent.EncryptCredential(sessionToken)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to encrypt session token: "+err.Error())
		return
	}

	// 1. Create / Persist Agent
	cleanHandle := strings.TrimPrefix(handle, "@")
	cleanHandle = strings.ReplaceAll(cleanHandle, "-", "_")
	agentID := "agent_" + cleanHandle
	connID := "conn_" + agentID
	agentDisplayName := fmt.Sprintf("%s (%s)", name, handle)

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

	_ = s.agentRepo.DeleteAgent(r.Context(), agentID)
	if err := s.agentRepo.CreateAgent(r.Context(), agentEntity); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to create agent: "+err.Error())
		return
	}

	// 2. Create / Persist Connection
	_ = s.agentRepo.DeleteConnection(r.Context(), connID)
	connEntity := &db.AgentConnection{
		ID:                   connID,
		UserID:               "default",
		Provider:             provider,
		CredentialsEncrypted: encryptedToken,
		PlanHint:             "Sesi Host Server (Tanpa API Key)",
		Status:               db.ConnectionStatusActive,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	_ = s.agentRepo.CreateConnection(r.Context(), connEntity)

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
			_ = s.projectStudioRepo.CreateRoom(r.Context(), &db.ProjectRoom{
				ID:        roomID,
				ProjectID: "default",
				Title:     agentDisplayName,
				Topic:     "agent:" + agentID,
				CreatedAt: now,
				UpdatedAt: now,
			})
			_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
				ID:         db.GenerateUUID(),
				RoomID:     roomID,
				AuthorType: "agent",
				AuthorID:   agentID,
				AuthorName: agentEntity.Name,
				Content:    fmt.Sprintf("Halo Buchin! Saya %s. Sesi CLI telah terhubung tanpa API Key (%s). Saya siap menerima instruksi chat.", agentEntity.Name, provider),
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
	} else {
		roomID = db.GenerateUUID()
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status":          "success",
		"agent":           agentEntity.ToDTO(),
		"connection":      connEntity.ToDTO(),
		"handle":          handle,
		"provider":        provider,
		"room_id":         roomID,
		"conversation_id": roomID,
		"auth_model":      "fural_cli_session",
		"message":         "Agent connected successfully via CLI login without API Key",
	})
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
		respondJSON(w, http.StatusOK, status)
		return
	}

	// For claude / codex / custom harnesses
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "success",
		"harness":       harness,
		"authenticated": true,
		"message":       fmt.Sprintf("Sesi %s siap digunakan", strings.ToUpper(harness)),
	})
}

// handleInitHarnessAuth initiates authentication flow for a CLI harness (e.g. agy Google OAuth).
// Route: POST /api/v1/agents/auth-init, POST /api/agents/auth-init
func (s *Server) handleInitHarnessAuth(w http.ResponseWriter, r *http.Request) {
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
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":        "success",
				"harness":       "agy",
				"authenticated": true,
				"message":       "Sesi Antigravity server aktif & siap digunakan",
				"auth_url":      "",
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
			"auth_url":      session.AuthURL,
			"authenticated": false,
			"message":       "Buka link Google OAuth dan tempel kode otorisasi dari browser",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "success",
		"harness":       harness,
		"authenticated": true,
		"message":       fmt.Sprintf("Sesi %s siap digunakan", strings.ToUpper(harness)),
	})
}

// handleSubmitHarnessAuthCode exchanges the authorization code from browser Google OAuth.
// Route: POST /api/v1/agents/auth-code, POST /api/agents/auth-code
func (s *Server) handleSubmitHarnessAuthCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
		Code      string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
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

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "success",
		"authenticated": true,
		"message":       "Otentikasi berhasil! Sesi Antigravity server aktif & siap digunakan.",
	})
}



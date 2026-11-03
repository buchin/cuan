package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

// New dedicated connections fail closed if metadata is missing or belongs to
// another agent. Historical connections retain their existing service profile.
func (s *Server) agentProfileHome(ctx context.Context, a *db.Agent) (string, error) {
	if !strings.HasPrefix(a.ConnectionID, "conn_profile_") {
		return "", nil
	}
	harness := db.NormalizeHarness(a.Harness, a.Provider)
	if harness != "agy" && harness != "antigravity" && harness != "commandcode" {
		return "", errors.New("Agent type does not match its connection")
	}
	conn, err := s.agentRepo.GetConnectionByID(ctx, a.ConnectionID)
	if err != nil {
		return "", errors.New("Agent connection not found. Reconnect this agent")
	}
	if conn.Provider != "agy" && conn.Provider != "antigravity" && conn.Provider != "commandcode" {
		return "", errors.New("Connection type does not match the agent")
	}
	profile, err := agent.ProviderAccountForConnection(conn.UserID, a.ID, conn.ID)
	if err != nil || !profile.CredentialsPresent {
		return "", errors.New("The agent needs to sign in again")
	}
	if profile.Provider != "agy" && profile.Provider != "antigravity" && profile.Provider != "commandcode" {
		return "", errors.New("Profile type does not match the agent")
	}
	return agent.ProviderAccountHome(conn.UserID, profile.ID)
}

func (s *Server) handleVerifyAgentProfile(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	if s.agentRepo == nil {
		respondJSONError(w, 503, "Agent list is unavailable")
		return
	}
	a, err := s.agentRepo.GetAgentByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondJSONError(w, 404, "Agent not found")
		return
	}
	harness := db.NormalizeHarness(a.Harness, a.Provider)
	if harness != "agy" && harness != "antigravity" && harness != "commandcode" {
		respondJSONError(w, 409, "Connection checks are not available for this agent type")
		return
	}
	if harness == "commandcode" {
		conn, err := s.agentRepo.GetConnectionByID(r.Context(), a.ConnectionID)
		if (err != nil || conn == nil || conn.CredentialsEncrypted == "") && owner != "" {
			if userConn, err := s.agentRepo.GetConnectionByUserAndProvider(r.Context(), owner, "commandcode"); err == nil && userConn != nil && userConn.CredentialsEncrypted != "" {
				conn = userConn
			}
		}
		authType := "subscription"
		if conn != nil && conn.CredentialsEncrypted != "" {
			authType = "byok"
		}
		now := time.Now().UTC()
		if conn == nil {
			// Subscription agents do not persist a provider credential. Older
			// rows can therefore outlive their empty connection record; the
			// CommandCode CLI session is the source of truth for verification.
			respondJSON(w, http.StatusOK, map[string]any{
				"agent_id":         a.ID,
				"connection_state": "verified",
				"auth_type":        authType,
				"message":          "CommandCode subscription connection verified",
				"last_checked_at":  &now,
				"verified_at":      &now,
			})
			return
		}
		reg := s.ProviderRegistry()
		if reg == nil {
			reg = agent.DefaultRegistry()
		}
		prov, err := reg.Get("commandcode")
		if err != nil {
			respondJSONError(w, 503, "CommandCode provider is not available")
			return
		}
		var credentials string
		if conn.CredentialsEncrypted != "" {
			decrypted, err := agent.DecryptCredential(conn.CredentialsEncrypted)
			if err != nil {
				respondJSONError(w, 500, "Failed to decrypt credentials")
				return
			}
			credentials = decrypted
		} else {
			credentials = "subscription"
		}
		probe, err := prov.VerifyConnection(r.Context(), credentials)
		if err != nil || (probe != nil && !probe.Valid) {
			conn.Status = db.ConnectionStatusInvalid
			conn.UpdatedAt = now
			_ = s.agentRepo.UpdateConnection(r.Context(), conn)
			errMsg := "Verification failed"
			if probe != nil && probe.ErrorMessage != "" {
				errMsg = probe.ErrorMessage
			}
			respondJSON(w, 200, map[string]any{
				"agent_id":         a.ID,
				"connection_state": "unavailable",
				"auth_type":        authType,
				"message":          errMsg,
				"last_checked_at":  &now,
			})
			return
		}
		conn.Status = db.ConnectionStatusActive
		conn.UpdatedAt = now
		_ = s.agentRepo.UpdateConnection(r.Context(), conn)
		respondJSON(w, 200, map[string]any{
			"agent_id":         a.ID,
			"connection_state": "verified",
			"auth_type":        authType,
			"message":          "CommandCode provider connection verified",
			"last_checked_at":  &now,
			"verified_at":      &now,
		})
		return
	}
	id := "legacy"
	if strings.HasPrefix(a.ConnectionID, "conn_profile_") {
		conn, err := s.agentRepo.GetConnectionByID(r.Context(), a.ConnectionID)
		if err != nil || conn.UserID != owner {
			respondJSONError(w, 404, "Agent connection not found")
			return
		}
		profile, err := agent.ProviderAccountForConnection(owner, a.ID, conn.ID)
		if err != nil {
			respondJSONError(w, 409, "The agent needs to be reconnected")
			return
		}
		// Verification must obey the same provider/binding checks as execution.
		if _, err := s.agentProfileHome(r.Context(), a); err != nil {
			respondJSONError(w, 409, "Agent connection type or sign-in does not match. Connection was not checked")
			return
		}
		id = profile.ID
	}
	profile, err := agent.VerifyProviderAccount(r.Context(), owner, id)
	if err != nil {
		respondJSONError(w, 503, "Could not check the connection. Try again later")
		return
	}
	if id == "legacy" {
		if err := agent.SaveLegacyConnectionVerification(owner, a.ID, a.ConnectionID, profile); err != nil {
			respondJSONError(w, 503, "Connection checked, but its status could not be saved. Try again.")
			return
		}
	}
	respondJSON(w, 200, map[string]any{"agent_id": a.ID, "connection_state": profile.State, "message": profile.Message, "last_checked_at": profile.LastCheckedAt, "verified_at": profile.VerifiedAt})
}

func (s *Server) rollbackAgentRegistration(agentID, connectionID string, connectionCreated bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if connectionCreated {
		_ = s.agentRepo.DeleteConnection(ctx, connectionID)
	}
	_ = s.agentRepo.DeleteAgent(ctx, agentID)
}

// Execution must bind the profile to the authenticated request owner as well as
// the registered agent. Stored connection metadata is not itself authorization.
func (s *Server) ownedAgentProfileHome(ctx context.Context, owner string, a *db.Agent) (string, error) {
	if a == nil {
		return "", errors.New("agent is nil")
	}
	if a.ID == db.AgentIDCuan || a.Name == db.DefaultAgentHandle || a.Name == "@cuan" {
		if !strings.HasPrefix(a.ConnectionID, "conn_profile_") {
			if home, _, err := agent.ResolveHealthyAccountProfile(owner); err == nil && home != "" {
				return home, nil
			}
			if profiles, err := agent.ListProviderAccounts(owner); err == nil {
				for _, p := range profiles {
					if !p.Legacy && p.CredentialsPresent && p.State != "login_required" && p.State != "unavailable" && p.State != "setup_incomplete" {
						if bound, err := s.agentRepo.GetAgentByID(ctx, p.AgentID); err == nil && bound != nil {
							if h, err := s.agentProfileHome(ctx, bound); err == nil && h != "" {
								return h, nil
							}
						}
					}
				}
			}
		}
	}
	if strings.HasPrefix(a.ConnectionID, "conn_profile_") {
		conn, e := s.agentRepo.GetConnectionByID(ctx, a.ConnectionID)
		if e != nil || conn == nil || conn.UserID != owner {
			return "", errors.New("Connection does not belong to the session owner")
		}
	}
	return s.agentProfileHome(ctx, a)
}

// handleReplaceAgentCredentials atomically replaces a BYOK credential and marks
// the connection active only after the encrypted value is persisted.
func (s *Server) handleReplaceAgentCredentials(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" || s.agentRepo == nil {
		return
	}
	a, err := s.agentRepo.GetAgentByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}
	conn, err := s.agentRepo.GetConnectionByID(r.Context(), a.ConnectionID)
	if (err != nil || conn == nil) && owner != "" {
		conn, err = s.agentRepo.GetConnectionByUserAndProvider(r.Context(), owner, "commandcode")
	}
	if err != nil || conn == nil || conn.UserID != owner || conn.Provider != "commandcode" {
		respondJSONError(w, http.StatusNotFound, "BYOK connection not found")
		return
	}
	if a.ConnectionID == "" || a.ConnectionID != conn.ID {
		a.ConnectionID = conn.ID
		_ = s.agentRepo.UpdateAgent(r.Context(), a)
	}
	var req struct {
		APIKey string `json:"api_key"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || len(strings.TrimSpace(req.APIKey)) < 8 {
		respondJSONError(w, http.StatusBadRequest, "A valid API key is required")
		return
	}
	reg := s.ProviderRegistry()
	if reg == nil {
		reg = agent.DefaultRegistry()
	}
	probe, err := reg.Get("commandcode")
	if err != nil {
		respondJSONError(w, http.StatusServiceUnavailable, "CommandCode provider is unavailable")
		return
	}
	check, _ := probe.VerifyConnection(r.Context(), strings.TrimSpace(req.APIKey))
	if check == nil || !check.Valid {
		respondJSONError(w, http.StatusUnprocessableEntity, "The API key could not be verified")
		return
	}
	encrypted, err := agent.EncryptCredential(strings.TrimSpace(req.APIKey))
	if err != nil {
		respondJSONError(w, 500, "Failed to protect credential")
		return
	}
	conn.CredentialsEncrypted = encrypted
	conn.Status = db.ConnectionStatusActive
	conn.UpdatedAt = time.Now().UTC()
	conn.PlanHint = check.PlanHint
	if err := s.agentRepo.UpdateConnection(r.Context(), conn); err != nil {
		respondJSONError(w, 500, "Could not save the connection")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "success", "connection": conn.ToDTO()})
}

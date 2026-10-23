package server

import (
	"context"
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
	if a.Harness != "agy" && a.Harness != "antigravity" && a.Harness != "commandcode" {
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
	if a.Harness != "agy" && a.Provider != "agy" && a.Provider != "antigravity" {
		respondJSONError(w, 409, "Connection checks are not available for this agent type")
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
	if strings.HasPrefix(a.ConnectionID, "conn_profile_") {
		conn, e := s.agentRepo.GetConnectionByID(ctx, a.ConnectionID)
		if e != nil || conn == nil || conn.UserID != owner {
			return "", errors.New("Connection does not belong to the session owner")
		}
	}
	return s.agentProfileHome(ctx, a)
}

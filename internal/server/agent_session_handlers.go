package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
)

// handleGetAgentBinaries returns available agent CLI binaries detected on the host.
// GET /api/v1/agents/binaries
func (s *Server) handleGetAgentBinaries(w http.ResponseWriter, r *http.Request) {
	bins := agent.DetectAgentBinaries()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"binaries": bins,
		"count":    len(bins),
	})
}

// pairAgentPayload represents the request body for device-pairing an agent CLI.
type pairAgentPayload struct {
	DeviceCode string `json:"device_code"`
	MemberID   string `json:"member_id,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	AgentName  string `json:"agent_name,omitempty"`
	Provider   string `json:"provider,omitempty"`
	BinaryType string `json:"binary_type,omitempty"`
}

// handlePairAgentDevice authorizes an agent account via device pairing code.
// POST /api/v1/agents/pair
func (s *Server) handlePairAgentDevice(w http.ResponseWriter, r *http.Request) {
	if s.agentSessionStore == nil {
		s.agentSessionStore = agent.NewMemorySessionStore()
	}

	var payload pairAgentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	code := strings.TrimSpace(payload.DeviceCode)
	if code == "" {
		respondJSONError(w, http.StatusBadRequest, "device_code is required")
		return
	}

	memberID := strings.TrimSpace(payload.MemberID)
	if memberID == "" {
		// Attempt to resolve from auth token
		if owner, err := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r)); err == nil && owner != nil {
			if owner.Username != "" {
				memberID = owner.Username
			} else {
				memberID = owner.Email
			}
		} else {
			memberID = "buchin" // default owner member
		}
	}

	agentID := strings.TrimSpace(payload.AgentID)
	if agentID == "" {
		respondJSONError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	sess, err := s.agentSessionStore.AuthorizeDevicePairing(r.Context(), agent.AuthorizePairingRequest{
		DeviceCode: code,
		MemberID:   memberID,
		AgentID:    agentID,
		AgentName:  payload.AgentName,
		Provider:   payload.Provider,
		BinaryType: payload.BinaryType,
	})
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "paired",
		"session": sess,
	})
}

// handleListAgentSessions returns active sessions for a member.
// GET /api/v1/agents/sessions
func (s *Server) handleListAgentSessions(w http.ResponseWriter, r *http.Request) {
	if s.agentSessionStore == nil {
		s.agentSessionStore = agent.NewMemorySessionStore()
	}

	memberID := strings.TrimSpace(r.URL.Query().Get("member_id"))
	if memberID == "" {
		if owner, err := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r)); err == nil && owner != nil {
			if owner.Username != "" {
				memberID = owner.Username
			} else {
				memberID = owner.Email
			}
		}
	}

	sessions, err := s.agentSessionStore.ListSessions(r.Context(), memberID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// handleRevokeAgentSession revokes an active session.
// DELETE /api/v1/agents/sessions/{id}
func (s *Server) handleRevokeAgentSession(w http.ResponseWriter, r *http.Request) {
	if s.agentSessionStore == nil {
		s.agentSessionStore = agent.NewMemorySessionStore()
	}

	sessionID := strings.TrimSpace(chi.URLParam(r, "id"))
	if sessionID == "" {
		respondJSONError(w, http.StatusBadRequest, "session id is required")
		return
	}

	if err := s.agentSessionStore.RevokeSession(r.Context(), sessionID); err != nil {
		respondJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "revoked",
		"id":     sessionID,
	})
}

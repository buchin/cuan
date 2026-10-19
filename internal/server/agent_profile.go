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
	if a.Harness != "agy" && a.Harness != "antigravity" {
		return "", errors.New("Jenis agen tidak sesuai dengan koneksinya")
	}
	conn, err := s.agentRepo.GetConnectionByID(ctx, a.ConnectionID)
	if err != nil {
		return "", errors.New("Koneksi agen tidak ditemukan. Hubungkan ulang agen ini")
	}
	profile, err := agent.ProviderAccountForConnection(conn.UserID, a.ID, conn.ID)
	if err != nil || !profile.CredentialsPresent {
		return "", errors.New("Agen perlu login ulang")
	}
	return agent.ProviderAccountHome(conn.UserID, profile.ID)
}

func (s *Server) handleVerifyAgentProfile(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	if s.agentRepo == nil {
		respondJSONError(w, 503, "Daftar agen tidak tersedia")
		return
	}
	a, err := s.agentRepo.GetAgentByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondJSONError(w, 404, "Agen tidak ditemukan")
		return
	}
	if a.Harness != "agy" && a.Provider != "agy" && a.Provider != "antigravity" {
		respondJSONError(w, 409, "Pemeriksaan jenis agen ini belum tersedia")
		return
	}
	id := "legacy"
	if strings.HasPrefix(a.ConnectionID, "conn_profile_") {
		conn, err := s.agentRepo.GetConnectionByID(r.Context(), a.ConnectionID)
		if err != nil || conn.UserID != owner {
			respondJSONError(w, 404, "Koneksi agen tidak ditemukan")
			return
		}
		profile, err := agent.ProviderAccountForConnection(owner, a.ID, conn.ID)
		if err != nil {
			respondJSONError(w, 409, "Agen perlu dihubungkan ulang")
			return
		}
		id = profile.ID
	}
	profile, err := agent.VerifyProviderAccount(r.Context(), owner, id)
	if err != nil {
		respondJSONError(w, 503, "Koneksi belum dapat diperiksa. Coba lagi nanti")
		return
	}
	respondJSON(w, 200, map[string]any{"agent_id": a.ID, "connection_state": profile.State, "message": profile.Message})
}

func (s *Server) rollbackAgentRegistration(agentID, connectionID string, connectionCreated bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if connectionCreated {
		_ = s.agentRepo.DeleteConnection(ctx, connectionID)
	}
	_ = s.agentRepo.DeleteAgent(ctx, agentID)
}

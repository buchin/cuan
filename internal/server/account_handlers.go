package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/go-chi/chi/v5"
)

func (s *Server) accountOwner(w http.ResponseWriter, r *http.Request) string {
	if s.authSvc == nil {
		respondJSONError(w, 503, "Owner authentication is unavailable")
		return ""
	}
	owner, err := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r))
	if err != nil {
		respondJSONError(w, 401, "Masuk ke Cuan terlebih dahulu")
		return ""
	}
	return owner.ID
}

func (s *Server) handleListProviderAccounts(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	accounts, err := agent.ListProviderAccounts(owner)
	if err != nil {
		respondJSONError(w, 500, "Could not load accounts")
		return
	}
	respondJSON(w, 200, map[string]any{"accounts": accounts})
}
func (s *Server) handleCreateProviderAccount(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	var req struct {
		Label string `json:"label"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		respondJSONError(w, 400, "Invalid account name")
		return
	}
	a, err := agent.CreateProviderAccount(owner, req.Label)
	if err != nil {
		respondJSONError(w, 400, "Could not create the account. Enter a name of up to 100 characters.")
		return
	}
	respondJSON(w, 201, map[string]any{"account": a})
}
func (s *Server) handleProviderAccountAuthInit(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	id := chi.URLParam(r, "accountID")
	if _, err := agent.GetProviderAccount(owner, id); err != nil {
		respondJSONError(w, 404, "Account not found")
		return
	}
	session, err := agent.StartProviderAccountAuth(r.Context(), owner, id)
	if err != nil {
		respondJSONError(w, 409, err.Error())
		return
	}
	respondJSON(w, 200, map[string]any{"account_id": id, "session_id": session.ID, "auth_url": session.AuthURL, "expires_at": session.CreatedAt.Add(5 * time.Minute), "authenticated": false, "verified": false})
}
func (s *Server) handleProviderAccountAuthCode(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	id := chi.URLParam(r, "accountID")
	var req struct {
		SessionID string `json:"session_id"`
		Code      string `json:"code"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req) != nil {
		respondJSONError(w, 400, "Invalid authorization code request")
		return
	}
	if r.URL.Query().Get("cancel") == "true" {
		if err := agent.CancelProviderAccountAuth(owner, id, req.SessionID); err != nil {
			respondJSONError(w, 404, "Sign-in session is unavailable")
			return
		}
		respondJSON(w, 200, map[string]any{"status": "cancelled"})
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		respondJSONError(w, 400, "Enter an authorization code")
		return
	}
	if err := agent.SubmitProviderAccountAuth(owner, id, req.SessionID, req.Code); err != nil {
		respondJSONError(w, 400, err.Error())
		return
	}
	a, err := agent.VerifyProviderAccount(r.Context(), owner, id)
	if err != nil {
		respondJSONError(w, 500, "Sign-in processed, but account status could not be saved. Reload the account list.")
		return
	}
	// Successful exchange and provider availability are distinct; quota never discards login.
	respondJSON(w, 200, map[string]any{"account": a, "credentials_saved": a.CredentialsPresent})
}
func (s *Server) handleVerifyProviderAccount(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	a, err := agent.VerifyProviderAccount(r.Context(), owner, chi.URLParam(r, "accountID"))
	if err != nil {
		respondJSONError(w, 404, "Account is unavailable for verification")
		return
	}
	respondJSON(w, 200, map[string]any{"account": a})
}

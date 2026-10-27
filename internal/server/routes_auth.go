package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/buchin/cuan/internal/auth"
	"github.com/go-chi/chi/v5"
)

// registerAuthV1Routes mounts authentication endpoints inside /api/v1 router.
func (s *Server) registerAuthV1Routes(r chi.Router) {
	// Public auth endpoints
	r.Get("/auth/status", s.handleAuthStatus)
	r.Post("/auth/setup", s.handleAuthSetup)
	r.Post("/auth/login", s.handleAuthLogin)
	r.Post("/auth/logout", s.handleAuthLogout)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.authSvc.Status(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to check auth status")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	var req auth.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.authSvc.Setup(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrSetupAlreadyCompleted) {
			respondJSONError(w, http.StatusForbidden, "setup already completed: single owner already exists")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("setup failed: %v", err))
		return
	}

	setSessionCookie(w, r, resp.Token)
	respondJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	identifier := strings.TrimSpace(req.Username)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}

	resp, err := s.authSvc.Login(r.Context(), identifier, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrSetupRequired) {
			respondJSONError(w, http.StatusBadRequest, "setup required: no owner account exists")
			return
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			respondJSONError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("login failed: %v", err))
		return
	}

	setSessionCookie(w, r, resp.Token)
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.ExtractToken(r)
	if token != "" {
		_ = s.authSvc.Logout(r.Context(), token)
	}

	clearSessionCookie(w)
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	owner := auth.GetOwnerFromContext(r.Context())
	if owner == nil {
		respondJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ownerPreferencesMu.Lock()
	preferences, _ := readOwnerPreferences(owner.ID)
	ownerPreferencesMu.Unlock()
	userData := map[string]interface{}{
		"display_name": preferences.DisplayName,
		"id":           owner.ID,
		"username":     owner.Username,
		"email":        owner.Email,
		"created_at":   owner.CreatedAt,
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"owner": userData,
		"user":  userData,
	})
}

func (s *Server) handleAuthPassword(w http.ResponseWriter, r *http.Request) {
	owner := auth.GetOwnerFromContext(r.Context())
	if owner == nil {
		respondJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req auth.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.authSvc.ChangePassword(r.Context(), owner.ID, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			respondJSONError(w, http.StatusUnauthorized, "invalid current password")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update password: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "password updated successfully",
	})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	isTLS := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "cuan_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isTLS,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.SessionDuration.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "cuan_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

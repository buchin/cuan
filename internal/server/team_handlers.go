package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/buchin/cuan/internal/auth"
	"github.com/go-chi/chi/v5"
)

// requireOwnerRole checks that if an authenticated session exists or is required,
// the caller must have the "owner" role. If the caller has "member" role, it returns 403 Forbidden.
func (s *Server) requireOwnerRole(w http.ResponseWriter, r *http.Request) bool {
	owner := auth.GetOwnerFromContext(r.Context())
	if owner == nil && s.authSvc != nil {
		token := auth.ExtractToken(r)
		if token != "" {
			var err error
			owner, err = s.authSvc.ValidateToken(r.Context(), token)
			if err != nil {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: invalid or expired session")
				return false
			}
		}
	}

	if owner != nil {
		role := strings.ToLower(strings.TrimSpace(owner.Role))
		if role == auth.RoleMember {
			respondJSONError(w, http.StatusForbidden, "forbidden: owner role required")
			return false
		}
	}

	return true
}

func (s *Server) handleListTeamMembers(w http.ResponseWriter, r *http.Request) {
	if s.authSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	members, invites, err := s.authSvc.ListTeamMembers(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list team members: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"members":     members,
		"invitations": invites,
	})
}

func (s *Server) handleCreateTeamInvite(w http.ResponseWriter, r *http.Request) {
	if s.authSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	owner := auth.GetOwnerFromContext(r.Context())
	if owner == nil {
		token := auth.ExtractToken(r)
		if token != "" {
			var err error
			owner, err = s.authSvc.ValidateToken(r.Context(), token)
			if err != nil {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: invalid session")
				return
			}
		}
	}
	if owner == nil {
		respondJSONError(w, http.StatusUnauthorized, "unauthorized: authentication required")
		return
	}
	if strings.ToLower(strings.TrimSpace(owner.Role)) == auth.RoleMember {
		respondJSONError(w, http.StatusForbidden, "forbidden: owner role required")
		return
	}

	var req auth.CreateInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.authSvc.CreateInvite(r.Context(), owner.ID, req.Email, req.Role)
	if err != nil {
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			respondJSONError(w, http.StatusConflict, "user with this email already exists")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to create invite: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleDeleteTeamInvite(w http.ResponseWriter, r *http.Request) {
	if s.authSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	owner := auth.GetOwnerFromContext(r.Context())
	if owner == nil {
		token := auth.ExtractToken(r)
		if token != "" {
			var err error
			owner, err = s.authSvc.ValidateToken(r.Context(), token)
			if err != nil {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: invalid session")
				return
			}
		}
	}
	if owner == nil {
		respondJSONError(w, http.StatusUnauthorized, "unauthorized: authentication required")
		return
	}
	if strings.ToLower(strings.TrimSpace(owner.Role)) == auth.RoleMember {
		respondJSONError(w, http.StatusForbidden, "forbidden: owner role required")
		return
	}

	inviteID := strings.TrimSpace(chi.URLParam(r, "id"))
	if inviteID == "" {
		respondJSONError(w, http.StatusBadRequest, "invite id is required")
		return
	}

	if err := s.authSvc.DeleteInvite(r.Context(), inviteID); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to delete invite: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "invitation deleted successfully",
	})
}

func (s *Server) handleDeleteTeamMember(w http.ResponseWriter, r *http.Request) {
	if s.authSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	owner := auth.GetOwnerFromContext(r.Context())
	if owner == nil {
		token := auth.ExtractToken(r)
		if token != "" {
			var err error
			owner, err = s.authSvc.ValidateToken(r.Context(), token)
			if err != nil {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: invalid session")
				return
			}
		}
	}
	if owner == nil {
		respondJSONError(w, http.StatusUnauthorized, "unauthorized: authentication required")
		return
	}
	if strings.ToLower(strings.TrimSpace(owner.Role)) == auth.RoleMember {
		respondJSONError(w, http.StatusForbidden, "forbidden: owner role required")
		return
	}

	memberID := strings.TrimSpace(chi.URLParam(r, "id"))
	if memberID == "" {
		respondJSONError(w, http.StatusBadRequest, "member id is required")
		return
	}

	if owner.ID == memberID {
		respondJSONError(w, http.StatusBadRequest, "cannot delete your own active owner account")
		return
	}

	if err := s.authSvc.DeleteMember(r.Context(), memberID); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to delete member: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "team member removed successfully",
	})
}

func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	if s.authSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	var req auth.AcceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.authSvc.AcceptInvite(r.Context(), req.Token, req.Name, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvitationNotFound) {
			respondJSONError(w, http.StatusNotFound, "invitation not found or invalid token")
			return
		}
		if errors.Is(err, auth.ErrInvitationAccepted) {
			respondJSONError(w, http.StatusBadRequest, "invitation has already been accepted")
			return
		}
		if errors.Is(err, auth.ErrInvitationExpired) {
			respondJSONError(w, http.StatusBadRequest, "invitation has expired")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			respondJSONError(w, http.StatusConflict, "user with this email already exists")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to accept invite: "+err.Error())
		return
	}

	setSessionCookie(w, r, resp.Token)
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleVerifyInvite(w http.ResponseWriter, r *http.Request) {
	if s.authSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		respondJSONError(w, http.StatusBadRequest, "token query parameter is required")
		return
	}

	inv, err := s.authSvc.VerifyInvite(r.Context(), token)
	if err != nil {
		if errors.Is(err, auth.ErrInvitationNotFound) {
			respondJSONError(w, http.StatusNotFound, "invitation not found")
			return
		}
		if errors.Is(err, auth.ErrInvitationAccepted) {
			respondJSONError(w, http.StatusBadRequest, "invitation has already been accepted")
			return
		}
		if errors.Is(err, auth.ErrInvitationExpired) {
			respondJSONError(w, http.StatusBadRequest, "invitation has expired")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"valid":  true,
		"email":  inv.Email,
		"role":   inv.Role,
	})
}

// registerTeamV1Routes mounts team collaboration endpoints inside /api/v1 router.
func (s *Server) registerTeamV1Routes(r chi.Router) {
	r.Route("/team", func(tr chi.Router) {
		tr.Get("/members", s.handleListTeamMembers)
		tr.Post("/invites", s.handleCreateTeamInvite)
		tr.Delete("/invites/{id}", s.handleDeleteTeamInvite)
		tr.Delete("/members/{id}", s.handleDeleteTeamMember)
	})
}

// registerTeamRoutes mounts team collaboration endpoints on root router.
func (s *Server) registerTeamRoutes(r chi.Router) {
	r.Route("/api/team", func(tr chi.Router) {
		tr.Get("/members", s.handleListTeamMembers)
		tr.Post("/invites", s.handleCreateTeamInvite)
		tr.Delete("/invites/{id}", s.handleDeleteTeamInvite)
		tr.Delete("/members/{id}", s.handleDeleteTeamMember)
	})
	r.Post("/api/auth/invites/accept", s.handleAcceptInvite)
	r.Get("/api/auth/invites/verify", s.handleVerifyInvite)
}


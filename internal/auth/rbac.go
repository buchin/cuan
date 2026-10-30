package auth

import (
	"net/http"
	"strings"
)

const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

// RequireRole checks if the authenticated user has one of the specified roles.
// If the user is unauthenticated, it returns 401 Unauthorized.
// If the user's role is not allowed, it returns 403 Forbidden.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, r := range roles {
		allowed[strings.ToLower(strings.TrimSpace(r))] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			owner := GetOwnerFromContext(r.Context())
			if owner == nil {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: authentication required")
				return
			}

			role := strings.ToLower(strings.TrimSpace(owner.Role))
			if role == "" {
				role = RoleOwner // default backward-compatible
			}

			if !allowed[role] {
				respondJSONError(w, http.StatusForbidden, "forbidden: insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireOwner is shorthand for RequireRole(RoleOwner).
func RequireOwner() func(http.Handler) http.Handler {
	return RequireRole(RoleOwner)
}

// RequireRoleWithService verifies token from request if not already in context, then checks role.
func RequireRoleWithService(svc *Service, roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, r := range roles {
		allowed[strings.ToLower(strings.TrimSpace(r))] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			owner := GetOwnerFromContext(r.Context())
			if owner == nil && svc != nil {
				token := ExtractToken(r)
				if token != "" {
					var err error
					owner, err = svc.ValidateToken(r.Context(), token)
					if err != nil {
						respondJSONError(w, http.StatusUnauthorized, "unauthorized: invalid or expired session")
						return
					}
					r = r.WithContext(ContextWithOwner(r.Context(), owner))
				}
			}

			if owner == nil {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: authentication required")
				return
			}

			role := strings.ToLower(strings.TrimSpace(owner.Role))
			if role == "" {
				role = RoleOwner
			}

			if !allowed[role] {
				respondJSONError(w, http.StatusForbidden, "forbidden: insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireOwnerWithService is shorthand for RequireRoleWithService(svc, RoleOwner).
func RequireOwnerWithService(svc *Service) func(http.Handler) http.Handler {
	return RequireRoleWithService(svc, RoleOwner)
}

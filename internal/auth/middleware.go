package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const ownerContextKey = contextKey("cuan_auth_owner")

// DefaultPublicBypassPaths defines endpoints that bypass authentication.
var DefaultPublicBypassPaths = []string{
	"/healthz",
	"/readyz",
	"/api/info",
	"/api/mode",
	"/api/caddy/ask",
	"/api/v1/auth/login",
	"/api/v1/auth/status",
	"/api/v1/auth/setup",
	"/api/v1/auth/invites/accept",
	"/api/v1/auth/invites/verify",
	"/api/auth/invites/accept",
	"/api/auth/invites/verify",
	"/invite/accept",
	"/api/v1/info",
	"/api/v1/system/mode",
}

// AuthGuard returns a middleware that checks authentication while bypassing configured public routes.
func AuthGuard(svc *Service, customBypass ...string) func(http.Handler) http.Handler {
	bypassMap := make(map[string]bool)
	for _, p := range DefaultPublicBypassPaths {
		bypassMap[strings.ToLower(p)] = true
	}
	for _, p := range customBypass {
		bypassMap[strings.ToLower(p)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := strings.ToLower(r.URL.Path)
			cleanPath := strings.TrimSuffix(path, "/")

			if bypassMap[path] || bypassMap[cleanPath] {
				// If token is present, optionally attach owner to context without blocking
				token := ExtractToken(r)
				if token != "" {
					if owner, err := svc.ValidateToken(r.Context(), token); err == nil && owner != nil {
						r = r.WithContext(context.WithValue(r.Context(), ownerContextKey, owner))
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			token := ExtractToken(r)
			if token == "" {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: authentication required")
				return
			}

			owner, err := svc.ValidateToken(r.Context(), token)
			if err != nil {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: invalid or expired session")
				return
			}

			ctx := context.WithValue(r.Context(), ownerContextKey, owner)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Middleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ExtractToken(r)
			if token == "" {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: authentication required")
				return
			}

			owner, err := svc.ValidateToken(r.Context(), token)
			if err != nil {
				respondJSONError(w, http.StatusUnauthorized, "unauthorized: invalid or expired session")
				return
			}

			ctx := context.WithValue(r.Context(), ownerContextKey, owner)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ExtractToken(r *http.Request) string {
	// 1. Check Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// 2. Check Cookie "cuan_session"
	cookie, err := r.Cookie("cuan_session")
	if err == nil && cookie.Value != "" {
		return strings.TrimSpace(cookie.Value)
	}

	return ""
}

// ContextWithOwner returns a new context with the given Owner attached.
func ContextWithOwner(ctx context.Context, owner *Owner) context.Context {
	return context.WithValue(ctx, ownerContextKey, owner)
}

func GetOwnerFromContext(ctx context.Context) *Owner {
	val := ctx.Value(ownerContextKey)
	if val == nil {
		return nil
	}
	owner, ok := val.(*Owner)
	if !ok {
		return nil
	}
	return owner
}

func respondJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
}

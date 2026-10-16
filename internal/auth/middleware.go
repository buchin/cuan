package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type contextKey string

const ownerContextKey = contextKey("cuan_auth_owner")

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

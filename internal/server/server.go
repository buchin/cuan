package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
)

type Server struct {
	cfg     *config.Config
	db      *sql.DB
	authSvc *auth.Service
	router  *chi.Mux
}

func NewServer(cfg *config.Config, db *sql.DB, authSvc ...*auth.Service) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	var svc *auth.Service
	if len(authSvc) > 0 && authSvc[0] != nil {
		svc = authSvc[0]
	} else if db != nil {
		svc = auth.NewService(auth.NewDBStore(db))
	} else {
		svc = auth.NewService(auth.NewMemoryStore())
	}

	s := &Server{
		cfg:     cfg,
		db:      db,
		authSvc: svc,
		router:  r,
	}

	s.routes()
	s.setupStaticRoutes()
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) AuthService() *auth.Service {
	return s.authSvc
}

func (s *Server) routes() {
	// Public Health & Readiness
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/readyz", s.handleReadyz)

	s.router.Route("/api/v1", func(r chi.Router) {
		// Public endpoints
		r.Get("/info", s.handleInfo)
		r.Get("/auth/status", s.handleAuthStatus)
		r.Post("/auth/setup", s.handleAuthSetup)
		r.Post("/auth/login", s.handleAuthLogin)
		r.Post("/auth/logout", s.handleAuthLogout)

		// Protected endpoints (Single-Owner access only)
		r.Group(func(pr chi.Router) {
			pr.Use(auth.Middleware(s.authSvc))

			pr.Get("/auth/me", s.handleAuthMe)
			pr.Get("/tasks", s.handleTasks)
			pr.Post("/tasks", s.handleCreateTask)
			pr.Get("/projects", s.handleProjects)
		})
	})
}

func (s *Server) setupStaticRoutes() {
	staticDir := s.cfg.StaticDir
	if staticDir == "" {
		staticDir = "web/dist"
	}

	if fi, err := os.Stat(staticDir); err == nil && fi.IsDir() {
		fileServer := http.FileServer(http.Dir(staticDir))
		s.router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(r.URL.Path)
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") {
				http.NotFound(w, r)
				return
			}

			filePath := filepath.Join(staticDir, path)
			if f, err := os.Stat(filePath); err == nil && !f.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}

			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		})
	} else {
		s.router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(r.URL.Path)
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Cuan Server</title></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; padding: 2rem; max-width: 600px; margin: 0 auto; line-height: 1.5;">
  <h2>Cuan API Server</h2>
  <p>Status: Healthy. Backend services are operational.</p>
  <p>To view the React UI shell, build the frontend bundle:</p>
  <pre style="background: #f1f5f9; padding: 1rem; border-radius: 6px;">cd web && npm run build</pre>
  <ul>
    <li><a href="/healthz">/healthz</a> — Liveness check</li>
    <li><a href="/readyz">/readyz</a> — Readiness check</li>
    <li><a href="/api/v1/info">/api/v1/info</a> — Server metadata</li>
  </ul>
</body>
</html>`))
		})
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"status":    "healthy",
		"app":       "cuan",
		"version":   "0.1.0-dev",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		if s.cfg.DatabaseURL == "" {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":   "ready",
				"database": "in_memory_dev",
			})
			return
		}
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":   "not_ready",
			"database": "disconnected",
			"error":    "database pool is not initialized",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":   "not_ready",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	var migrationCount int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM cuan_schema_migrations").Scan(&migrationCount)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":     "not_ready",
			"database":   "connected",
			"migrations": "table_missing_or_error",
			"error":      err.Error(),
		})
		return
	}

	status, _ := s.authSvc.Status(ctx)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "ready",
		"database":         "connected",
		"migrations":       "applied",
		"migrations_count": migrationCount,
		"owner_configured": status.OwnerConfigured,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"name":         "Cuan",
		"description":  "Agentic project management for self-hosted teams",
		"license":      "Apache-2.0",
		"sprint":       "S1",
		"status":       "running",
		"adapter":      "agy",
		"architecture": "Go backend + React Vite TS frontend",
	}
	respondJSON(w, http.StatusOK, resp)
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

	resp, err := s.authSvc.Login(r.Context(), req.Email, req.Password)
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

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"owner": map[string]interface{}{
			"id":         owner.ID,
			"email":      owner.Email,
			"created_at": owner.CreatedAt,
		},
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	// Protected task list for authenticated owner
	tasks := []map[string]interface{}{
		{
			"id":       "T1",
			"title":    "Tetapkan fondasi monorepo dan kelayakan adapter agent",
			"status":   "done",
			"priority": "high",
		},
		{
			"id":       "T2",
			"title":    "Sediakan instalasi persisten dan akses owner",
			"status":   "doing",
			"priority": "high",
		},
		{
			"id":       "T3",
			"title":    "Hubungkan repository GitHub existing",
			"status":   "backlog",
			"priority": "high",
		},
		{
			"id":       "T4",
			"title":    "Bangun task board dan histori pekerjaan",
			"status":   "backlog",
			"priority": "normal",
		},
		{
			"id":       "T5",
			"title":    "Jalankan task melalui runner dan satu agent",
			"status":   "backlog",
			"priority": "high",
		},
		{
			"id":       "T6",
			"title":    "Hasilkan perubahan Git dan PR yang dapat direview",
			"status":   "backlog",
			"priority": "high",
		},
		{
			"id":       "T7",
			"title":    "Buktikan staging, update, backup, dan pemulihan",
			"status":   "backlog",
			"priority": "normal",
		},
		{
			"id":       "T8",
			"title":    "Demo dogfooding dan siapkan release OSS",
			"status":   "backlog",
			"priority": "normal",
		},
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title        string `json:"title"`
		Instructions string `json:"instructions"`
		Priority     string `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		respondJSONError(w, http.StatusBadRequest, "task title is required")
		return
	}

	task := map[string]interface{}{
		"id":           fmt.Sprintf("task_%d", time.Now().Unix()),
		"title":        strings.TrimSpace(body.Title),
		"instructions": strings.TrimSpace(body.Instructions),
		"status":       "backlog",
		"priority":     body.Priority,
		"created_at":   time.Now().UTC().Format(time.RFC3339),
	}
	respondJSON(w, http.StatusCreated, task)
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects := []map[string]interface{}{
		{
			"id":             "proj_cuan",
			"name":           "cuan",
			"repo_url":       "https://github.com/buchin/cuan",
			"default_branch": "main",
			"status":         "active",
		},
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
		"total":    len(projects),
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

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondJSONError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": message,
	})
}

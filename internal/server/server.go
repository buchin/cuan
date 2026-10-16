package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/buchin/cuan/internal/config"
)

type Server struct {
	cfg    *config.Config
	router *chi.Mux
}

func NewServer(cfg *config.Config) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	s := &Server{
		cfg:    cfg,
		router: r,
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.Get("/healthz", s.handleHealthz)

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/info", s.handleInfo)
		r.Get("/tasks", s.handleTasks)
	})
}

func (s *Server) Router() http.Handler {
	return s.router
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

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	// Baseline initial task response
	tasks := []map[string]interface{}{
		{
			"id":       "T1",
			"title":    "Tetapkan fondasi dan kelayakan agent",
			"status":   "doing",
			"priority": "high",
		},
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

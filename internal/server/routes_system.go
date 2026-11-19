package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/scheduler"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/updater"
)

func (s *Server) registerSystemRoutes(r chi.Router) {
	r.Get("/healthz", s.handleHealthz)
	r.Head("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)
	r.Head("/readyz", s.handleReadyz)
	r.Get("/api/info", s.handleInfo)
	r.Get("/api/mode", s.handleGetSystemMode)
	r.Get("/api/system/mode", s.handleGetSystemMode)
	r.Get("/api/system/update", s.handleGetSystemUpdate)
	r.Post("/api/system/update/check", s.handleCheckSystemUpdate)
	r.Patch("/api/system/update/settings", s.handleUpdateSystemUpdateSettings)
	r.Get("/api/system/scheduler/jobs", s.handleListScheduledJobs)
	r.Post("/api/system/scheduler/jobs/{id}/trigger", s.handleTriggerScheduledJob)
	r.Get("/api/system/scheduler/tasks", s.handleListScheduledTasks)
	r.Post("/api/system/scheduler/tasks", s.handleCreateScheduledTask)
	r.Delete("/api/system/scheduler/tasks/{id}", s.handleDeleteScheduledTask)
	r.Patch("/api/system/scheduler/tasks/{id}/toggle", s.handleToggleScheduledTask)
	r.Post("/api/system/scheduler/tasks/{id}/trigger", s.handleTriggerScheduledTask)
	r.Get("/api/system/worker-health", s.handleGetWorkerHealth)
	r.Get("/install.sh", s.handleInstallScript)
}

func (s *Server) registerSystemV1Routes(r chi.Router) {
	r.Get("/info", s.handleInfo)
	r.Get("/system/mode", s.handleGetSystemMode)
	r.Get("/system/update", s.handleGetSystemUpdate)
	r.Post("/system/update/check", s.handleCheckSystemUpdate)
	r.Patch("/system/update/settings", s.handleUpdateSystemUpdateSettings)
	r.Get("/system/scheduler/jobs", s.handleListScheduledJobs)
	r.Post("/system/scheduler/jobs/{id}/trigger", s.handleTriggerScheduledJob)
	r.Get("/system/scheduler/tasks", s.handleListScheduledTasks)
	r.Post("/system/scheduler/tasks", s.handleCreateScheduledTask)
	r.Delete("/system/scheduler/tasks/{id}", s.handleDeleteScheduledTask)
	r.Patch("/system/scheduler/tasks/{id}/toggle", s.handleToggleScheduledTask)
	r.Post("/system/scheduler/tasks/{id}/trigger", s.handleTriggerScheduledTask)
	r.Get("/system/worker-health", s.handleGetWorkerHealth)
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
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") || strings.HasPrefix(path, "/uploads") || path == "/chat/widget.js" || path == "/install.sh" {
				http.NotFound(w, r)
				return
			}

			filePath := filepath.Join(staticDir, path)
			if f, err := os.Stat(filePath); err == nil && !f.IsDir() {
				w.Header().Set("X-Content-Type-Options", "nosniff")
				if strings.HasPrefix(path, "/assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else if path == "/sw.js" {
					w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Set("Service-Worker-Allowed", "/")
				} else if path == "/manifest.webmanifest" {
					w.Header().Set("Content-Type", "application/manifest+json")
					w.Header().Set("Cache-Control", "public, max-age=86400")
				}
				fileServer.ServeHTTP(w, r)
				return
			}

			if strings.HasPrefix(path, "/assets/") {
				w.Header().Set("X-Content-Type-Options", "nosniff")
				if retained := retainedAssetPath(staticDir, path); retained != "" {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					http.ServeFile(w, r, retained)
				} else {
					w.Header().Set("Cache-Control", "no-store")
					http.NotFound(w, r)
				}
				return
			}

			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		})
	} else {
		s.router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(r.URL.Path)
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") || strings.HasPrefix(path, "/uploads") || path == "/install.sh" {
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
	version := "0.1.0"
	commitSHA := ""
	if s.cfg != nil {
		if s.cfg.AppVersion != "" {
			version = s.cfg.AppVersion
		}
		commitSHA = s.cfg.CommitSHA
	}
	resp := map[string]interface{}{
		"status":     "healthy",
		"app":        "cuan",
		"version":    version,
		"commit_sha": commitSHA,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	candidates := []string{
		filepath.Join(".", "scripts", "install.sh"),
		filepath.Join("..", "scripts", "install.sh"),
		filepath.Join("..", "..", "scripts", "install.sh"),
		"/opt/cuan/scripts/install.sh",
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}
	}
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("#!/usr/bin/env bash\necho 'Cuan installer script temporarily unavailable. Please visit https://github.com/buchin/cuan'\nexit 1\n"))
}

func (s *Server) handleGetSystemMode(w http.ResponseWriter, r *http.Request) {
	mode := "marketing"
	version := "0.1.0"
	baseDomain := ""

	if s.cfg != nil {
		if s.cfg.AppMode != "" {
			mode = s.cfg.AppMode
		}
		if s.cfg.AppVersion != "" {
			version = s.cfg.AppVersion
		}
		if s.cfg.BaseDomain != "" {
			baseDomain = s.cfg.BaseDomain
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"app_mode":    mode,
		"version":     version,
		"base_domain": baseDomain,
	})
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
	version := "0.1.0"
	commitSHA := ""
	repository := "https://github.com/buchin/cuan"
	if s.cfg != nil {
		if s.cfg.AppVersion != "" {
			version = s.cfg.AppVersion
		}
		commitSHA = s.cfg.CommitSHA
		if source := s.cfg.SourceRepositoryURL(); source != "" {
			repository = source
		}
	}
	resp := map[string]interface{}{
		"name":          "Cuan",
		"version":       version,
		"commit_sha":    commitSHA,
		"description":   "Agentic project management for self-hosted teams",
		"license":       "Apache-2.0",
		"repository":    repository,
		"documentation": "https://github.com/buchin/cuan#readme",
		"release_stage": "release-candidate",
		"author":        "Buchin <buchin@dropsugar.com>",
		"sprint":        "S1",
		"status":        "running",
		"adapter":       "agy",
		"architecture":  "Go backend + React Vite TS frontend",
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetSystemUpdate(w http.ResponseWriter, r *http.Request) {
	if s.updater == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "update service is not available")
		return
	}
	respondJSON(w, http.StatusOK, s.updater.GetInfo())
}

func (s *Server) handleCheckSystemUpdate(w http.ResponseWriter, r *http.Request) {
	if s.updater == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "update service is not available")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	info, err := s.updater.CheckForUpdate(ctx)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"update":  info,
			"warning": err.Error(),
		})
		return
	}
	respondJSON(w, http.StatusOK, info)
}

func (s *Server) handleUpdateSystemUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if s.updater == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "update service is not available")
		return
	}
	var req updater.SettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	info, err := s.updater.UpdateSettings(req)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update scheduler job parameters if changed
	if s.scheduler != nil {
		if job, ok := s.scheduler.GetJob("system_update_check"); ok {
			job.Enabled = info.AutoUpdateEnabled
			job.TimeOfDay = info.AutoUpdateTime
			_ = s.scheduler.Register(job)
		}
	}

	respondJSON(w, http.StatusOK, info)
}

func (s *Server) handleListScheduledJobs(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"jobs": []interface{}{},
		})
		return
	}
	jobs := s.scheduler.ListJobs()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs": jobs,
	})
}

func (s *Server) handleTriggerScheduledJob(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "scheduler is not running")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "job id is required")
		return
	}

	go func() {
		_ = s.scheduler.Trigger(context.Background(), id)
	}()

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "triggered",
		"job_id":  id,
		"message": "Job execution started in background",
	})
}

func (s *Server) handleListScheduledTasks(w http.ResponseWriter, r *http.Request) {
	if s.taskManager == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"tasks": []interface{}{}})
		return
	}
	tasks := s.taskManager.ListTasks()
	respondJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks})
}

func (s *Server) handleCreateScheduledTask(w http.ResponseWriter, r *http.Request) {
	if s.taskManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "task manager not available")
		return
	}
	var task scheduler.ScheduledTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := s.taskManager.CreateTask(&task)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

func (s *Server) handleDeleteScheduledTask(w http.ResponseWriter, r *http.Request) {
	if s.taskManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "task manager not available")
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.taskManager.DeleteTask(id); err != nil {
		respondJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "id": id})
}

func (s *Server) handleToggleScheduledTask(w http.ResponseWriter, r *http.Request) {
	if s.taskManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "task manager not available")
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	toggled, err := s.taskManager.ToggleTask(id, req.Enabled)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, toggled)
}

func (s *Server) handleTriggerScheduledTask(w http.ResponseWriter, r *http.Request) {
	if s.taskManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "task manager not available")
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.taskManager.TriggerTask(context.Background(), id); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"status": "triggered",
		"id":     id,
	})
}

// WorkerHealthResponse represents health metrics and queue depth of cuan-runner daemon.
type WorkerHealthResponse struct {
	Status             string    `json:"status"`      // "ready" (green), "busy" (yellow), "attention" (red)
	RunnerID           string    `json:"runner_id"`   // "cuan-runner" or matched runner ID
	RunnerName         string    `json:"runner_name"` // Human-readable runner name
	Hostname           string    `json:"hostname,omitempty"`
	IsAlive            bool      `json:"is_alive"` // true if heartbeat received within threshold (45s)
	LastHeartbeatAt    time.Time `json:"last_heartbeat_at"`
	LastSeenSecondsAgo int64     `json:"last_seen_seconds_ago"`
	QueuedCount        int       `json:"queued_count"`  // Tasks with status queued or pending
	RunningCount       int       `json:"running_count"` // Tasks currently doing / running
	QueueDepth         int       `json:"queue_depth"`   // queued_count + running_count
	Timestamp          time.Time `json:"timestamp"`
}

func (s *Server) handleGetWorkerHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now().UTC()

	var queuedCount, runningCount int
	if s.taskSvc != nil {
		tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
		if err == nil {
			for _, t := range tasks {
				st := strings.ToLower(strings.TrimSpace(t.Status))
				if st == task.TaskStatusQueued || st == task.TaskStatusPending {
					queuedCount++
				} else if st == task.TaskStatusDoing {
					runningCount++
				} else if t.LatestAttempt != nil {
					attSt := strings.ToLower(strings.TrimSpace(t.LatestAttempt.Status))
					if attSt == task.AttemptStatusRunning {
						runningCount++
					}
				}
			}
		}
	}

	var matchedRunner *runner.RunnerInfo
	if s.runnerSvc != nil {
		runners, err := s.runnerSvc.ListRunners(ctx)
		if err == nil && len(runners) > 0 {
			// Prioritize runner matching "cuan-runner"
			for _, rn := range runners {
				if rn.ID == "cuan-runner" || rn.Name == "cuan-runner" {
					matchedRunner = rn
					break
				}
			}
			// If not found, pick the runner with the latest heartbeat
			if matchedRunner == nil {
				matchedRunner = runners[0]
				for _, rn := range runners[1:] {
					if rn.LastHeartbeatAt.After(matchedRunner.LastHeartbeatAt) {
						matchedRunner = rn
					}
				}
			}
		}
		if matchedRunner == nil {
			if rn, err := s.runnerSvc.GetRunner(ctx, "cuan-runner"); err == nil && rn != nil {
				matchedRunner = rn
			}
		}
	}

	resp := WorkerHealthResponse{
		Status:             "attention",
		RunnerID:           "cuan-runner",
		RunnerName:         "cuan-runner",
		QueuedCount:        queuedCount,
		RunningCount:       runningCount,
		QueueDepth:         queuedCount + runningCount,
		Timestamp:          now,
		LastSeenSecondsAgo: -1,
	}

	const heartbeatThreshold = 45 * time.Second

	if matchedRunner != nil {
		resp.RunnerID = matchedRunner.ID
		if matchedRunner.Name != "" {
			resp.RunnerName = matchedRunner.Name
		}
		resp.Hostname = matchedRunner.Hostname
		resp.LastHeartbeatAt = matchedRunner.LastHeartbeatAt

		if !matchedRunner.LastHeartbeatAt.IsZero() {
			diff := now.Sub(matchedRunner.LastHeartbeatAt)
			if diff >= 0 {
				resp.LastSeenSecondsAgo = int64(diff.Seconds())
			}
			if diff <= heartbeatThreshold {
				resp.IsAlive = true
			}
		}
	}

	if resp.IsAlive {
		if resp.RunningCount > 0 || (matchedRunner != nil && matchedRunner.Status == runner.StatusBusy) {
			resp.Status = "busy"
		} else {
			resp.Status = "ready"
		}
	} else {
		resp.Status = "attention"
	}

	respondJSON(w, http.StatusOK, resp)
}

package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/observability"
)

// DeploymentHealthResponse represents the consolidated health and uptime report for a deployment.
type DeploymentHealthResponse struct {
	// Process health fields (for backward compatibility with Sprint 3 & 4 tests)
	Alive          bool   `json:"alive"`
	PID            int    `json:"pid"`
	Port           int    `json:"port"`
	PortResponding bool   `json:"port_responding"`
	Error          string `json:"error,omitempty"`

	// Observability metrics & history (Sprint 6 S06.2)
	DeploymentID        string                      `json:"deployment_id"`
	URL                 string                      `json:"url"`
	Status              string                      `json:"status"` // 'healthy', 'degraded', 'unhealthy'
	LastCheckedAt       *time.Time                  `json:"last_checked_at,omitempty"`
	LastStatusCode      int                         `json:"last_status_code"`
	ResponseTimeMs      int64                       `json:"response_time_ms"`
	ConsecutiveFailures int                         `json:"consecutive_failures"`
	UptimePercent       float64                     `json:"uptime_percent"`
	AverageLatencyMs    float64                     `json:"avg_latency_ms"`
	TotalProbes         int                         `json:"total_probes"`
	History             []observability.ProbeRecord `json:"history"`
}

// handleGetDeploymentHealth retrieves the current health status, latency, and uptime history.
func (s *Server) handleGetDeploymentHealth(w http.ResponseWriter, r *http.Request) {
	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	dep, err := s.deployerDriver.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, deployer.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get deployment: "+err.Error())
		return
	}

	// 1. Check process liveness via deployer driver
	hs, err := s.deployerDriver.Health(r.Context(), id)
	if err != nil {
		hs = &deployer.HealthStatus{
			Alive: false,
			Error: err.Error(),
		}
	}

	// 2. Lookup or ensure health check in observability repository
	var hc *cuanDB.HealthCheck
	if s.obsRepo != nil {
		hc, _ = s.obsRepo.GetHealthCheckByDeploymentID(r.Context(), id)
		if hc == nil {
			targetURL := dep.Domain
			if targetURL != "" && !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
				targetURL = "http://" + targetURL
			}
			if targetURL == "" && dep.InternalPort > 0 {
				targetURL = fmt.Sprintf("http://127.0.0.1:%d", dep.InternalPort)
			}
			if targetURL == "" {
				targetURL = "http://127.0.0.1:8080"
			}
			if s.obsHealthPoller != nil {
				hc, _ = s.obsHealthPoller.EnsureHealthCheck(r.Context(), id, targetURL)
			}
		}
	}

	// 3. Compute uptime statistics from rolling history
	var uptimeRep observability.UptimeReport
	if s.obsHealthPoller != nil {
		uptimeRep = s.obsHealthPoller.GetUptimeReport(id)
	}

	// Determine status
	status := observability.StatusHealthy
	if dep.Status != "running" {
		status = observability.StatusUnhealthy
	} else if hc != nil {
		status = hc.Status
	} else if !hs.Alive {
		status = observability.StatusUnhealthy
	}

	url := ""
	var lastChecked *time.Time
	var statusCode int
	var latency int64
	var consecutiveFailures int

	if hc != nil {
		url = hc.URL
		lastChecked = hc.LastCheckedAt
		statusCode = hc.LastStatusCode
		latency = hc.ResponseTimeMs
		consecutiveFailures = hc.ConsecutiveFailures
	}

	history := uptimeRep.RecentProbes
	if history == nil {
		history = []observability.ProbeRecord{}
	}

	resp := DeploymentHealthResponse{
		Alive:               hs.Alive,
		PID:                 hs.PID,
		Port:                hs.Port,
		PortResponding:      hs.PortResponding,
		Error:               hs.Error,
		DeploymentID:        dep.ID,
		URL:                 url,
		Status:              status,
		LastCheckedAt:       lastChecked,
		LastStatusCode:      statusCode,
		ResponseTimeMs:      latency,
		ConsecutiveFailures: consecutiveFailures,
		UptimePercent:       uptimeRep.UptimePercent,
		AverageLatencyMs:    uptimeRep.AverageLatencyMs,
		TotalProbes:         uptimeRep.TotalProbes,
		History:             history,
	}

	respondJSON(w, http.StatusOK, resp)
}

// handleCheckDeploymentHealthNow executes an instant on-demand health check probe.
func (s *Server) handleCheckDeploymentHealthNow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	if s.deployerDriver == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "deployer driver is not configured")
		return
	}

	dep, err := s.deployerDriver.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, deployer.ErrDeploymentNotFound) {
			respondJSONError(w, http.StatusNotFound, "deployment not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get deployment: "+err.Error())
		return
	}

	if s.obsHealthPoller == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "observability health poller not configured")
		return
	}

	targetURL := dep.Domain
	if targetURL != "" && !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}
	if targetURL == "" && dep.InternalPort > 0 {
		targetURL = fmt.Sprintf("http://127.0.0.1:%d", dep.InternalPort)
	}
	if targetURL == "" {
		targetURL = "http://127.0.0.1:8080"
	}

	// Ensure health check is registered
	_, err = s.obsHealthPoller.EnsureHealthCheck(r.Context(), id, targetURL)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "ensure health check failed: "+err.Error())
		return
	}

	updated, report, err := s.obsHealthPoller.ProbeNow(r.Context(), id)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "probe failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":              true,
		"deployment_id":        updated.DeploymentID,
		"url":                  updated.URL,
		"status":               updated.Status,
		"last_status_code":     updated.LastStatusCode,
		"response_time_ms":     updated.ResponseTimeMs,
		"consecutive_failures": updated.ConsecutiveFailures,
		"last_checked_at":      updated.LastCheckedAt,
		"uptime_percent":       report.UptimePercent,
		"avg_latency_ms":       report.AverageLatencyMs,
		"total_probes":         report.TotalProbes,
	})
}

// handleGetServerMetrics returns CPU, RAM, and Disk telemetry for a server.
func (s *Server) handleGetServerMetrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "server id is required")
		return
	}

	if s.serverRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "server repository not configured")
		return
	}

	_, err := s.serverRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, cuanDB.ErrServerNotFound) {
			respondJSONError(w, http.StatusNotFound, "server not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "get server failed: "+err.Error())
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
			if limit > 200 {
				limit = 200
			}
		}
	}

	// Optional on-demand collect trigger
	if r.URL.Query().Get("collect") == "true" && s.obsMetricsCollector != nil {
		_, _ = s.obsMetricsCollector.CollectServer(r.Context(), id)
	}

	var latestDTO *cuanDB.ServerMetricDTO
	if s.obsMetricsCollector != nil {
		latestDTO, _ = s.obsMetricsCollector.GetLatestServerMetric(r.Context(), id)
	}

	var metricDTOs []cuanDB.ServerMetricDTO
	if s.obsMetricsCollector != nil {
		metricDTOs, _ = s.obsMetricsCollector.ListServerMetrics(r.Context(), id, limit)
	}
	if metricDTOs == nil {
		metricDTOs = []cuanDB.ServerMetricDTO{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"server_id": id,
		"latest":    latestDTO,
		"metrics":   metricDTOs,
		"count":     len(metricDTOs),
	})
}

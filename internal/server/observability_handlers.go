package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/crypto"
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

// ----------------- Alert Channels & Incidents Handlers (Sprint 6 S06.3) -----------------

// CreateAlertChannelPayload defines the JSON body for registering an alert channel.
type CreateAlertChannelPayload struct {
	Name            string `json:"name"`
	Type            string `json:"type"` // 'telegram', 'discord', 'webhook'
	IsActive        *bool  `json:"is_active,omitempty"`
	BotToken        string `json:"bot_token,omitempty"`
	ChatID          string `json:"chat_id,omitempty"`
	TelegramBaseURL string `json:"telegram_base_url,omitempty"`
	WebhookURL      string `json:"webhook_url,omitempty"`
	URL             string `json:"url,omitempty"`
	Secret          string `json:"secret,omitempty"`
}

// handleCreateAlertChannel registers a new alert notification channel for a project,
// encrypting secrets at-rest via KeyVault AES-GCM.
func (s *Server) handleCreateAlertChannel(w http.ResponseWriter, r *http.Request) {
	if s.obsRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "observability repository is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var payload CreateAlertChannelPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		respondJSONError(w, http.StatusBadRequest, "channel name is required")
		return
	}

	channelType := strings.ToLower(strings.TrimSpace(payload.Type))
	if err := cuanDB.ValidateChannelType(channelType); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	webhookURL := strings.TrimSpace(payload.WebhookURL)
	if webhookURL == "" && strings.TrimSpace(payload.URL) != "" {
		webhookURL = strings.TrimSpace(payload.URL)
	}

	cfg := &observability.ChannelConfig{
		BotToken:        strings.TrimSpace(payload.BotToken),
		ChatID:          strings.TrimSpace(payload.ChatID),
		TelegramBaseURL: strings.TrimSpace(payload.TelegramBaseURL),
		WebhookURL:      webhookURL,
		Secret:          strings.TrimSpace(payload.Secret),
	}

	if err := observability.ValidateChannelConfig(channelType, cfg); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	vault := s.keyVault
	if vault == nil {
		vault = crypto.DefaultKeyVault()
	}

	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to serialize channel config: "+err.Error())
		return
	}

	encryptedB64, err := vault.Encrypt(cfgBytes)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to encrypt credentials: "+err.Error())
		return
	}

	isActive := true
	if payload.IsActive != nil {
		isActive = *payload.IsActive
	}

	channel := &cuanDB.AlertChannel{
		ProjectID:       projectID,
		Type:            channelType,
		Name:            name,
		ConfigEncrypted: encryptedB64,
		IsActive:        isActive,
	}

	if err := s.obsRepo.CreateAlertChannel(r.Context(), channel); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to persist alert channel: "+err.Error())
		return
	}

	dto := channel.ToDTO()
	dto.MaskedConfig = observability.MaskChannelConfig(channelType, cfg)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"channel":       dto,
		"id":            dto.ID,
		"project_id":    dto.ProjectID,
		"type":          dto.Type,
		"name":          dto.Name,
		"is_active":     dto.IsActive,
		"masked_config": dto.MaskedConfig,
		"created_at":    dto.CreatedAt,
		"updated_at":    dto.UpdatedAt,
	})
}

// handleListAlertChannels retrieves sanitized alert channels for a project with masked credentials.
func (s *Server) handleListAlertChannels(w http.ResponseWriter, r *http.Request) {
	if s.obsRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "observability repository is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	channels, err := s.obsRepo.ListAlertChannelsByProjectID(r.Context(), projectID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to query alert channels: "+err.Error())
		return
	}

	vault := s.keyVault
	if vault == nil {
		vault = crypto.DefaultKeyVault()
	}

	dtos := make([]cuanDB.AlertChannelDTO, 0, len(channels))
	for _, ch := range channels {
		dto := ch.ToDTO()
		if ch.ConfigEncrypted != "" {
			if plainBytes, err := vault.Decrypt(ch.ConfigEncrypted); err == nil {
				var cfg observability.ChannelConfig
				if err := json.Unmarshal(plainBytes, &cfg); err == nil {
					dto.MaskedConfig = observability.MaskChannelConfig(ch.Type, &cfg)
				}
			}
		}
		dtos = append(dtos, dto)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"channels": dtos,
		"count":    len(dtos),
	})
}

// handleTestAlertChannel dispatches a 1-click test alert to verify integration credentials.
func (s *Server) handleTestAlertChannel(w http.ResponseWriter, r *http.Request) {
	if s.obsRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "observability repository is not configured")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "alert channel id is required")
		return
	}

	channel, err := s.obsRepo.GetAlertChannelByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, cuanDB.ErrAlertChannelNotFound) {
			respondJSONError(w, http.StatusNotFound, "alert channel not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get alert channel: "+err.Error())
		return
	}

	dispatcher := s.obsAlertDispatcher
	if dispatcher == nil {
		dispatcher = observability.NewAlertDispatcher(
			s.obsRepo,
			observability.WithAlertKeyVault(s.keyVault),
		)
	}

	if err := dispatcher.TestChannel(r.Context(), channel); err != nil {
		respondJSONError(w, http.StatusBadGateway, "failed to send test alert: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"channel_id": channel.ID,
		"type":       channel.Type,
		"name":       channel.Name,
		"message":    "test alert sent successfully",
	})
}

// handleDeleteAlertChannel removes a configured alert channel by ID.
func (s *Server) handleDeleteAlertChannel(w http.ResponseWriter, r *http.Request) {
	if s.obsRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "observability repository is not configured")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "alert channel id is required")
		return
	}

	if err := s.obsRepo.DeleteAlertChannel(r.Context(), id); err != nil {
		if errors.Is(err, cuanDB.ErrAlertChannelNotFound) {
			respondJSONError(w, http.StatusNotFound, "alert channel not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete alert channel: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "alert channel deleted successfully",
	})
}

// handleListIncidents retrieves the history of alert incidents and resolutions.
func (s *Server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	if s.obsRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "observability repository is not configured")
		return
	}

	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	targetID := strings.TrimSpace(r.URL.Query().Get("target_id"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
			if limit > 200 {
				limit = 200
			}
		}
	}

	var incidents []cuanDB.AlertIncident
	var err error
	if projectID != "" {
		incidents, err = s.obsRepo.ListAlertIncidentsByProjectID(r.Context(), projectID, limit)
	} else {
		incidents, err = s.obsRepo.ListAlertIncidents(r.Context(), limit)
	}

	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to query alert incidents: "+err.Error())
		return
	}

	dtos := make([]cuanDB.AlertIncidentDTO, 0, len(incidents))
	for _, inc := range incidents {
		if targetID != "" && inc.TargetID != targetID {
			continue
		}
		if status != "" && inc.Status != status {
			continue
		}
		dtos = append(dtos, inc.ToDTO())
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"incidents": dtos,
		"count":     len(dtos),
	})
}

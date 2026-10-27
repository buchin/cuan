package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrHealthCheckNotFound   = errors.New("health check not found")
	ErrDuplicateHealthCheck  = errors.New("health check already exists for deployment")
	ErrInvalidHealthCheckURL = errors.New("invalid health check url format")

	ErrAlertChannelNotFound = errors.New("alert channel not found")
	ErrInvalidChannelType   = errors.New("invalid alert channel type (must be telegram, discord, or webhook)")
	ErrEmptyChannelName     = errors.New("alert channel name cannot be empty")
	ErrEmptyProjectID       = errors.New("project id cannot be empty")

	ErrAlertIncidentNotFound = errors.New("alert incident not found")
)

// generateObservabilityID produces a random identifier prefixed with the specified domain prefix.
func generateObservabilityID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

// ValidateHealthCheckURL verifies that a target URL is non-empty and has a valid HTTP/HTTPS scheme.
func ValidateHealthCheckURL(targetURL string) error {
	trimmed := strings.TrimSpace(targetURL)
	if trimmed == "" {
		return fmt.Errorf("%w: url cannot be empty", ErrInvalidHealthCheckURL)
	}
	u, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return fmt.Errorf("%w: parse error: %v", ErrInvalidHealthCheckURL, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https, got %s", ErrInvalidHealthCheckURL, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: url host cannot be empty", ErrInvalidHealthCheckURL)
	}
	return nil
}

// ValidateChannelType checks if the channel type matches supported notification dispatchers.
func ValidateChannelType(channelType string) error {
	normalized := strings.ToLower(strings.TrimSpace(channelType))
	switch normalized {
	case "telegram", "discord", "webhook":
		return nil
	default:
		return fmt.Errorf("%w: got '%s'", ErrInvalidChannelType, channelType)
	}
}

// HealthCheck represents an active HTTP/HTTPS endpoint probe for a deployment.
type HealthCheck struct {
	ID                  string     `json:"id"`
	DeploymentID        string     `json:"deployment_id"`
	URL                 string     `json:"url"`
	Status              string     `json:"status"` // 'healthy', 'degraded', 'unhealthy'
	LastCheckedAt       *time.Time `json:"last_checked_at,omitempty"`
	LastStatusCode      int        `json:"last_status_code"`
	ResponseTimeMs      int64      `json:"response_time_ms"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// HealthCheckDTO is a public Data Transfer Object for health check responses.
type HealthCheckDTO struct {
	ID                  string     `json:"id"`
	DeploymentID        string     `json:"deployment_id"`
	URL                 string     `json:"url"`
	Status              string     `json:"status"`
	LastCheckedAt       *time.Time `json:"last_checked_at,omitempty"`
	LastStatusCode      int        `json:"last_status_code"`
	ResponseTimeMs      int64      `json:"response_time_ms"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ToDTO converts HealthCheck to HealthCheckDTO.
func (hc *HealthCheck) ToDTO() HealthCheckDTO {
	return HealthCheckDTO{
		ID:                  hc.ID,
		DeploymentID:        hc.DeploymentID,
		URL:                 hc.URL,
		Status:              hc.Status,
		LastCheckedAt:       hc.LastCheckedAt,
		LastStatusCode:      hc.LastStatusCode,
		ResponseTimeMs:      hc.ResponseTimeMs,
		ConsecutiveFailures: hc.ConsecutiveFailures,
		CreatedAt:           hc.CreatedAt,
		UpdatedAt:           hc.UpdatedAt,
	}
}

// ServerMetric represents hardware/runtime resource telemetry for a server or deployment.
type ServerMetric struct {
	ID           string    `json:"id"`
	ServerID     *string   `json:"server_id,omitempty"`
	DeploymentID *string   `json:"deployment_id,omitempty"`
	CPUPercent   float64   `json:"cpu_percent"`
	MemoryUsedMB int64     `json:"memory_used_mb"`
	MemoryTotalMB int64    `json:"memory_total_mb"`
	DiskUsedGB   float64   `json:"disk_used_gb"`
	DiskTotalGB  float64   `json:"disk_total_gb"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// ServerMetricDTO is a sanitized Data Transfer Object with computed percentage fields.
type ServerMetricDTO struct {
	ID            string    `json:"id"`
	ServerID      string    `json:"server_id,omitempty"`
	DeploymentID  string    `json:"deployment_id,omitempty"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsedMB  int64     `json:"memory_used_mb"`
	MemoryTotalMB int64     `json:"memory_total_mb"`
	MemoryPercent float64   `json:"memory_percent"`
	DiskUsedGB    float64   `json:"disk_used_gb"`
	DiskTotalGB   float64   `json:"disk_total_gb"`
	DiskPercent   float64   `json:"disk_percent"`
	RecordedAt    time.Time `json:"recorded_at"`
}

// ToDTO converts ServerMetric to ServerMetricDTO with calculated percentages.
func (sm *ServerMetric) ToDTO() ServerMetricDTO {
	var srvID, depID string
	if sm.ServerID != nil {
		srvID = *sm.ServerID
	}
	if sm.DeploymentID != nil {
		depID = *sm.DeploymentID
	}

	var memPct float64
	if sm.MemoryTotalMB > 0 {
		memPct = float64(sm.MemoryUsedMB) / float64(sm.MemoryTotalMB) * 100.0
	}

	var diskPct float64
	if sm.DiskTotalGB > 0 {
		diskPct = sm.DiskUsedGB / sm.DiskTotalGB * 100.0
	}

	return ServerMetricDTO{
		ID:            sm.ID,
		ServerID:      srvID,
		DeploymentID:  depID,
		CPUPercent:    sm.CPUPercent,
		MemoryUsedMB:  sm.MemoryUsedMB,
		MemoryTotalMB: sm.MemoryTotalMB,
		MemoryPercent: memPct,
		DiskUsedGB:    sm.DiskUsedGB,
		DiskTotalGB:   sm.DiskTotalGB,
		DiskPercent:   diskPct,
		RecordedAt:    sm.RecordedAt,
	}
}

// AlertChannel represents a configured multi-channel notification destination.
type AlertChannel struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	Type            string    `json:"type"` // 'telegram', 'discord', 'webhook'
	Name            string    `json:"name"`
	ConfigEncrypted string    `json:"-"` // Explicitly masked from JSON serialization for zero credential leaks
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AlertChannelDTO is a safe Data Transfer Object without encrypted secrets.
type AlertChannelDTO struct {
	ID           string            `json:"id"`
	ProjectID    string            `json:"project_id"`
	Type         string            `json:"type"`
	Name         string            `json:"name"`
	IsActive     bool              `json:"is_active"`
	MaskedConfig map[string]string `json:"masked_config,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ToDTO converts AlertChannel to safe AlertChannelDTO.
func (ac *AlertChannel) ToDTO() AlertChannelDTO {
	return AlertChannelDTO{
		ID:        ac.ID,
		ProjectID: ac.ProjectID,
		Type:      ac.Type,
		Name:      ac.Name,
		IsActive:  ac.IsActive,
		CreatedAt: ac.CreatedAt,
		UpdatedAt: ac.UpdatedAt,
	}
}

// ToSanitizedDTO aliases ToDTO ensuring zero credential leakage.
func (ac *AlertChannel) ToSanitizedDTO() AlertChannelDTO {
	return ac.ToDTO()
}

// AlertIncident represents an active or resolved incident outage/degradation event.
type AlertIncident struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	TargetType  string     `json:"target_type"` // 'deployment', 'server'
	TargetID    string     `json:"target_id"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	Status      string     `json:"status"` // 'firing', 'resolved'
	TriggeredAt time.Time  `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// AlertIncidentDTO is a Data Transfer Object for public incident responses.
type AlertIncidentDTO struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	TargetType  string     `json:"target_type"`
	TargetID    string     `json:"target_id"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	Status      string     `json:"status"`
	TriggeredAt time.Time  `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// ToDTO converts AlertIncident to AlertIncidentDTO.
func (ai *AlertIncident) ToDTO() AlertIncidentDTO {
	return AlertIncidentDTO{
		ID:          ai.ID,
		ProjectID:   ai.ProjectID,
		TargetType:  ai.TargetType,
		TargetID:    ai.TargetID,
		Title:       ai.Title,
		Message:     ai.Message,
		Status:      ai.Status,
		TriggeredAt: ai.TriggeredAt,
		ResolvedAt:  ai.ResolvedAt,
	}
}

// ObservabilityRepository defines persistence contracts for health checks, metrics, alert channels, and incidents.
type ObservabilityRepository interface {
	// Health Checks
	CreateHealthCheck(ctx context.Context, hc *HealthCheck) error
	GetHealthCheckByID(ctx context.Context, id string) (*HealthCheck, error)
	GetHealthCheckByDeploymentID(ctx context.Context, deploymentID string) (*HealthCheck, error)
	ListHealthChecks(ctx context.Context) ([]HealthCheck, error)
	UpdateHealthCheck(ctx context.Context, hc *HealthCheck) error
	UpdateHealthCheckResult(ctx context.Context, id string, status string, statusCode int, responseTimeMs int64, consecutiveFailures int, checkedAt time.Time) error
	DeleteHealthCheck(ctx context.Context, id string) error

	// Server & Deployment Metrics
	RecordServerMetric(ctx context.Context, metric *ServerMetric) error
	ListServerMetrics(ctx context.Context, serverID string, limit int) ([]ServerMetric, error)
	ListDeploymentMetrics(ctx context.Context, deploymentID string, limit int) ([]ServerMetric, error)
	GetLatestServerMetric(ctx context.Context, serverID string) (*ServerMetric, error)

	// Alert Channels
	CreateAlertChannel(ctx context.Context, channel *AlertChannel) error
	GetAlertChannelByID(ctx context.Context, id string) (*AlertChannel, error)
	ListAlertChannelsByProjectID(ctx context.Context, projectID string) ([]AlertChannel, error)
	UpdateAlertChannel(ctx context.Context, channel *AlertChannel) error
	DeleteAlertChannel(ctx context.Context, id string) error

	// Alert Incidents
	CreateAlertIncident(ctx context.Context, incident *AlertIncident) error
	GetAlertIncidentByID(ctx context.Context, id string) (*AlertIncident, error)
	ListAlertIncidents(ctx context.Context, limit int) ([]AlertIncident, error)
	ListAlertIncidentsByProjectID(ctx context.Context, projectID string, limit int) ([]AlertIncident, error)
	ListActiveAlertIncidents(ctx context.Context, projectID string) ([]AlertIncident, error)
	ResolveAlertIncident(ctx context.Context, id string, resolvedAt time.Time) error
}

// SQLObservabilityRepository implements ObservabilityRepository backed by PostgreSQL.
type SQLObservabilityRepository struct {
	db *sql.DB
}

// NewSQLObservabilityRepository creates an instance of SQLObservabilityRepository.
func NewSQLObservabilityRepository(db *sql.DB) *SQLObservabilityRepository {
	return &SQLObservabilityRepository{db: db}
}

// ----------------- Health Checks (SQL) -----------------

func (r *SQLObservabilityRepository) CreateHealthCheck(ctx context.Context, hc *HealthCheck) error {
	if hc == nil {
		return errors.New("health check cannot be nil")
	}
	if strings.TrimSpace(hc.DeploymentID) == "" {
		return ErrEmptyDeploymentID
	}
	if err := ValidateHealthCheckURL(hc.URL); err != nil {
		return err
	}

	if strings.TrimSpace(hc.ID) == "" {
		hc.ID = generateObservabilityID("hc")
	}
	if strings.TrimSpace(hc.Status) == "" {
		hc.Status = "healthy"
	}
	now := time.Now().UTC()
	if hc.CreatedAt.IsZero() {
		hc.CreatedAt = now
	}
	hc.UpdatedAt = now

	query := `
		INSERT INTO health_checks (
			id, deployment_id, url, status, last_checked_at,
			last_status_code, response_time_ms, consecutive_failures, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		hc.ID,
		hc.DeploymentID,
		hc.URL,
		hc.Status,
		hc.LastCheckedAt,
		hc.LastStatusCode,
		hc.ResponseTimeMs,
		hc.ConsecutiveFailures,
		hc.CreatedAt,
		hc.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(err.Error(), "23505") {
			return ErrDuplicateHealthCheck
		}
		return fmt.Errorf("create health check: %w", err)
	}
	return nil
}

func (r *SQLObservabilityRepository) GetHealthCheckByID(ctx context.Context, id string) (*HealthCheck, error) {
	query := `
		SELECT id, deployment_id, url, status, last_checked_at,
		       last_status_code, response_time_ms, consecutive_failures, created_at, updated_at
		FROM health_checks
		WHERE id = $1
	`
	var hc HealthCheck
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&hc.ID,
		&hc.DeploymentID,
		&hc.URL,
		&hc.Status,
		&hc.LastCheckedAt,
		&hc.LastStatusCode,
		&hc.ResponseTimeMs,
		&hc.ConsecutiveFailures,
		&hc.CreatedAt,
		&hc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHealthCheckNotFound
		}
		return nil, fmt.Errorf("get health check by id: %w", err)
	}
	return &hc, nil
}

func (r *SQLObservabilityRepository) GetHealthCheckByDeploymentID(ctx context.Context, deploymentID string) (*HealthCheck, error) {
	query := `
		SELECT id, deployment_id, url, status, last_checked_at,
		       last_status_code, response_time_ms, consecutive_failures, created_at, updated_at
		FROM health_checks
		WHERE deployment_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var hc HealthCheck
	row := r.db.QueryRowContext(ctx, query, deploymentID)
	err := row.Scan(
		&hc.ID,
		&hc.DeploymentID,
		&hc.URL,
		&hc.Status,
		&hc.LastCheckedAt,
		&hc.LastStatusCode,
		&hc.ResponseTimeMs,
		&hc.ConsecutiveFailures,
		&hc.CreatedAt,
		&hc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrHealthCheckNotFound
		}
		return nil, fmt.Errorf("get health check by deployment id: %w", err)
	}
	return &hc, nil
}

func (r *SQLObservabilityRepository) ListHealthChecks(ctx context.Context) ([]HealthCheck, error) {
	query := `
		SELECT id, deployment_id, url, status, last_checked_at,
		       last_status_code, response_time_ms, consecutive_failures, created_at, updated_at
		FROM health_checks
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list health checks: %w", err)
	}
	defer rows.Close()

	var list []HealthCheck
	for rows.Next() {
		var hc HealthCheck
		if err := rows.Scan(
			&hc.ID,
			&hc.DeploymentID,
			&hc.URL,
			&hc.Status,
			&hc.LastCheckedAt,
			&hc.LastStatusCode,
			&hc.ResponseTimeMs,
			&hc.ConsecutiveFailures,
			&hc.CreatedAt,
			&hc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan health check: %w", err)
		}
		list = append(list, hc)
	}
	return list, nil
}

func (r *SQLObservabilityRepository) UpdateHealthCheck(ctx context.Context, hc *HealthCheck) error {
	if hc == nil {
		return errors.New("health check cannot be nil")
	}
	if err := ValidateHealthCheckURL(hc.URL); err != nil {
		return err
	}
	hc.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE health_checks
		SET url = $2, status = $3, last_checked_at = $4,
		    last_status_code = $5, response_time_ms = $6, consecutive_failures = $7, updated_at = $8
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query,
		hc.ID,
		hc.URL,
		hc.Status,
		hc.LastCheckedAt,
		hc.LastStatusCode,
		hc.ResponseTimeMs,
		hc.ConsecutiveFailures,
		hc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update health check: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update health check rows: %w", err)
	}
	if rows == 0 {
		return ErrHealthCheckNotFound
	}
	return nil
}

func (r *SQLObservabilityRepository) UpdateHealthCheckResult(ctx context.Context, id string, status string, statusCode int, responseTimeMs int64, consecutiveFailures int, checkedAt time.Time) error {
	now := time.Now().UTC()
	query := `
		UPDATE health_checks
		SET status = $2, last_status_code = $3, response_time_ms = $4,
		    consecutive_failures = $5, last_checked_at = $6, updated_at = $7
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query,
		id,
		status,
		statusCode,
		responseTimeMs,
		consecutiveFailures,
		checkedAt.UTC(),
		now,
	)
	if err != nil {
		return fmt.Errorf("update health check result: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update health check result rows: %w", err)
	}
	if rows == 0 {
		return ErrHealthCheckNotFound
	}
	return nil
}

func (r *SQLObservabilityRepository) DeleteHealthCheck(ctx context.Context, id string) error {
	query := `DELETE FROM health_checks WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete health check: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete health check rows: %w", err)
	}
	if rows == 0 {
		return ErrHealthCheckNotFound
	}
	return nil
}

// ----------------- Server & Deployment Metrics (SQL) -----------------

func (r *SQLObservabilityRepository) RecordServerMetric(ctx context.Context, metric *ServerMetric) error {
	if metric == nil {
		return errors.New("server metric cannot be nil")
	}
	if strings.TrimSpace(metric.ID) == "" {
		metric.ID = generateObservabilityID("met")
	}
	if metric.RecordedAt.IsZero() {
		metric.RecordedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO server_metrics (
			id, server_id, deployment_id, cpu_percent, memory_used_mb,
			memory_total_mb, disk_used_gb, disk_total_gb, recorded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		metric.ID,
		metric.ServerID,
		metric.DeploymentID,
		metric.CPUPercent,
		metric.MemoryUsedMB,
		metric.MemoryTotalMB,
		metric.DiskUsedGB,
		metric.DiskTotalGB,
		metric.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("record server metric: %w", err)
	}
	return nil
}

func (r *SQLObservabilityRepository) ListServerMetrics(ctx context.Context, serverID string, limit int) ([]ServerMetric, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, server_id, deployment_id, cpu_percent, memory_used_mb,
		       memory_total_mb, disk_used_gb, disk_total_gb, recorded_at
		FROM server_metrics
		WHERE server_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("list server metrics: %w", err)
	}
	defer rows.Close()

	var list []ServerMetric
	for rows.Next() {
		var sm ServerMetric
		if err := rows.Scan(
			&sm.ID,
			&sm.ServerID,
			&sm.DeploymentID,
			&sm.CPUPercent,
			&sm.MemoryUsedMB,
			&sm.MemoryTotalMB,
			&sm.DiskUsedGB,
			&sm.DiskTotalGB,
			&sm.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan server metric: %w", err)
		}
		list = append(list, sm)
	}
	return list, nil
}

func (r *SQLObservabilityRepository) ListDeploymentMetrics(ctx context.Context, deploymentID string, limit int) ([]ServerMetric, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, server_id, deployment_id, cpu_percent, memory_used_mb,
		       memory_total_mb, disk_used_gb, disk_total_gb, recorded_at
		FROM server_metrics
		WHERE deployment_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, deploymentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list deployment metrics: %w", err)
	}
	defer rows.Close()

	var list []ServerMetric
	for rows.Next() {
		var sm ServerMetric
		if err := rows.Scan(
			&sm.ID,
			&sm.ServerID,
			&sm.DeploymentID,
			&sm.CPUPercent,
			&sm.MemoryUsedMB,
			&sm.MemoryTotalMB,
			&sm.DiskUsedGB,
			&sm.DiskTotalGB,
			&sm.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan deployment metric: %w", err)
		}
		list = append(list, sm)
	}
	return list, nil
}

func (r *SQLObservabilityRepository) GetLatestServerMetric(ctx context.Context, serverID string) (*ServerMetric, error) {
	metrics, err := r.ListServerMetrics(ctx, serverID, 1)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		return nil, sql.ErrNoRows
	}
	return &metrics[0], nil
}

// ----------------- Alert Channels (SQL) -----------------

func (r *SQLObservabilityRepository) CreateAlertChannel(ctx context.Context, channel *AlertChannel) error {
	if channel == nil {
		return errors.New("alert channel cannot be nil")
	}
	if strings.TrimSpace(channel.ProjectID) == "" {
		return ErrEmptyProjectID
	}
	if strings.TrimSpace(channel.Name) == "" {
		return ErrEmptyChannelName
	}
	if err := ValidateChannelType(channel.Type); err != nil {
		return err
	}
	channel.Type = strings.ToLower(strings.TrimSpace(channel.Type))

	if strings.TrimSpace(channel.ID) == "" {
		channel.ID = generateObservabilityID("alc")
	}
	now := time.Now().UTC()
	if channel.CreatedAt.IsZero() {
		channel.CreatedAt = now
	}
	channel.UpdatedAt = now

	query := `
		INSERT INTO alert_channels (
			id, project_id, type, name, config_encrypted, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		channel.ID,
		channel.ProjectID,
		channel.Type,
		channel.Name,
		channel.ConfigEncrypted,
		channel.IsActive,
		channel.CreatedAt,
		channel.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create alert channel: %w", err)
	}
	return nil
}

func (r *SQLObservabilityRepository) GetAlertChannelByID(ctx context.Context, id string) (*AlertChannel, error) {
	query := `
		SELECT id, project_id, type, name, config_encrypted, is_active, created_at, updated_at
		FROM alert_channels
		WHERE id = $1
	`
	var ac AlertChannel
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&ac.ID,
		&ac.ProjectID,
		&ac.Type,
		&ac.Name,
		&ac.ConfigEncrypted,
		&ac.IsActive,
		&ac.CreatedAt,
		&ac.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAlertChannelNotFound
		}
		return nil, fmt.Errorf("get alert channel by id: %w", err)
	}
	return &ac, nil
}

func (r *SQLObservabilityRepository) ListAlertChannelsByProjectID(ctx context.Context, projectID string) ([]AlertChannel, error) {
	query := `
		SELECT id, project_id, type, name, config_encrypted, is_active, created_at, updated_at
		FROM alert_channels
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list alert channels: %w", err)
	}
	defer rows.Close()

	var list []AlertChannel
	for rows.Next() {
		var ac AlertChannel
		if err := rows.Scan(
			&ac.ID,
			&ac.ProjectID,
			&ac.Type,
			&ac.Name,
			&ac.ConfigEncrypted,
			&ac.IsActive,
			&ac.CreatedAt,
			&ac.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert channel: %w", err)
		}
		list = append(list, ac)
	}
	return list, nil
}

func (r *SQLObservabilityRepository) UpdateAlertChannel(ctx context.Context, channel *AlertChannel) error {
	if channel == nil {
		return errors.New("alert channel cannot be nil")
	}
	if strings.TrimSpace(channel.Name) == "" {
		return ErrEmptyChannelName
	}
	if err := ValidateChannelType(channel.Type); err != nil {
		return err
	}
	channel.Type = strings.ToLower(strings.TrimSpace(channel.Type))
	channel.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE alert_channels
		SET name = $2, type = $3, config_encrypted = $4, is_active = $5, updated_at = $6
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query,
		channel.ID,
		channel.Name,
		channel.Type,
		channel.ConfigEncrypted,
		channel.IsActive,
		channel.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update alert channel: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update alert channel rows: %w", err)
	}
	if rows == 0 {
		return ErrAlertChannelNotFound
	}
	return nil
}

func (r *SQLObservabilityRepository) DeleteAlertChannel(ctx context.Context, id string) error {
	query := `DELETE FROM alert_channels WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete alert channel: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete alert channel rows: %w", err)
	}
	if rows == 0 {
		return ErrAlertChannelNotFound
	}
	return nil
}

// ----------------- Alert Incidents (SQL) -----------------

func (r *SQLObservabilityRepository) CreateAlertIncident(ctx context.Context, incident *AlertIncident) error {
	if incident == nil {
		return errors.New("alert incident cannot be nil")
	}
	if strings.TrimSpace(incident.ProjectID) == "" {
		return ErrEmptyProjectID
	}
	if strings.TrimSpace(incident.ID) == "" {
		incident.ID = generateObservabilityID("inc")
	}
	if strings.TrimSpace(incident.Status) == "" {
		incident.Status = "firing"
	}
	if incident.TriggeredAt.IsZero() {
		incident.TriggeredAt = time.Now().UTC()
	}

	query := `
		INSERT INTO alert_incidents (
			id, project_id, target_type, target_id, title, message, status, triggered_at, resolved_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		incident.ID,
		incident.ProjectID,
		incident.TargetType,
		incident.TargetID,
		incident.Title,
		incident.Message,
		incident.Status,
		incident.TriggeredAt,
		incident.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("create alert incident: %w", err)
	}
	return nil
}

func (r *SQLObservabilityRepository) GetAlertIncidentByID(ctx context.Context, id string) (*AlertIncident, error) {
	query := `
		SELECT id, project_id, target_type, target_id, title, message, status, triggered_at, resolved_at
		FROM alert_incidents
		WHERE id = $1
	`
	var ai AlertIncident
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&ai.ID,
		&ai.ProjectID,
		&ai.TargetType,
		&ai.TargetID,
		&ai.Title,
		&ai.Message,
		&ai.Status,
		&ai.TriggeredAt,
		&ai.ResolvedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAlertIncidentNotFound
		}
		return nil, fmt.Errorf("get alert incident by id: %w", err)
	}
	return &ai, nil
}

func (r *SQLObservabilityRepository) ListAlertIncidents(ctx context.Context, limit int) ([]AlertIncident, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, project_id, target_type, target_id, title, message, status, triggered_at, resolved_at
		FROM alert_incidents
		ORDER BY triggered_at DESC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list all alert incidents: %w", err)
	}
	defer rows.Close()

	var list []AlertIncident
	for rows.Next() {
		var ai AlertIncident
		if err := rows.Scan(
			&ai.ID,
			&ai.ProjectID,
			&ai.TargetType,
			&ai.TargetID,
			&ai.Title,
			&ai.Message,
			&ai.Status,
			&ai.TriggeredAt,
			&ai.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert incident: %w", err)
		}
		list = append(list, ai)
	}
	return list, nil
}

func (r *SQLObservabilityRepository) ListAlertIncidentsByProjectID(ctx context.Context, projectID string, limit int) ([]AlertIncident, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, project_id, target_type, target_id, title, message, status, triggered_at, resolved_at
		FROM alert_incidents
		WHERE project_id = $1
		ORDER BY triggered_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list alert incidents: %w", err)
	}
	defer rows.Close()

	var list []AlertIncident
	for rows.Next() {
		var ai AlertIncident
		if err := rows.Scan(
			&ai.ID,
			&ai.ProjectID,
			&ai.TargetType,
			&ai.TargetID,
			&ai.Title,
			&ai.Message,
			&ai.Status,
			&ai.TriggeredAt,
			&ai.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert incident: %w", err)
		}
		list = append(list, ai)
	}
	return list, nil
}

func (r *SQLObservabilityRepository) ListActiveAlertIncidents(ctx context.Context, projectID string) ([]AlertIncident, error) {
	query := `
		SELECT id, project_id, target_type, target_id, title, message, status, triggered_at, resolved_at
		FROM alert_incidents
		WHERE project_id = $1 AND status = 'firing'
		ORDER BY triggered_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list active alert incidents: %w", err)
	}
	defer rows.Close()

	var list []AlertIncident
	for rows.Next() {
		var ai AlertIncident
		if err := rows.Scan(
			&ai.ID,
			&ai.ProjectID,
			&ai.TargetType,
			&ai.TargetID,
			&ai.Title,
			&ai.Message,
			&ai.Status,
			&ai.TriggeredAt,
			&ai.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan active alert incident: %w", err)
		}
		list = append(list, ai)
	}
	return list, nil
}

func (r *SQLObservabilityRepository) ResolveAlertIncident(ctx context.Context, id string, resolvedAt time.Time) error {
	query := `
		UPDATE alert_incidents
		SET status = 'resolved', resolved_at = $2
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, id, resolvedAt.UTC())
	if err != nil {
		return fmt.Errorf("resolve alert incident: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("resolve alert incident rows: %w", err)
	}
	if rows == 0 {
		return ErrAlertIncidentNotFound
	}
	return nil
}

// ----------------- Memory Implementation -----------------

// MemoryObservabilityRepository provides a thread-safe in-memory store for unit tests and local development.
type MemoryObservabilityRepository struct {
	mu           sync.RWMutex
	healthChecks map[string]*HealthCheck
	metrics      []*ServerMetric
	channels     map[string]*AlertChannel
	incidents    map[string]*AlertIncident
}

// NewMemoryObservabilityRepository instantiates a new thread-safe in-memory ObservabilityRepository.
func NewMemoryObservabilityRepository() *MemoryObservabilityRepository {
	return &MemoryObservabilityRepository{
		healthChecks: make(map[string]*HealthCheck),
		metrics:      make([]*ServerMetric, 0),
		channels:     make(map[string]*AlertChannel),
		incidents:    make(map[string]*AlertIncident),
	}
}

// --- Health Checks (Memory) ---

func (m *MemoryObservabilityRepository) CreateHealthCheck(ctx context.Context, hc *HealthCheck) error {
	if hc == nil {
		return errors.New("health check cannot be nil")
	}
	if strings.TrimSpace(hc.DeploymentID) == "" {
		return ErrEmptyDeploymentID
	}
	if err := ValidateHealthCheckURL(hc.URL); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.healthChecks {
		if existing.DeploymentID == hc.DeploymentID {
			return ErrDuplicateHealthCheck
		}
	}

	if strings.TrimSpace(hc.ID) == "" {
		hc.ID = generateObservabilityID("hc")
	}
	if strings.TrimSpace(hc.Status) == "" {
		hc.Status = "healthy"
	}
	now := time.Now().UTC()
	if hc.CreatedAt.IsZero() {
		hc.CreatedAt = now
	}
	hc.UpdatedAt = now

	copied := *hc
	m.healthChecks[hc.ID] = &copied
	return nil
}

func (m *MemoryObservabilityRepository) GetHealthCheckByID(ctx context.Context, id string) (*HealthCheck, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hc, exists := m.healthChecks[id]
	if !exists {
		return nil, ErrHealthCheckNotFound
	}
	copied := *hc
	return &copied, nil
}

func (m *MemoryObservabilityRepository) GetHealthCheckByDeploymentID(ctx context.Context, deploymentID string) (*HealthCheck, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hc := range m.healthChecks {
		if hc.DeploymentID == deploymentID {
			copied := *hc
			return &copied, nil
		}
	}
	return nil, ErrHealthCheckNotFound
}

func (m *MemoryObservabilityRepository) ListHealthChecks(ctx context.Context) ([]HealthCheck, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]HealthCheck, 0, len(m.healthChecks))
	for _, hc := range m.healthChecks {
		list = append(list, *hc)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (m *MemoryObservabilityRepository) UpdateHealthCheck(ctx context.Context, hc *HealthCheck) error {
	if hc == nil {
		return errors.New("health check cannot be nil")
	}
	if err := ValidateHealthCheckURL(hc.URL); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.healthChecks[hc.ID]
	if !exists {
		return ErrHealthCheckNotFound
	}

	hc.UpdatedAt = time.Now().UTC()
	copied := *hc
	copied.CreatedAt = existing.CreatedAt
	m.healthChecks[hc.ID] = &copied
	return nil
}

func (m *MemoryObservabilityRepository) UpdateHealthCheckResult(ctx context.Context, id string, status string, statusCode int, responseTimeMs int64, consecutiveFailures int, checkedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hc, exists := m.healthChecks[id]
	if !exists {
		return ErrHealthCheckNotFound
	}

	hc.Status = status
	hc.LastStatusCode = statusCode
	hc.ResponseTimeMs = responseTimeMs
	hc.ConsecutiveFailures = consecutiveFailures
	t := checkedAt.UTC()
	hc.LastCheckedAt = &t
	hc.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *MemoryObservabilityRepository) DeleteHealthCheck(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.healthChecks[id]; !exists {
		return ErrHealthCheckNotFound
	}
	delete(m.healthChecks, id)
	return nil
}

// --- Server & Deployment Metrics (Memory) ---

func (m *MemoryObservabilityRepository) RecordServerMetric(ctx context.Context, metric *ServerMetric) error {
	if metric == nil {
		return errors.New("server metric cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(metric.ID) == "" {
		metric.ID = generateObservabilityID("met")
	}
	if metric.RecordedAt.IsZero() {
		metric.RecordedAt = time.Now().UTC()
	}

	copied := *metric
	m.metrics = append(m.metrics, &copied)
	return nil
}

func (m *MemoryObservabilityRepository) ListServerMetrics(ctx context.Context, serverID string, limit int) ([]ServerMetric, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var list []ServerMetric
	for i := len(m.metrics) - 1; i >= 0; i-- {
		met := m.metrics[i]
		if met.ServerID != nil && *met.ServerID == serverID {
			list = append(list, *met)
			if len(list) >= limit {
				break
			}
		}
	}
	return list, nil
}

func (m *MemoryObservabilityRepository) ListDeploymentMetrics(ctx context.Context, deploymentID string, limit int) ([]ServerMetric, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var list []ServerMetric
	for i := len(m.metrics) - 1; i >= 0; i-- {
		met := m.metrics[i]
		if met.DeploymentID != nil && *met.DeploymentID == deploymentID {
			list = append(list, *met)
			if len(list) >= limit {
				break
			}
		}
	}
	return list, nil
}

func (m *MemoryObservabilityRepository) GetLatestServerMetric(ctx context.Context, serverID string) (*ServerMetric, error) {
	metrics, err := m.ListServerMetrics(ctx, serverID, 1)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		return nil, sql.ErrNoRows
	}
	return &metrics[0], nil
}

// --- Alert Channels (Memory) ---

func (m *MemoryObservabilityRepository) CreateAlertChannel(ctx context.Context, channel *AlertChannel) error {
	if channel == nil {
		return errors.New("alert channel cannot be nil")
	}
	if strings.TrimSpace(channel.ProjectID) == "" {
		return ErrEmptyProjectID
	}
	if strings.TrimSpace(channel.Name) == "" {
		return ErrEmptyChannelName
	}
	if err := ValidateChannelType(channel.Type); err != nil {
		return err
	}
	channel.Type = strings.ToLower(strings.TrimSpace(channel.Type))

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(channel.ID) == "" {
		channel.ID = generateObservabilityID("alc")
	}
	now := time.Now().UTC()
	if channel.CreatedAt.IsZero() {
		channel.CreatedAt = now
	}
	channel.UpdatedAt = now

	copied := *channel
	m.channels[channel.ID] = &copied
	return nil
}

func (m *MemoryObservabilityRepository) GetAlertChannelByID(ctx context.Context, id string) (*AlertChannel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, exists := m.channels[id]
	if !exists {
		return nil, ErrAlertChannelNotFound
	}
	copied := *ch
	return &copied, nil
}

func (m *MemoryObservabilityRepository) ListAlertChannelsByProjectID(ctx context.Context, projectID string) ([]AlertChannel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]AlertChannel, 0)
	for _, ch := range m.channels {
		if ch.ProjectID == projectID {
			list = append(list, *ch)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list, nil
}

func (m *MemoryObservabilityRepository) UpdateAlertChannel(ctx context.Context, channel *AlertChannel) error {
	if channel == nil {
		return errors.New("alert channel cannot be nil")
	}
	if strings.TrimSpace(channel.Name) == "" {
		return ErrEmptyChannelName
	}
	if err := ValidateChannelType(channel.Type); err != nil {
		return err
	}
	channel.Type = strings.ToLower(strings.TrimSpace(channel.Type))

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.channels[channel.ID]
	if !exists {
		return ErrAlertChannelNotFound
	}

	channel.UpdatedAt = time.Now().UTC()
	copied := *channel
	copied.CreatedAt = existing.CreatedAt
	m.channels[channel.ID] = &copied
	return nil
}

func (m *MemoryObservabilityRepository) DeleteAlertChannel(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.channels[id]; !exists {
		return ErrAlertChannelNotFound
	}
	delete(m.channels, id)
	return nil
}

// --- Alert Incidents (Memory) ---

func (m *MemoryObservabilityRepository) CreateAlertIncident(ctx context.Context, incident *AlertIncident) error {
	if incident == nil {
		return errors.New("alert incident cannot be nil")
	}
	if strings.TrimSpace(incident.ProjectID) == "" {
		return ErrEmptyProjectID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(incident.ID) == "" {
		incident.ID = generateObservabilityID("inc")
	}
	if strings.TrimSpace(incident.Status) == "" {
		incident.Status = "firing"
	}
	if incident.TriggeredAt.IsZero() {
		incident.TriggeredAt = time.Now().UTC()
	}

	copied := *incident
	m.incidents[incident.ID] = &copied
	return nil
}

func (m *MemoryObservabilityRepository) GetAlertIncidentByID(ctx context.Context, id string) (*AlertIncident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inc, exists := m.incidents[id]
	if !exists {
		return nil, ErrAlertIncidentNotFound
	}
	copied := *inc
	return &copied, nil
}

func (m *MemoryObservabilityRepository) ListAlertIncidents(ctx context.Context, limit int) ([]AlertIncident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	list := make([]AlertIncident, 0, len(m.incidents))
	for _, inc := range m.incidents {
		list = append(list, *inc)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].TriggeredAt.After(list[j].TriggeredAt)
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func (m *MemoryObservabilityRepository) ListAlertIncidentsByProjectID(ctx context.Context, projectID string, limit int) ([]AlertIncident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	list := make([]AlertIncident, 0)
	for _, inc := range m.incidents {
		if inc.ProjectID == projectID {
			list = append(list, *inc)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].TriggeredAt.After(list[j].TriggeredAt)
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func (m *MemoryObservabilityRepository) ListActiveAlertIncidents(ctx context.Context, projectID string) ([]AlertIncident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]AlertIncident, 0)
	for _, inc := range m.incidents {
		if inc.ProjectID == projectID && inc.Status == "firing" {
			list = append(list, *inc)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].TriggeredAt.After(list[j].TriggeredAt)
	})
	return list, nil
}

func (m *MemoryObservabilityRepository) ResolveAlertIncident(ctx context.Context, id string, resolvedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inc, exists := m.incidents[id]
	if !exists {
		return ErrAlertIncidentNotFound
	}
	inc.Status = "resolved"
	t := resolvedAt.UTC()
	inc.ResolvedAt = &t
	return nil
}

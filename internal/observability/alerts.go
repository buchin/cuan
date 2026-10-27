package observability

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
)

const (
	AlertStatusFiring   = "firing"
	AlertStatusResolved = "resolved"

	ChannelTypeTelegram = "telegram"
	ChannelTypeDiscord  = "discord"
	ChannelTypeWebhook  = "webhook"

	DefaultCooldown           = 10 * time.Minute
	DefaultAlertHTTPTimeout   = 5 * time.Second
	DefaultTelegramBaseURL    = "https://api.telegram.org"
	DefaultMaxRetries         = 2
	DefaultRetryBackoffMillis = 50

	ResourceThresholdPercent = 90.0
)

var (
	ErrInvalidChannelConfig   = errors.New("invalid channel configuration")
	ErrUnsupportedChannelType = errors.New("unsupported alert channel type")
	ErrDeliveryFailed         = errors.New("notification delivery failed")
)

// ChannelConfig holds the unencrypted integration credentials for alert notification channels.
type ChannelConfig struct {
	// Telegram Bot API
	BotToken        string `json:"bot_token,omitempty"`
	ChatID          string `json:"chat_id,omitempty"`
	TelegramBaseURL string `json:"telegram_base_url,omitempty"` // For mock tests, defaults to https://api.telegram.org

	// Discord & Generic Webhook
	WebhookURL string `json:"webhook_url,omitempty"`

	// Generic Webhook HMAC secret
	Secret string `json:"secret,omitempty"`
}

// ValidateChannelConfig checks that required credentials exist and are well-formed for the given channel type.
func ValidateChannelConfig(channelType string, cfg *ChannelConfig) error {
	if cfg == nil {
		return fmt.Errorf("%w: config cannot be nil", ErrInvalidChannelConfig)
	}

	normType := strings.ToLower(strings.TrimSpace(channelType))
	switch normType {
	case ChannelTypeTelegram:
		if strings.TrimSpace(cfg.BotToken) == "" {
			return fmt.Errorf("%w: telegram bot_token is required", ErrInvalidChannelConfig)
		}
		if strings.TrimSpace(cfg.ChatID) == "" {
			return fmt.Errorf("%w: telegram chat_id is required", ErrInvalidChannelConfig)
		}
		return nil

	case ChannelTypeDiscord:
		trimmedURL := strings.TrimSpace(cfg.WebhookURL)
		if trimmedURL == "" {
			return fmt.Errorf("%w: discord webhook_url is required", ErrInvalidChannelConfig)
		}
		if err := db.ValidateHealthCheckURL(trimmedURL); err != nil {
			return fmt.Errorf("%w: invalid discord webhook url: %v", ErrInvalidChannelConfig, err)
		}
		return nil

	case ChannelTypeWebhook:
		trimmedURL := strings.TrimSpace(cfg.WebhookURL)
		if trimmedURL == "" {
			return fmt.Errorf("%w: webhook url is required", ErrInvalidChannelConfig)
		}
		if err := db.ValidateHealthCheckURL(trimmedURL); err != nil {
			return fmt.Errorf("%w: invalid webhook url: %v", ErrInvalidChannelConfig, err)
		}
		return nil

	default:
		return fmt.Errorf("%w: '%s'", ErrUnsupportedChannelType, channelType)
	}
}

// MaskSecret masks sensitive credentials for safe serialization in API responses.
func MaskSecret(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 4 {
		return "***"
	}
	if len(trimmed) <= 8 {
		return trimmed[:2] + "***"
	}
	return trimmed[:4] + "***" + trimmed[len(trimmed)-2:]
}

// MaskURL preserves scheme, host, and path prefix while masking sensitive webhook tokens.
func MaskURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return MaskSecret(trimmed)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) <= 1 {
		return fmt.Sprintf("%s://%s/***", u.Scheme, u.Host)
	}
	// Preserve first path segment, mask the rest
	return fmt.Sprintf("%s://%s/%s/***", u.Scheme, u.Host, parts[0])
}

// MaskChannelConfig produces a sanitized map of credential previews with zero secret leakage.
func MaskChannelConfig(channelType string, cfg *ChannelConfig) map[string]string {
	if cfg == nil {
		return map[string]string{}
	}

	normType := strings.ToLower(strings.TrimSpace(channelType))
	switch normType {
	case ChannelTypeTelegram:
		return map[string]string{
			"bot_token": MaskSecret(cfg.BotToken),
			"chat_id":   cfg.ChatID,
		}
	case ChannelTypeDiscord:
		return map[string]string{
			"webhook_url": MaskURL(cfg.WebhookURL),
		}
	case ChannelTypeWebhook:
		res := map[string]string{
			"webhook_url": MaskURL(cfg.WebhookURL),
		}
		if cfg.Secret != "" {
			res["secret"] = MaskSecret(cfg.Secret)
		}
		return res
	default:
		return map[string]string{}
	}
}

// ComputeHMACSHA256 returns the hex-encoded HMAC-SHA256 digest of payload using secret.
func ComputeHMACSHA256(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// AlertEvent represents the standard domain event dispatched to alert channels.
type AlertEvent struct {
	IncidentID string                 `json:"incident_id"`
	ProjectID  string                 `json:"project_id"`
	TargetType string                 `json:"target_type"` // 'deployment', 'server', 'system'
	TargetID   string                 `json:"target_id"`
	Title      string                 `json:"title"`
	Message    string                 `json:"message"`
	Status     string                 `json:"status"` // 'firing', 'resolved'
	Timestamp  time.Time              `json:"timestamp"`
	Data       map[string]interface{} `json:"data,omitempty"`
	StudioURL  string                 `json:"studio_url,omitempty"`
}

// Notifier defines the delivery contract for an alert notification channel.
type Notifier interface {
	Send(ctx context.Context, cfg *ChannelConfig, event *AlertEvent) error
}

// ----------------- Telegram Bot API Notifier -----------------

// TelegramNotifier sends formatted Markdown notifications via Telegram Bot API sendMessage endpoint.
type TelegramNotifier struct {
	client     *http.Client
	defaultURL string
}

// NewTelegramNotifier creates a TelegramNotifier.
func NewTelegramNotifier(client *http.Client, defaultURL ...string) *TelegramNotifier {
	if client == nil {
		client = &http.Client{Timeout: DefaultAlertHTTPTimeout}
	}
	base := DefaultTelegramBaseURL
	if len(defaultURL) > 0 && defaultURL[0] != "" {
		base = strings.TrimRight(defaultURL[0], "/")
	}
	return &TelegramNotifier{
		client:     client,
		defaultURL: base,
	}
}

// Send dispatches a Telegram message formatted with Markdown.
func (t *TelegramNotifier) Send(ctx context.Context, cfg *ChannelConfig, event *AlertEvent) error {
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("%w: bot_token and chat_id required", ErrInvalidChannelConfig)
	}

	baseURL := t.defaultURL
	if cfg.TelegramBaseURL != "" {
		baseURL = strings.TrimRight(cfg.TelegramBaseURL, "/")
	}
	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, cfg.BotToken)

	icon := "🚨"
	statusUpper := "FIRING"
	if event.Status == AlertStatusResolved {
		icon = "✅"
		statusUpper = "RESOLVED"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s *[%s] %s*\n\n", icon, statusUpper, event.Title))
	sb.WriteString(fmt.Sprintf("*Target:* `%s (%s)`\n", event.TargetType, event.TargetID))
	sb.WriteString(fmt.Sprintf("*Status:* %s\n", statusUpper))
	sb.WriteString(fmt.Sprintf("*Details:* %s\n", event.Message))
	sb.WriteString(fmt.Sprintf("*Time:* %s\n", event.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC")))

	if event.StudioURL != "" {
		sb.WriteString(fmt.Sprintf("\n[Open Studio Dashboard](%s)", event.StudioURL))
	}

	reqBody := map[string]interface{}{
		"chat_id":    cfg.ChatID,
		"text":       sb.String(),
		"parse_mode": "Markdown",
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	return executeHTTPWithRetry(ctx, t.client, http.MethodPost, endpoint, bodyBytes, map[string]string{
		"Content-Type": "application/json",
	})
}

// ----------------- Discord Webhook Notifier -----------------

// DiscordNotifier sends rich embed notifications with color accents to Discord webhooks.
type DiscordNotifier struct {
	client *http.Client
}

// NewDiscordNotifier creates a DiscordNotifier.
func NewDiscordNotifier(client *http.Client) *DiscordNotifier {
	if client == nil {
		client = &http.Client{Timeout: DefaultAlertHTTPTimeout}
	}
	return &DiscordNotifier{client: client}
}

// Send dispatches a rich embed to Discord webhook.
func (d *DiscordNotifier) Send(ctx context.Context, cfg *ChannelConfig, event *AlertEvent) error {
	if cfg.WebhookURL == "" {
		return fmt.Errorf("%w: discord webhook_url required", ErrInvalidChannelConfig)
	}

	color := 15158332 // Hex 0xE74C3C (Red for firing)
	icon := "🚨"
	statusUpper := "FIRING"
	if event.Status == AlertStatusResolved {
		color = 3066993 // Hex 0x2ECC71 (Green for resolved)
		icon = "✅"
		statusUpper = "RESOLVED"
	}

	fields := []map[string]interface{}{
		{"name": "Status", "value": statusUpper, "inline": true},
		{"name": "Target", "value": fmt.Sprintf("%s (%s)", event.TargetType, event.TargetID), "inline": true},
		{"name": "Details", "value": event.Message, "inline": false},
		{"name": "Time", "value": event.Timestamp.UTC().Format(time.RFC3339), "inline": true},
	}

	embed := map[string]interface{}{
		"title":       fmt.Sprintf("%s %s", icon, event.Title),
		"description": event.Message,
		"color":       color,
		"fields":      fields,
		"timestamp":   event.Timestamp.UTC().Format(time.RFC3339),
	}
	if event.StudioURL != "" {
		embed["url"] = event.StudioURL
	}

	payload := map[string]interface{}{
		"username": "Cuan Alert Dispatcher",
		"embeds":   []interface{}{embed},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	return executeHTTPWithRetry(ctx, d.client, http.MethodPost, cfg.WebhookURL, bodyBytes, map[string]string{
		"Content-Type": "application/json",
	})
}

// ----------------- Custom Generic Webhook Notifier -----------------

// WebhookNotifier sends standard JSON webhook payloads signed with HMAC-SHA256 headers.
type WebhookNotifier struct {
	client *http.Client
}

// NewWebhookNotifier creates a WebhookNotifier.
func NewWebhookNotifier(client *http.Client) *WebhookNotifier {
	if client == nil {
		client = &http.Client{Timeout: DefaultAlertHTTPTimeout}
	}
	return &WebhookNotifier{client: client}
}

// Send dispatches a standardized JSON alert payload with X-Cuan-Signature HMAC-SHA256 header.
func (w *WebhookNotifier) Send(ctx context.Context, cfg *ChannelConfig, event *AlertEvent) error {
	if cfg.WebhookURL == "" {
		return fmt.Errorf("%w: webhook_url required", ErrInvalidChannelConfig)
	}

	eventName := "alert.firing"
	if event.Status == AlertStatusResolved {
		eventName = "alert.resolved"
	}

	payload := map[string]interface{}{
		"event":       eventName,
		"incident_id": event.IncidentID,
		"project_id":  event.ProjectID,
		"target_type": event.TargetType,
		"target_id":   event.TargetID,
		"title":       event.Title,
		"message":     event.Message,
		"status":      event.Status,
		"timestamp":   event.Timestamp.UTC().Format(time.RFC3339),
		"data":        event.Data,
		"studio_url":  event.StudioURL,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "Cuan-AlertDispatcher/1.0",
	}

	if cfg.Secret != "" {
		signature := ComputeHMACSHA256(bodyBytes, cfg.Secret)
		headers["X-Cuan-Signature"] = "sha256=" + signature
	}

	return executeHTTPWithRetry(ctx, w.client, http.MethodPost, cfg.WebhookURL, bodyBytes, headers)
}

// executeHTTPWithRetry executes an HTTP request with bounded retries on transient errors (5xx, network).
func executeHTTPWithRetry(ctx context.Context, client *http.Client, method, targetURL string, body []byte, headers map[string]string) error {
	var lastErr error

	for attempt := 0; attempt <= DefaultMaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 0 {
			time.Sleep(time.Duration(attempt*DefaultRetryBackoffMillis) * time.Millisecond)
		}

		req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create http request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue // Transient network error, retry
		}

		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil // Successful delivery
		}

		lastErr = fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))

		// Do not retry 4xx client errors (e.g. 400 Bad Request, 401 Unauthorized, 404 Not Found)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("%w: %v", ErrDeliveryFailed, lastErr)
		}
	}

	return fmt.Errorf("%w after %d retries: %v", ErrDeliveryFailed, DefaultMaxRetries, lastErr)
}

// ----------------- Incident Alert Dispatcher & Anti-Spam Engine -----------------

// TargetAlertState maintains in-memory tracking of alert transitions per target.
type TargetAlertState struct {
	TargetType     string
	TargetID       string
	IsFiring       bool
	LastFiredAt    time.Time
	LastIncidentID string
	LastTitle      string
}

// AlertDispatcherOption configures an AlertDispatcher instance.
type AlertDispatcherOption func(*AlertDispatcher)

// WithCooldown sets the deduplication and anti-spam suppression window.
func WithCooldown(d time.Duration) AlertDispatcherOption {
	return func(ad *AlertDispatcher) {
		if d > 0 {
			ad.cooldown = d
		}
	}
}

// WithAlertKeyVault sets the KeyVault for at-rest credential encryption/decryption.
func WithAlertKeyVault(vault *crypto.KeyVault) AlertDispatcherOption {
	return func(ad *AlertDispatcher) {
		if vault != nil {
			ad.vault = vault
		}
	}
}

// WithAlertHTTPClient overrides the HTTP client for all notifiers.
func WithAlertHTTPClient(client *http.Client) AlertDispatcherOption {
	return func(ad *AlertDispatcher) {
		if client != nil {
			ad.httpClient = client
			ad.telegramNotifier = NewTelegramNotifier(client)
			ad.discordNotifier = NewDiscordNotifier(client)
			ad.webhookNotifier = NewWebhookNotifier(client)
		}
	}
}

// WithAlertBaseURL sets the base URL for generating Studio dashboard links.
func WithAlertBaseURL(baseURL string) AlertDispatcherOption {
	return func(ad *AlertDispatcher) {
		ad.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithProjectResolver sets a custom project resolver callback for mapping targets to project IDs.
func WithProjectResolver(resolver func(ctx context.Context, targetType, targetID string) string) AlertDispatcherOption {
	return func(ad *AlertDispatcher) {
		ad.projectResolver = resolver
	}
}

// AlertDispatcher evaluates health and telemetry checks, detects threshold transitions,
// suppresses notification storms via cooldown deduplication, and parallel dispatches alerts.
type AlertDispatcher struct {
	repo             db.ObservabilityRepository
	vault            *crypto.KeyVault
	httpClient       *http.Client
	telegramNotifier *TelegramNotifier
	discordNotifier  *DiscordNotifier
	webhookNotifier  *WebhookNotifier
	cooldown         time.Duration
	baseURL          string
	projectResolver  func(ctx context.Context, targetType, targetID string) string

	mu     sync.RWMutex
	states map[string]*TargetAlertState
}

// NewAlertDispatcher creates a new AlertDispatcher.
func NewAlertDispatcher(repo db.ObservabilityRepository, opts ...AlertDispatcherOption) *AlertDispatcher {
	client := &http.Client{Timeout: DefaultAlertHTTPTimeout}
	ad := &AlertDispatcher{
		repo:             repo,
		vault:            crypto.DefaultKeyVault(),
		httpClient:       client,
		telegramNotifier: NewTelegramNotifier(client),
		discordNotifier:  NewDiscordNotifier(client),
		webhookNotifier:  NewWebhookNotifier(client),
		cooldown:         DefaultCooldown,
		baseURL:          "https://cuan.dojo.cc",
		states:           make(map[string]*TargetAlertState),
	}

	for _, opt := range opts {
		opt(ad)
	}

	return ad
}

// SetHTTPClient dynamically sets or updates the HTTP client for all notifiers.
func (ad *AlertDispatcher) SetHTTPClient(client *http.Client) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	if client != nil {
		ad.httpClient = client
		ad.telegramNotifier = NewTelegramNotifier(client)
		ad.discordNotifier = NewDiscordNotifier(client)
		ad.webhookNotifier = NewWebhookNotifier(client)
	}
}

// HandleHealthProbe inspects health check probe results and evaluates outage or recovery transitions.
func (ad *AlertDispatcher) HandleHealthProbe(ctx context.Context, hc *db.HealthCheck, res *ProbeResult) error {
	if hc == nil {
		return nil
	}

	targetType := "deployment"
	targetID := hc.DeploymentID

	shouldFire, shouldResolve, title, message := ad.EvaluateHealth(hc, res)
	if !shouldFire && !shouldResolve {
		return nil
	}

	stateKey := fmt.Sprintf("%s:%s:health", targetType, targetID)
	now := time.Now().UTC()

	ad.mu.Lock()
	state, exists := ad.states[stateKey]
	if !exists {
		state = &TargetAlertState{
			TargetType: targetType,
			TargetID:   targetID,
		}
		ad.states[stateKey] = state
	}

	// 1. Evaluate OUTAGE FIRING
	if shouldFire {
		// Anti-spam & Cooldown check
		if state.IsFiring && time.Since(state.LastFiredAt) < ad.cooldown {
			// Suppress duplicate notification alert storm
			ad.mu.Unlock()
			return nil
		}

		projectID := ad.resolveProjectID(ctx, targetType, targetID)
		incident := &db.AlertIncident{
			ProjectID:   projectID,
			TargetType:  targetType,
			TargetID:    targetID,
			Title:       title,
			Message:     message,
			Status:      AlertStatusFiring,
			TriggeredAt: now,
		}

		if err := ad.repo.CreateAlertIncident(ctx, incident); err != nil {
			ad.mu.Unlock()
			return fmt.Errorf("create alert incident: %w", err)
		}

		state.IsFiring = true
		state.LastFiredAt = now
		state.LastIncidentID = incident.ID
		state.LastTitle = title
		ad.mu.Unlock()

		event := &AlertEvent{
			IncidentID: incident.ID,
			ProjectID:  projectID,
			TargetType: targetType,
			TargetID:   targetID,
			Title:      title,
			Message:    message,
			Status:     AlertStatusFiring,
			Timestamp:  now,
			StudioURL:  fmt.Sprintf("%s/deployments/%s", ad.baseURL, targetID),
			Data: map[string]interface{}{
				"consecutive_failures": hc.ConsecutiveFailures,
				"status_code":          hc.LastStatusCode,
				"response_time_ms":     hc.ResponseTimeMs,
			},
		}

		return ad.DispatchToProject(ctx, projectID, event)
	}

	// 2. Evaluate RECOVERY RESOLVED
	if shouldResolve && state.IsFiring {
		projectID := ad.resolveProjectID(ctx, targetType, targetID)
		incidentID := state.LastIncidentID

		if incidentID != "" {
			_ = ad.repo.ResolveAlertIncident(ctx, incidentID, now)
		}

		state.IsFiring = false
		state.LastIncidentID = ""
		ad.mu.Unlock()

		event := &AlertEvent{
			IncidentID: incidentID,
			ProjectID:  projectID,
			TargetType: targetType,
			TargetID:   targetID,
			Title:      title,
			Message:    message,
			Status:     AlertStatusResolved,
			Timestamp:  now,
			StudioURL:  fmt.Sprintf("%s/deployments/%s", ad.baseURL, targetID),
			Data: map[string]interface{}{
				"status":           hc.Status,
				"status_code":      hc.LastStatusCode,
				"response_time_ms": hc.ResponseTimeMs,
			},
		}

		return ad.DispatchToProject(ctx, projectID, event)
	}

	ad.mu.Unlock()
	return nil
}

// HandleServerMetric evaluates system hardware resource telemetry for resource threshold violations.
func (ad *AlertDispatcher) HandleServerMetric(ctx context.Context, metric *db.ServerMetric) error {
	if metric == nil {
		return nil
	}

	targetType := "server"
	targetID := "local"
	if metric.ServerID != nil && *metric.ServerID != "" {
		targetID = *metric.ServerID
	} else if metric.DeploymentID != nil && *metric.DeploymentID != "" {
		targetType = "deployment"
		targetID = *metric.DeploymentID
	}

	shouldFire, shouldResolve, title, message := ad.EvaluateMetric(metric)
	if !shouldFire && !shouldResolve {
		return nil
	}

	stateKey := fmt.Sprintf("%s:%s:metric", targetType, targetID)
	now := time.Now().UTC()

	ad.mu.Lock()
	state, exists := ad.states[stateKey]
	if !exists {
		state = &TargetAlertState{
			TargetType: targetType,
			TargetID:   targetID,
		}
		ad.states[stateKey] = state
	}

	// 1. Evaluate RESOURCE USAGE FIRING
	if shouldFire {
		// Anti-spam cooldown check
		if state.IsFiring && time.Since(state.LastFiredAt) < ad.cooldown {
			ad.mu.Unlock()
			return nil
		}

		projectID := ad.resolveProjectID(ctx, targetType, targetID)
		incident := &db.AlertIncident{
			ProjectID:   projectID,
			TargetType:  targetType,
			TargetID:    targetID,
			Title:       title,
			Message:     message,
			Status:      AlertStatusFiring,
			TriggeredAt: now,
		}

		if err := ad.repo.CreateAlertIncident(ctx, incident); err != nil {
			ad.mu.Unlock()
			return fmt.Errorf("create resource alert incident: %w", err)
		}

		state.IsFiring = true
		state.LastFiredAt = now
		state.LastIncidentID = incident.ID
		state.LastTitle = title
		ad.mu.Unlock()

		event := &AlertEvent{
			IncidentID: incident.ID,
			ProjectID:  projectID,
			TargetType: targetType,
			TargetID:   targetID,
			Title:      title,
			Message:    message,
			Status:     AlertStatusFiring,
			Timestamp:  now,
			StudioURL:  fmt.Sprintf("%s/%ss/%s", ad.baseURL, targetType, targetID),
			Data: map[string]interface{}{
				"cpu_percent":     metric.CPUPercent,
				"memory_used_mb":  metric.MemoryUsedMB,
				"memory_total_mb": metric.MemoryTotalMB,
				"disk_used_gb":    metric.DiskUsedGB,
				"disk_total_gb":   metric.DiskTotalGB,
			},
		}

		return ad.DispatchToProject(ctx, projectID, event)
	}

	// 2. Evaluate RESOURCE RECOVERY RESOLVED
	if shouldResolve && state.IsFiring {
		projectID := ad.resolveProjectID(ctx, targetType, targetID)
		incidentID := state.LastIncidentID

		if incidentID != "" {
			_ = ad.repo.ResolveAlertIncident(ctx, incidentID, now)
		}

		state.IsFiring = false
		state.LastIncidentID = ""
		ad.mu.Unlock()

		event := &AlertEvent{
			IncidentID: incidentID,
			ProjectID:  projectID,
			TargetType: targetType,
			TargetID:   targetID,
			Title:      title,
			Message:    message,
			Status:     AlertStatusResolved,
			Timestamp:  now,
			StudioURL:  fmt.Sprintf("%s/%ss/%s", ad.baseURL, targetType, targetID),
			Data: map[string]interface{}{
				"cpu_percent": metric.CPUPercent,
			},
		}

		return ad.DispatchToProject(ctx, projectID, event)
	}

	ad.mu.Unlock()
	return nil
}

// EvaluateHealth checks probe and health record values against outage and recovery rules.
func (ad *AlertDispatcher) EvaluateHealth(hc *db.HealthCheck, res *ProbeResult) (bool, bool, string, string) {
	isOutage := hc.ConsecutiveFailures >= 3 || hc.Status == StatusUnhealthy
	isHealthy := hc.Status == StatusHealthy && hc.ConsecutiveFailures == 0

	if isOutage {
		errMsg := ""
		if res != nil && res.Error != "" {
			errMsg = fmt.Sprintf(" - %s", res.Error)
		}
		title := fmt.Sprintf("Service Outage: %s", hc.DeploymentID)
		msg := fmt.Sprintf("Deployment probe failed %d consecutive times (Status: %d, Response: %dms%s)",
			hc.ConsecutiveFailures, hc.LastStatusCode, hc.ResponseTimeMs, errMsg)
		return true, false, title, msg
	}

	if isHealthy {
		title := fmt.Sprintf("Service Recovered: %s", hc.DeploymentID)
		msg := fmt.Sprintf("Deployment returned to healthy (Status: %d, Latency: %dms)",
			hc.LastStatusCode, hc.ResponseTimeMs)
		return false, true, title, msg
	}

	return false, false, "", ""
}

// EvaluateMetric checks server and deployment metrics against resource thresholds (>90%).
func (ad *AlertDispatcher) EvaluateMetric(metric *db.ServerMetric) (bool, bool, string, string) {
	var memPct float64
	if metric.MemoryTotalMB > 0 {
		memPct = float64(metric.MemoryUsedMB) / float64(metric.MemoryTotalMB) * 100.0
	}

	isExceeded := metric.CPUPercent > ResourceThresholdPercent || memPct > ResourceThresholdPercent
	isNormal := metric.CPUPercent <= ResourceThresholdPercent && memPct <= ResourceThresholdPercent

	target := "local-runner"
	if metric.ServerID != nil && *metric.ServerID != "" {
		target = *metric.ServerID
	} else if metric.DeploymentID != nil && *metric.DeploymentID != "" {
		target = *metric.DeploymentID
	}

	if isExceeded {
		title := fmt.Sprintf("High Resource Usage: %s", target)
		msg := fmt.Sprintf("Resource threshold (>90%%) exceeded: CPU: %.1f%%, RAM: %.1f%% (%dMB / %dMB)",
			metric.CPUPercent, memPct, metric.MemoryUsedMB, metric.MemoryTotalMB)
		return true, false, title, msg
	}

	if isNormal {
		title := fmt.Sprintf("Resource Usage Normal: %s", target)
		msg := fmt.Sprintf("Resource usage returned to normal: CPU: %.1f%%, RAM: %.1f%%",
			metric.CPUPercent, memPct)
		return false, true, title, msg
	}

	return false, false, "", ""
}

// DispatchToProject sends the alert event in parallel to all active channels registered for projectID.
func (ad *AlertDispatcher) DispatchToProject(ctx context.Context, projectID string, event *AlertEvent) error {
	channels, err := ad.repo.ListAlertChannelsByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list alert channels for project %s: %w", projectID, err)
	}

	var activeChannels []*db.AlertChannel
	for i := range channels {
		if channels[i].IsActive {
			activeChannels = append(activeChannels, &channels[i])
		}
	}

	if len(activeChannels) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(activeChannels))

	for _, ch := range activeChannels {
		wg.Add(1)
		channelCopy := ch
		go func() {
			defer wg.Done()
			if err := ad.DispatchChannel(ctx, channelCopy, event); err != nil {
				errChan <- fmt.Errorf("channel %s (%s): %w", channelCopy.ID, channelCopy.Type, err)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("alert dispatch errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// DispatchChannel decrypts channel credentials and delivers the event using the matching notifier.
func (ad *AlertDispatcher) DispatchChannel(ctx context.Context, channel *db.AlertChannel, event *AlertEvent) error {
	if channel == nil {
		return errors.New("channel cannot be nil")
	}

	cfg, err := ad.DecryptChannelConfig(channel)
	if err != nil {
		return fmt.Errorf("decrypt channel config: %w", err)
	}

	normType := strings.ToLower(strings.TrimSpace(channel.Type))
	switch normType {
	case ChannelTypeTelegram:
		return ad.telegramNotifier.Send(ctx, cfg, event)
	case ChannelTypeDiscord:
		return ad.discordNotifier.Send(ctx, cfg, event)
	case ChannelTypeWebhook:
		return ad.webhookNotifier.Send(ctx, cfg, event)
	default:
		return fmt.Errorf("%w: '%s'", ErrUnsupportedChannelType, channel.Type)
	}
}

// TestChannel sends a 1-click test demo notification to verify integration credentials.
func (ad *AlertDispatcher) TestChannel(ctx context.Context, channel *db.AlertChannel) error {
	if channel == nil {
		return errors.New("channel cannot be nil")
	}

	now := time.Now().UTC()
	testEvent := &AlertEvent{
		IncidentID: "test_demo_click",
		ProjectID:  channel.ProjectID,
		TargetType: "system",
		TargetID:   "studio",
		Title:      "Test Alert: Cuan Studio Notification Engine",
		Message:    fmt.Sprintf("This is a 1-click test alert from Cuan Studio for channel '%s'. Your integration is working perfectly!", channel.Name),
		Status:     AlertStatusFiring,
		Timestamp:  now,
		StudioURL:  fmt.Sprintf("%s/projects/%s", ad.baseURL, channel.ProjectID),
		Data: map[string]interface{}{
			"channel_name": channel.Name,
			"channel_type": channel.Type,
			"test_mode":    true,
		},
	}

	return ad.DispatchChannel(ctx, channel, testEvent)
}

// EncryptChannelConfig encrypts the channel configuration with KeyVault AES-GCM for at-rest security.
func (ad *AlertDispatcher) EncryptChannelConfig(cfg *ChannelConfig) (string, error) {
	if cfg == nil {
		return "", errors.New("channel config cannot be nil")
	}
	bytes, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal channel config: %w", err)
	}
	vault := ad.vault
	if vault == nil {
		vault = crypto.DefaultKeyVault()
	}
	return vault.Encrypt(bytes)
}

// DecryptChannelConfig decrypts channel credentials from at-rest encrypted storage.
func (ad *AlertDispatcher) DecryptChannelConfig(channel *db.AlertChannel) (*ChannelConfig, error) {
	if channel.ConfigEncrypted == "" {
		return nil, fmt.Errorf("%w: empty encrypted configuration", ErrInvalidChannelConfig)
	}
	vault := ad.vault
	if vault == nil {
		vault = crypto.DefaultKeyVault()
	}

	plaintextBytes, err := vault.Decrypt(channel.ConfigEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}

	var cfg ChannelConfig
	if err := json.Unmarshal(plaintextBytes, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal decrypted config: %w", err)
	}

	return &cfg, nil
}

// GetMaskedConfig returns the sanitized credential representation for safe API serialization.
func (ad *AlertDispatcher) GetMaskedConfig(channel *db.AlertChannel) map[string]string {
	cfg, err := ad.DecryptChannelConfig(channel)
	if err != nil {
		return map[string]string{"error": "unable to decrypt credentials"}
	}
	return MaskChannelConfig(channel.Type, cfg)
}

func (ad *AlertDispatcher) resolveProjectID(ctx context.Context, targetType, targetID string) string {
	if ad.projectResolver != nil {
		if pid := ad.projectResolver(ctx, targetType, targetID); pid != "" {
			return pid
		}
	}
	return "default"
}

package observability

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
)

func TestValidateChannelConfig(t *testing.T) {
	// 1. Nil config
	if err := ValidateChannelConfig("telegram", nil); err == nil {
		t.Errorf("expected error on nil config, got nil")
	}

	// 2. Unsupported channel
	if err := ValidateChannelConfig("slack", &ChannelConfig{}); err == nil {
		t.Errorf("expected error on unsupported channel, got nil")
	}

	// 3. Telegram validation
	if err := ValidateChannelConfig("telegram", &ChannelConfig{}); err == nil {
		t.Errorf("expected error on empty telegram config")
	}
	if err := ValidateChannelConfig("telegram", &ChannelConfig{BotToken: "123"}); err == nil {
		t.Errorf("expected error on missing chat_id")
	}
	if err := ValidateChannelConfig("telegram", &ChannelConfig{BotToken: "123", ChatID: "456"}); err != nil {
		t.Errorf("unexpected error on valid telegram config: %v", err)
	}

	// 4. Discord validation
	if err := ValidateChannelConfig("discord", &ChannelConfig{}); err == nil {
		t.Errorf("expected error on empty discord webhook_url")
	}
	if err := ValidateChannelConfig("discord", &ChannelConfig{WebhookURL: "ftp://invalid"}); err == nil {
		t.Errorf("expected error on invalid discord URL scheme")
	}
	if err := ValidateChannelConfig("discord", &ChannelConfig{WebhookURL: "https://discord.com/api/webhooks/123/abc"}); err != nil {
		t.Errorf("unexpected error on valid discord config: %v", err)
	}

	// 5. Generic Webhook validation
	if err := ValidateChannelConfig("webhook", &ChannelConfig{}); err == nil {
		t.Errorf("expected error on empty generic webhook URL")
	}
	if err := ValidateChannelConfig("webhook", &ChannelConfig{WebhookURL: "https://api.example.com/alerts", Secret: "sec123"}); err != nil {
		t.Errorf("unexpected error on valid webhook config: %v", err)
	}
}

func TestMaskingHelpers(t *testing.T) {
	// MaskSecret tests
	if MaskSecret("") != "" {
		t.Errorf("expected empty string for empty input")
	}
	if MaskSecret("123") != "***" {
		t.Errorf("expected *** for short secret, got %s", MaskSecret("123"))
	}
	if MaskSecret("1234567") != "12***" {
		t.Errorf("expected 12*** for medium secret, got %s", MaskSecret("1234567"))
	}
	if MaskSecret("1234567890abcdef") != "1234***ef" {
		t.Errorf("expected 1234***ef for long secret, got %s", MaskSecret("1234567890abcdef"))
	}

	// MaskURL tests
	discordURL := "https://discord.com/api/webhooks/987654321/secret_token_12345"
	maskedDiscord := MaskURL(discordURL)
	if strings.Contains(maskedDiscord, "secret_token_12345") {
		t.Fatalf("MaskURL leaked secret token: %s", maskedDiscord)
	}
	if !strings.HasPrefix(maskedDiscord, "https://discord.com/api/***") {
		t.Errorf("expected prefix https://discord.com/api/***, got %s", maskedDiscord)
	}

	// MaskChannelConfig tests
	cfg := &ChannelConfig{
		BotToken:   "123456789:ABCDefgh-Ijklmnop",
		ChatID:     "-100123456789",
		WebhookURL: "https://discord.com/api/webhooks/123/token_secret",
		Secret:     "super_secret_hmac_key",
	}

	tgMasked := MaskChannelConfig(ChannelTypeTelegram, cfg)
	if tgMasked["bot_token"] == cfg.BotToken || !strings.Contains(tgMasked["bot_token"], "***") {
		t.Errorf("telegram bot_token not masked properly: %v", tgMasked)
	}
	if tgMasked["chat_id"] != cfg.ChatID {
		t.Errorf("expected chat_id preserved, got: %s", tgMasked["chat_id"])
	}

	whMasked := MaskChannelConfig(ChannelTypeWebhook, cfg)
	if strings.Contains(whMasked["webhook_url"], "token_secret") || !strings.Contains(whMasked["webhook_url"], "***") {
		t.Errorf("webhook_url not masked: %v", whMasked)
	}
	if whMasked["secret"] == cfg.Secret || !strings.Contains(whMasked["secret"], "***") {
		t.Errorf("webhook secret not masked: %v", whMasked)
	}
}

func TestHMACSHA256Calculation(t *testing.T) {
	payload := []byte(`{"event":"alert.firing","status":"firing"}`)
	secret := "secret-key-12345"

	sig := ComputeHMACSHA256(payload, secret)
	if sig == "" {
		t.Fatalf("expected non-empty HMAC signature")
	}

	// Verify deterministic property
	sig2 := ComputeHMACSHA256(payload, secret)
	if sig != sig2 {
		t.Errorf("HMAC not deterministic: %s != %s", sig, sig2)
	}

	// Verify changes on different payload
	sigAlt := ComputeHMACSHA256([]byte(`{"event":"alert.resolved"}`), secret)
	if sig == sigAlt {
		t.Errorf("expected different HMAC for different payload")
	}
}

func TestTelegramNotifier_MockServer(t *testing.T) {
	var receivedBody map[string]interface{}
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/botmy-bot-token/sendMessage") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":100}}`))
	}))
	defer server.Close()

	notifier := NewTelegramNotifier(server.Client(), server.URL)
	cfg := &ChannelConfig{
		BotToken: "my-bot-token",
		ChatID:   "-100123456",
	}

	event := &AlertEvent{
		IncidentID: "inc_001",
		ProjectID:  "proj_prod",
		TargetType: "deployment",
		TargetID:   "dep_web",
		Title:      "Service Outage: dep_web",
		Message:    "Health check failed 3 consecutive times (Status: 502)",
		Status:     AlertStatusFiring,
		Timestamp:  time.Now().UTC(),
		StudioURL:  "https://cuan.dojo.cc/deployments/dep_web",
	}

	err := notifier.Send(context.Background(), cfg, event)
	if err != nil {
		t.Fatalf("TelegramNotifier.Send failed: %v", err)
	}

	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}
	if receivedBody["chat_id"] != "-100123456" {
		t.Errorf("expected chat_id -100123456, got %v", receivedBody["chat_id"])
	}
	text, ok := receivedBody["text"].(string)
	if !ok || !strings.Contains(text, "🚨 *[FIRING]") || !strings.Contains(text, "Open Studio Dashboard") {
		t.Errorf("expected formatted markdown with emoji and link, got: %s", text)
	}
}

func TestDiscordNotifier_MockServer(t *testing.T) {
	var receivedBody map[string]interface{}
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &receivedBody)

		w.WriteHeader(http.StatusNoContent) // Discord typical response
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.Client())
	cfg := &ChannelConfig{
		WebhookURL: server.URL,
	}

	// 1. Test Firing (Red embed)
	firingEvent := &AlertEvent{
		IncidentID: "inc_002",
		ProjectID:  "proj_prod",
		TargetType: "server",
		TargetID:   "srv_byos_1",
		Title:      "High Resource Usage: srv_byos_1",
		Message:    "CPU: 94.5%, RAM: 88.0%",
		Status:     AlertStatusFiring,
		Timestamp:  time.Now().UTC(),
		StudioURL:  "https://cuan.dojo.cc/servers/srv_byos_1",
	}

	err := notifier.Send(context.Background(), cfg, firingEvent)
	if err != nil {
		t.Fatalf("DiscordNotifier.Send firing failed: %v", err)
	}

	embeds, ok := receivedBody["embeds"].([]interface{})
	if !ok || len(embeds) == 0 {
		t.Fatalf("expected embeds array in discord payload, got: %+v", receivedBody)
	}
	firstEmbed := embeds[0].(map[string]interface{})
	if firstEmbed["color"] != float64(15158332) { // 0xE74C3C red
		t.Errorf("expected red color for firing alert, got %v", firstEmbed["color"])
	}

	// 2. Test Resolved (Green embed)
	resolvedEvent := &AlertEvent{
		IncidentID: "inc_002",
		ProjectID:  "proj_prod",
		TargetType: "server",
		TargetID:   "srv_byos_1",
		Title:      "Resource Usage Normal: srv_byos_1",
		Message:    "CPU: 45.0%, RAM: 50.0%",
		Status:     AlertStatusResolved,
		Timestamp:  time.Now().UTC(),
	}

	err = notifier.Send(context.Background(), cfg, resolvedEvent)
	if err != nil {
		t.Fatalf("DiscordNotifier.Send resolved failed: %v", err)
	}
	embedsResolved := receivedBody["embeds"].([]interface{})
	embedResolved := embedsResolved[0].(map[string]interface{})
	if embedResolved["color"] != float64(3066993) { // 0x2ECC71 green
		t.Errorf("expected green color for resolved alert, got %v", embedResolved["color"])
	}
}

func TestWebhookNotifier_HMACSignatureAndMockServer(t *testing.T) {
	secret := "test-secret-key-12345"
	var receivedSignature string
	var receivedBody []byte
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		receivedSignature = r.Header.Get("X-Cuan-Signature")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.Client())
	cfg := &ChannelConfig{
		WebhookURL: server.URL,
		Secret:     secret,
	}

	event := &AlertEvent{
		IncidentID: "inc_wh_1",
		ProjectID:  "proj_alpha",
		TargetType: "deployment",
		TargetID:   "dep_alpha",
		Title:      "Service Outage",
		Message:    "Gateway Timeout 504",
		Status:     AlertStatusFiring,
		Timestamp:  time.Now().UTC(),
		Data: map[string]interface{}{
			"status_code": 504,
		},
	}

	err := notifier.Send(context.Background(), cfg, event)
	if err != nil {
		t.Fatalf("WebhookNotifier.Send failed: %v", err)
	}

	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}

	// Verify HMAC-SHA256 signature
	expectedHMAC := "sha256=" + ComputeHMACSHA256(receivedBody, secret)
	if receivedSignature != expectedHMAC {
		t.Errorf("signature verification failed: got %s, expected %s", receivedSignature, expectedHMAC)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("failed to parse JSON body: %v", err)
	}
	if parsed["event"] != "alert.firing" {
		t.Errorf("expected event alert.firing, got %v", parsed["event"])
	}
	if parsed["incident_id"] != "inc_wh_1" {
		t.Errorf("expected incident_id inc_wh_1, got %v", parsed["incident_id"])
	}
}

func TestNotifier_RetryTransientErrors(t *testing.T) {
	var attempts int32
	// Fail twice with 500, succeed on 3rd attempt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curr := atomic.AddInt32(&attempts, 1)
		if curr <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("temporary upstream error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.Client())
	cfg := &ChannelConfig{
		WebhookURL: server.URL,
	}

	event := &AlertEvent{
		IncidentID: "inc_retry_test",
		Title:      "Test Retry",
		Status:     AlertStatusFiring,
		Timestamp:  time.Now().UTC(),
	}

	err := notifier.Send(context.Background(), cfg, event)
	if err != nil {
		t.Fatalf("expected retry to succeed on 3rd attempt, got error: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts (2 retries), got %d", attempts)
	}
}

func TestNotifier_NonRetryClientErrors(t *testing.T) {
	var attempts int32
	// Return 401 Unauthorized (client error)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid bot token"))
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.Client())
	cfg := &ChannelConfig{
		WebhookURL: server.URL,
	}

	event := &AlertEvent{
		IncidentID: "inc_client_err",
		Title:      "Test Client Error",
		Status:     AlertStatusFiring,
		Timestamp:  time.Now().UTC(),
	}

	err := notifier.Send(context.Background(), cfg, event)
	if err == nil {
		t.Fatalf("expected error on 401 unauthorized, got nil")
	}
	// Should fail immediately without wasting retries on 4xx
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected exactly 1 attempt on 4xx error, got %d", attempts)
	}
}

func TestAlertDispatcher_ThresholdEvaluation(t *testing.T) {
	repo := db.NewMemoryObservabilityRepository()
	dispatcher := NewAlertDispatcher(repo)

	// 1. HealthCheck evaluation
	// Healthy probe
	hcHealthy := &db.HealthCheck{
		DeploymentID:        "dep_1",
		Status:              StatusHealthy,
		ConsecutiveFailures: 0,
		LastStatusCode:      200,
		ResponseTimeMs:      45,
	}
	fire, resolve, _, _ := dispatcher.EvaluateHealth(hcHealthy, nil)
	if fire || !resolve {
		t.Errorf("expected healthy probe: fire=false, resolve=true, got fire=%v resolve=%v", fire, resolve)
	}

	// Degraded probe (failures = 1)
	hcDegraded := &db.HealthCheck{
		DeploymentID:        "dep_1",
		Status:              StatusDegraded,
		ConsecutiveFailures: 1,
		LastStatusCode:      500,
	}
	fire, resolve, _, _ = dispatcher.EvaluateHealth(hcDegraded, nil)
	if fire || resolve {
		t.Errorf("expected degraded with 1 failure to neither fire outage nor resolve, got fire=%v resolve=%v", fire, resolve)
	}

	// Outage probe (failures = 3)
	hcOutage := &db.HealthCheck{
		DeploymentID:        "dep_1",
		Status:              StatusUnhealthy,
		ConsecutiveFailures: 3,
		LastStatusCode:      504,
		ResponseTimeMs:      2500,
	}
	fire, resolve, title, _ := dispatcher.EvaluateHealth(hcOutage, nil)
	if !fire || resolve {
		t.Errorf("expected outage with >= 3 failures to fire: fire=%v resolve=%v", fire, resolve)
	}
	if !strings.Contains(title, "Service Outage") {
		t.Errorf("expected Service Outage title, got: %s", title)
	}

	// 2. Metric evaluation
	normalMet := &db.ServerMetric{
		CPUPercent:    45.0,
		MemoryUsedMB:  2048,
		MemoryTotalMB: 8192, // 25% RAM
	}
	fire, resolve, _, _ = dispatcher.EvaluateMetric(normalMet)
	if fire || !resolve {
		t.Errorf("expected normal metrics: fire=false, resolve=true")
	}

	// High CPU (>90%)
	highCPUMet := &db.ServerMetric{
		CPUPercent:    92.5,
		MemoryUsedMB:  2048,
		MemoryTotalMB: 8192,
	}
	fire, resolve, title, _ = dispatcher.EvaluateMetric(highCPUMet)
	if !fire || resolve {
		t.Errorf("expected high CPU to fire: fire=%v resolve=%v", fire, resolve)
	}
	if !strings.Contains(title, "High Resource Usage") {
		t.Errorf("expected High Resource Usage title, got: %s", title)
	}

	// High RAM (>90%)
	highRAMMet := &db.ServerMetric{
		CPUPercent:    35.0,
		MemoryUsedMB:  7800,
		MemoryTotalMB: 8192, // ~95.2% RAM
	}
	fire, resolve, title, _ = dispatcher.EvaluateMetric(highRAMMet)
	if !fire || resolve {
		t.Errorf("expected high RAM to fire: fire=%v resolve=%v", fire, resolve)
	}
	if !strings.Contains(title, "High Resource Usage") {
		t.Errorf("expected High Resource Usage title, got: %s", title)
	}
}

func TestAlertDispatcher_AntiSpamCooldownAndRecovery(t *testing.T) {
	repo := db.NewMemoryObservabilityRepository()
	ctx := context.Background()

	var dispatchCount int32
	var lastEvent *AlertEvent

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&dispatchCount, 1)
		b, _ := io.ReadAll(r.Body)
		var ev AlertEvent
		_ = json.Unmarshal(b, &ev)
		lastEvent = &ev
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// 10 minute cooldown
	dispatcher := NewAlertDispatcher(
		repo,
		WithCooldown(10*time.Minute),
		WithAlertHTTPClient(mockServer.Client()),
		WithProjectResolver(func(ctx context.Context, targetType, targetID string) string {
			return "proj_123"
		}),
	)

	// Create active webhook alert channel for proj_123
	rawCfg := &ChannelConfig{
		WebhookURL: mockServer.URL,
	}
	encCfg, err := dispatcher.EncryptChannelConfig(rawCfg)
	if err != nil {
		t.Fatalf("EncryptChannelConfig failed: %v", err)
	}

	channel := &db.AlertChannel{
		ID:              "alc_wh_1",
		ProjectID:       "proj_123",
		Type:            ChannelTypeWebhook,
		Name:            "Test Webhook",
		ConfigEncrypted: encCfg,
		IsActive:        true,
	}
	_ = repo.CreateAlertChannel(ctx, channel)

	// Step 1: Outage occurs (failures = 3) -> Should fire first alert
	hc := &db.HealthCheck{
		DeploymentID:        "dep_api",
		Status:              StatusUnhealthy,
		ConsecutiveFailures: 3,
		LastStatusCode:      503,
		ResponseTimeMs:      1200,
	}
	err = dispatcher.HandleHealthProbe(ctx, hc, &ProbeResult{StatusCode: 503, Error: "Service Unavailable"})
	if err != nil {
		t.Fatalf("HandleHealthProbe initial firing failed: %v", err)
	}

	if atomic.LoadInt32(&dispatchCount) != 1 {
		t.Fatalf("expected exactly 1 dispatch for initial outage, got %d", dispatchCount)
	}
	if lastEvent.Status != AlertStatusFiring {
		t.Errorf("expected firing status, got %s", lastEvent.Status)
	}

	// Step 2: Next minute, outage persists (failures = 4) -> Within 10m cooldown, MUST BE SUPPRESSED!
	hc.ConsecutiveFailures = 4
	err = dispatcher.HandleHealthProbe(ctx, hc, &ProbeResult{StatusCode: 503})
	if err != nil {
		t.Fatalf("HandleHealthProbe 2nd outage failed: %v", err)
	}
	if atomic.LoadInt32(&dispatchCount) != 1 {
		t.Fatalf("ANTI-SPAM FAILED: duplicate alert was sent during cooldown window! Total dispatches: %d", dispatchCount)
	}

	// Verify incident in repository
	incidents, _ := repo.ListActiveAlertIncidents(ctx, "proj_123")
	if len(incidents) != 1 {
		t.Errorf("expected 1 active incident in DB, got %d", len(incidents))
	}

	// Step 3: Recovery occurs (failures = 0, status = healthy) -> MUST fire RESOLVED immediately!
	hcHealthy := &db.HealthCheck{
		DeploymentID:        "dep_api",
		Status:              StatusHealthy,
		ConsecutiveFailures: 0,
		LastStatusCode:      200,
		ResponseTimeMs:      35,
	}
	err = dispatcher.HandleHealthProbe(ctx, hcHealthy, &ProbeResult{StatusCode: 200})
	if err != nil {
		t.Fatalf("HandleHealthProbe recovery failed: %v", err)
	}

	if atomic.LoadInt32(&dispatchCount) != 2 {
		t.Fatalf("expected recovery alert to be dispatched immediately (dispatchCount=2), got %d", dispatchCount)
	}
	if lastEvent.Status != AlertStatusResolved {
		t.Errorf("expected resolved status for recovery alert, got %s", lastEvent.Status)
	}

	// Verify active incidents resolved in repository
	activeAfter, _ := repo.ListActiveAlertIncidents(ctx, "proj_123")
	if len(activeAfter) != 0 {
		t.Errorf("expected 0 active incidents after recovery, got %d", len(activeAfter))
	}
}

func TestAlertDispatcher_TestChannelOneClick(t *testing.T) {
	repo := db.NewMemoryObservabilityRepository()
	ctx := context.Background()

	var testReceived bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var ev AlertEvent
		_ = json.Unmarshal(b, &ev)
		if strings.Contains(ev.Title, "Test Alert") {
			testReceived = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := NewAlertDispatcher(
		repo,
		WithAlertHTTPClient(server.Client()),
	)

	encCfg, _ := dispatcher.EncryptChannelConfig(&ChannelConfig{
		WebhookURL: server.URL,
	})

	channel := &db.AlertChannel{
		ID:              "alc_test_demo",
		ProjectID:       "proj_demo",
		Type:            ChannelTypeWebhook,
		Name:            "Demo Channel",
		ConfigEncrypted: encCfg,
		IsActive:        false, // Allowed to test even if inactive
	}

	err := dispatcher.TestChannel(ctx, channel)
	if err != nil {
		t.Fatalf("TestChannel failed: %v", err)
	}
	if !testReceived {
		t.Errorf("test demo alert was not received by mock server")
	}
}

func TestAlertDispatcher_EncryptionAtRest(t *testing.T) {
	vault := crypto.NewKeyVault("custom-test-encryption-key-for-alerts")
	dispatcher := NewAlertDispatcher(nil, WithAlertKeyVault(vault))

	original := &ChannelConfig{
		BotToken:   "bot-token-secret-99999",
		ChatID:     "chat-id-secret-88888",
		WebhookURL: "https://hooks.slack.com/services/T00/B00/secret_url",
		Secret:     "webhook-hmac-secret-77777",
	}

	encryptedB64, err := dispatcher.EncryptChannelConfig(original)
	if err != nil {
		t.Fatalf("EncryptChannelConfig failed: %v", err)
	}

	// Ensure ciphertext does not contain any plaintext secrets
	if strings.Contains(encryptedB64, "bot-token-secret") ||
		strings.Contains(encryptedB64, "chat-id-secret") ||
		strings.Contains(encryptedB64, "secret_url") ||
		strings.Contains(encryptedB64, "webhook-hmac-secret") {
		t.Fatalf("SECURITY VIOLATION: ciphertext contains plaintext secret!")
	}

	channel := &db.AlertChannel{
		ID:              "alc_enc_test",
		Type:            ChannelTypeTelegram,
		ConfigEncrypted: encryptedB64,
	}

	decrypted, err := dispatcher.DecryptChannelConfig(channel)
	if err != nil {
		t.Fatalf("DecryptChannelConfig failed: %v", err)
	}

	if decrypted.BotToken != original.BotToken {
		t.Errorf("decrypted BotToken mismatch: %s != %s", decrypted.BotToken, original.BotToken)
	}
	if decrypted.ChatID != original.ChatID {
		t.Errorf("decrypted ChatID mismatch: %s != %s", decrypted.ChatID, original.ChatID)
	}
}

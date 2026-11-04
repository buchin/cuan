package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestValidateHealthCheckURL(t *testing.T) {
	validURLs := []string{
		"http://localhost:8080/healthz",
		"https://app.clientbrand.com",
		"https://cuan.app/api/v1/health",
		"http://127.0.0.1:3000/ping",
		"https://sub.domain.co.id/status?check=1",
	}

	for _, u := range validURLs {
		if err := ValidateHealthCheckURL(u); err != nil {
			t.Errorf("expected URL '%s' to be valid, got: %v", u, err)
		}
	}

	invalidURLs := []struct {
		url    string
		reason string
	}{
		{"", "empty url"},
		{"   ", "whitespace only"},
		{"ftp://example.com/file", "unsupported scheme"},
		{"example.com/healthz", "missing scheme"},
		{"http://", "missing host"},
		{"://invalid", "malformed uri"},
	}

	for _, tc := range invalidURLs {
		if err := ValidateHealthCheckURL(tc.url); err == nil {
			t.Errorf("expected URL '%s' (%s) to fail validation, got nil", tc.url, tc.reason)
		} else if !errors.Is(err, ErrInvalidHealthCheckURL) {
			t.Errorf("expected ErrInvalidHealthCheckURL for '%s', got: %v", tc.url, err)
		}
	}
}

func TestValidateChannelType(t *testing.T) {
	valid := []string{"telegram", "discord", "webhook", "Telegram", "DISCORD", " Webhook "}
	for _, ch := range valid {
		if err := ValidateChannelType(ch); err != nil {
			t.Errorf("expected channel '%s' to be valid, got: %v", ch, err)
		}
	}

	invalid := []string{"", "email", "sms", "slack", "pagerduty", "unknown"}
	for _, ch := range invalid {
		if err := ValidateChannelType(ch); err == nil {
			t.Errorf("expected channel '%s' to fail validation, got nil", ch)
		} else if !errors.Is(err, ErrInvalidChannelType) {
			t.Errorf("expected ErrInvalidChannelType for '%s', got: %v", ch, err)
		}
	}
}

func TestAlertChannel_JSONMaskingSecurity(t *testing.T) {
	now := time.Now().UTC()
	ch := AlertChannel{
		ID:              "alc_secret_test",
		ProjectID:       "proj_123",
		Type:            "telegram",
		Name:            "Ops Telegram",
		ConfigEncrypted: "ENC_AES_GCM_SUPER_SECRET_BOT_TOKEN_12345",
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	b, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	rawJSON := string(b)

	if strings.Contains(rawJSON, "SUPER_SECRET_BOT_TOKEN") {
		t.Fatalf("CRITICAL SECURITY FLAW: AlertChannel JSON serialization exposed ConfigEncrypted!")
	}
	if strings.Contains(rawJSON, "config_encrypted") {
		t.Fatalf("CRITICAL SECURITY FLAW: AlertChannel JSON contains config_encrypted field key!")
	}

	dto := ch.ToDTO()
	dtoBytes, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("json.Marshal(dto) failed: %v", err)
	}
	dtoJSON := string(dtoBytes)
	if strings.Contains(dtoJSON, "SUPER_SECRET_BOT_TOKEN") || strings.Contains(dtoJSON, "config_encrypted") {
		t.Fatalf("CRITICAL SECURITY FLAW: AlertChannelDTO exposed secret credentials!")
	}
}

func TestServerMetric_PercentageCalculations(t *testing.T) {
	srvID := "srv_1"
	depID := "dep_1"
	metric := ServerMetric{
		ID:            "met_calc_test",
		ServerID:      &srvID,
		DeploymentID:  &depID,
		CPUPercent:    45.5,
		MemoryUsedMB:  2048,
		MemoryTotalMB: 8192,
		DiskUsedGB:    25.0,
		DiskTotalGB:   100.0,
		RecordedAt:    time.Now().UTC(),
	}

	dto := metric.ToDTO()
	if dto.ServerID != "srv_1" || dto.DeploymentID != "dep_1" {
		t.Errorf("dto IDs mismatch: srv=%s dep=%s", dto.ServerID, dto.DeploymentID)
	}
	if dto.CPUPercent != 45.5 {
		t.Errorf("expected CPU 45.5, got %f", dto.CPUPercent)
	}
	if dto.MemoryPercent != 25.0 {
		t.Errorf("expected MemoryPercent 25.0, got %f", dto.MemoryPercent)
	}
	if dto.DiskPercent != 25.0 {
		t.Errorf("expected DiskPercent 25.0, got %f", dto.DiskPercent)
	}
}

func TestMemoryObservabilityRepository_HealthChecks(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryObservabilityRepository()

	// 1. Validation failure: empty deployment_id
	err := repo.CreateHealthCheck(ctx, &HealthCheck{
		URL: "http://example.com/healthz",
	})
	if !errors.Is(err, ErrEmptyDeploymentID) {
		t.Errorf("expected ErrEmptyDeploymentID, got: %v", err)
	}

	// 2. Validation failure: invalid URL
	err = repo.CreateHealthCheck(ctx, &HealthCheck{
		DeploymentID: "dep_1",
		URL:          "ftp://bad.url",
	})
	if !errors.Is(err, ErrInvalidHealthCheckURL) {
		t.Errorf("expected ErrInvalidHealthCheckURL, got: %v", err)
	}

	// 3. Create valid health check
	hc1 := &HealthCheck{
		ID:           "hc_001",
		DeploymentID: "dep_1",
		URL:          "https://app1.example.com/healthz",
		Status:       "healthy",
	}
	if err := repo.CreateHealthCheck(ctx, hc1); err != nil {
		t.Fatalf("CreateHealthCheck failed: %v", err)
	}

	// 4. Duplicate deployment_id check
	duplicate := &HealthCheck{
		ID:           "hc_002",
		DeploymentID: "dep_1",
		URL:          "https://app1-alt.example.com/healthz",
	}
	err = repo.CreateHealthCheck(ctx, duplicate)
	if !errors.Is(err, ErrDuplicateHealthCheck) {
		t.Errorf("expected ErrDuplicateHealthCheck, got: %v", err)
	}

	// 5. GetHealthCheckByID
	fetched, err := repo.GetHealthCheckByID(ctx, "hc_001")
	if err != nil {
		t.Fatalf("GetHealthCheckByID failed: %v", err)
	}
	if fetched.DeploymentID != "dep_1" || fetched.URL != "https://app1.example.com/healthz" {
		t.Errorf("fetched health check mismatch: %+v", fetched)
	}

	// Get non-existent
	_, err = repo.GetHealthCheckByID(ctx, "hc_missing")
	if !errors.Is(err, ErrHealthCheckNotFound) {
		t.Errorf("expected ErrHealthCheckNotFound, got: %v", err)
	}

	// 6. GetHealthCheckByDeploymentID
	fetchedByDep, err := repo.GetHealthCheckByDeploymentID(ctx, "dep_1")
	if err != nil {
		t.Fatalf("GetHealthCheckByDeploymentID failed: %v", err)
	}
	if fetchedByDep.ID != "hc_001" {
		t.Errorf("expected hc_001, got: %s", fetchedByDep.ID)
	}

	_, err = repo.GetHealthCheckByDeploymentID(ctx, "dep_unknown")
	if !errors.Is(err, ErrHealthCheckNotFound) {
		t.Errorf("expected ErrHealthCheckNotFound, got: %v", err)
	}

	// 7. UpdateHealthCheckResult
	checkedAt := time.Now().UTC()
	err = repo.UpdateHealthCheckResult(ctx, "hc_001", "degraded", 502, 1250, 1, checkedAt)
	if err != nil {
		t.Fatalf("UpdateHealthCheckResult failed: %v", err)
	}

	updated, _ := repo.GetHealthCheckByID(ctx, "hc_001")
	if updated.Status != "degraded" || updated.LastStatusCode != 502 || updated.ResponseTimeMs != 1250 || updated.ConsecutiveFailures != 1 {
		t.Errorf("updated result mismatch: %+v", updated)
	}
	if updated.LastCheckedAt == nil || !updated.LastCheckedAt.Equal(checkedAt) {
		t.Errorf("expected lastCheckedAt=%v, got %v", checkedAt, updated.LastCheckedAt)
	}

	// Update non-existent result
	err = repo.UpdateHealthCheckResult(ctx, "hc_missing", "healthy", 200, 100, 0, checkedAt)
	if !errors.Is(err, ErrHealthCheckNotFound) {
		t.Errorf("expected ErrHealthCheckNotFound, got: %v", err)
	}

	// 8. UpdateHealthCheck
	updated.URL = "https://app1.example.com/health"
	if err := repo.UpdateHealthCheck(ctx, updated); err != nil {
		t.Fatalf("UpdateHealthCheck failed: %v", err)
	}
	fetchedUpdated, _ := repo.GetHealthCheckByID(ctx, "hc_001")
	if fetchedUpdated.URL != "https://app1.example.com/health" {
		t.Errorf("expected updated URL, got: %s", fetchedUpdated.URL)
	}

	// 9. Add another health check & list
	hc2 := &HealthCheck{
		DeploymentID: "dep_2",
		URL:          "https://app2.example.com/healthz",
	}
	if err := repo.CreateHealthCheck(ctx, hc2); err != nil {
		t.Fatalf("CreateHealthCheck hc2 failed: %v", err)
	}
	if !strings.HasPrefix(hc2.ID, "hc_") {
		t.Errorf("expected auto-generated hc_ ID, got %s", hc2.ID)
	}

	list, err := repo.ListHealthChecks(ctx)
	if err != nil {
		t.Fatalf("ListHealthChecks failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 health checks, got %d", len(list))
	}

	// 10. DeleteHealthCheck
	if err := repo.DeleteHealthCheck(ctx, "hc_001"); err != nil {
		t.Fatalf("DeleteHealthCheck failed: %v", err)
	}
	_, err = repo.GetHealthCheckByID(ctx, "hc_001")
	if !errors.Is(err, ErrHealthCheckNotFound) {
		t.Errorf("expected ErrHealthCheckNotFound after delete, got %v", err)
	}

	// Delete already deleted
	err = repo.DeleteHealthCheck(ctx, "hc_001")
	if !errors.Is(err, ErrHealthCheckNotFound) {
		t.Errorf("expected ErrHealthCheckNotFound on re-delete, got %v", err)
	}
}

func TestMemoryObservabilityRepository_Metrics(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryObservabilityRepository()

	srv1 := "srv_alpha"
	srv2 := "srv_beta"
	dep1 := "dep_web"

	now := time.Now().UTC()

	// 1. Record metrics
	m1 := &ServerMetric{
		ServerID:      &srv1,
		DeploymentID:  &dep1,
		CPUPercent:    25.5,
		MemoryUsedMB:  1024,
		MemoryTotalMB: 4096,
		DiskUsedGB:    10.0,
		DiskTotalGB:   50.0,
		RecordedAt:    now.Add(-2 * time.Minute),
	}
	if err := repo.RecordServerMetric(ctx, m1); err != nil {
		t.Fatalf("RecordServerMetric m1 failed: %v", err)
	}
	if !strings.HasPrefix(m1.ID, "met_") {
		t.Errorf("expected auto-generated met_ ID, got: %s", m1.ID)
	}

	m2 := &ServerMetric{
		ServerID:      &srv1,
		DeploymentID:  &dep1,
		CPUPercent:    35.0,
		MemoryUsedMB:  1500,
		MemoryTotalMB: 4096,
		DiskUsedGB:    11.0,
		DiskTotalGB:   50.0,
		RecordedAt:    now.Add(-1 * time.Minute),
	}
	if err := repo.RecordServerMetric(ctx, m2); err != nil {
		t.Fatalf("RecordServerMetric m2 failed: %v", err)
	}

	m3 := &ServerMetric{
		ServerID:      &srv2,
		CPUPercent:    75.0,
		MemoryUsedMB:  3000,
		MemoryTotalMB: 4096,
		DiskUsedGB:    30.0,
		DiskTotalGB:   50.0,
		RecordedAt:    now,
	}
	if err := repo.RecordServerMetric(ctx, m3); err != nil {
		t.Fatalf("RecordServerMetric m3 failed: %v", err)
	}

	// 2. ListServerMetrics
	srv1Metrics, err := repo.ListServerMetrics(ctx, srv1, 10)
	if err != nil {
		t.Fatalf("ListServerMetrics srv1 failed: %v", err)
	}
	if len(srv1Metrics) != 2 {
		t.Fatalf("expected 2 metrics for srv1, got %d", len(srv1Metrics))
	}
	// Verify descending order (latest first)
	if srv1Metrics[0].CPUPercent != 35.0 {
		t.Errorf("expected latest metric first (35.0), got %f", srv1Metrics[0].CPUPercent)
	}

	// 3. ListDeploymentMetrics
	depMetrics, err := repo.ListDeploymentMetrics(ctx, dep1, 1)
	if err != nil {
		t.Fatalf("ListDeploymentMetrics failed: %v", err)
	}
	if len(depMetrics) != 1 {
		t.Fatalf("expected 1 metric with limit 1, got %d", len(depMetrics))
	}
	if depMetrics[0].CPUPercent != 35.0 {
		t.Errorf("expected latest metric for deployment (35.0), got %f", depMetrics[0].CPUPercent)
	}

	// 4. GetLatestServerMetric
	latestSrv1, err := repo.GetLatestServerMetric(ctx, srv1)
	if err != nil {
		t.Fatalf("GetLatestServerMetric failed: %v", err)
	}
	if latestSrv1.CPUPercent != 35.0 {
		t.Errorf("expected CPU 35.0, got %f", latestSrv1.CPUPercent)
	}

	// Unknown server returns error
	_, err = repo.GetLatestServerMetric(ctx, "srv_unknown")
	if err == nil {
		t.Errorf("expected error for unknown server, got nil")
	}
}

func TestMemoryObservabilityRepository_AlertChannels(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryObservabilityRepository()

	// 1. Validation failure: empty project_id
	err := repo.CreateAlertChannel(ctx, &AlertChannel{
		Type: "telegram",
		Name: "Telegram Alerts",
	})
	if !errors.Is(err, ErrEmptyProjectID) {
		t.Errorf("expected ErrEmptyProjectID, got %v", err)
	}

	// 2. Validation failure: empty name
	err = repo.CreateAlertChannel(ctx, &AlertChannel{
		ProjectID: "proj_1",
		Type:      "telegram",
	})
	if !errors.Is(err, ErrEmptyChannelName) {
		t.Errorf("expected ErrEmptyChannelName, got %v", err)
	}

	// 3. Validation failure: invalid type
	err = repo.CreateAlertChannel(ctx, &AlertChannel{
		ProjectID: "proj_1",
		Name:      "Email Channel",
		Type:      "email",
	})
	if !errors.Is(err, ErrInvalidChannelType) {
		t.Errorf("expected ErrInvalidChannelType, got %v", err)
	}

	// 4. Create valid channels
	ch1 := &AlertChannel{
		ID:              "alc_001",
		ProjectID:       "proj_1",
		Type:            "Telegram", // test case normalization
		Name:            "Telegram Ops",
		ConfigEncrypted: "enc_token_123",
		IsActive:        true,
	}
	if err := repo.CreateAlertChannel(ctx, ch1); err != nil {
		t.Fatalf("CreateAlertChannel ch1 failed: %v", err)
	}
	if ch1.Type != "telegram" {
		t.Errorf("expected normalized type 'telegram', got '%s'", ch1.Type)
	}

	ch2 := &AlertChannel{
		ProjectID:       "proj_1",
		Type:            "discord",
		Name:            "Discord Alerts",
		ConfigEncrypted: "enc_webhook_456",
		IsActive:        true,
	}
	if err := repo.CreateAlertChannel(ctx, ch2); err != nil {
		t.Fatalf("CreateAlertChannel ch2 failed: %v", err)
	}
	if !strings.HasPrefix(ch2.ID, "alc_") {
		t.Errorf("expected auto-generated alc_ ID, got %s", ch2.ID)
	}

	// 5. GetAlertChannelByID
	fetched, err := repo.GetAlertChannelByID(ctx, "alc_001")
	if err != nil {
		t.Fatalf("GetAlertChannelByID failed: %v", err)
	}
	if fetched.Name != "Telegram Ops" || fetched.ConfigEncrypted != "enc_token_123" {
		t.Errorf("fetched channel mismatch: %+v", fetched)
	}

	_, err = repo.GetAlertChannelByID(ctx, "alc_missing")
	if !errors.Is(err, ErrAlertChannelNotFound) {
		t.Errorf("expected ErrAlertChannelNotFound, got %v", err)
	}

	// 6. ListAlertChannelsByProjectID
	channels, err := repo.ListAlertChannelsByProjectID(ctx, "proj_1")
	if err != nil {
		t.Fatalf("ListAlertChannelsByProjectID failed: %v", err)
	}
	if len(channels) != 2 {
		t.Errorf("expected 2 channels for proj_1, got %d", len(channels))
	}

	emptyList, err := repo.ListAlertChannelsByProjectID(ctx, "proj_empty")
	if err != nil {
		t.Fatalf("ListAlertChannelsByProjectID empty failed: %v", err)
	}
	if len(emptyList) != 0 {
		t.Errorf("expected 0 channels for proj_empty, got %d", len(emptyList))
	}

	// 7. UpdateAlertChannel
	ch1.Name = "Telegram Production Alerts"
	ch1.IsActive = false
	if err := repo.UpdateAlertChannel(ctx, ch1); err != nil {
		t.Fatalf("UpdateAlertChannel failed: %v", err)
	}
	updated, _ := repo.GetAlertChannelByID(ctx, "alc_001")
	if updated.Name != "Telegram Production Alerts" || updated.IsActive != false {
		t.Errorf("updated channel mismatch: %+v", updated)
	}

	// 8. DeleteAlertChannel
	if err := repo.DeleteAlertChannel(ctx, "alc_001"); err != nil {
		t.Fatalf("DeleteAlertChannel failed: %v", err)
	}
	_, err = repo.GetAlertChannelByID(ctx, "alc_001")
	if !errors.Is(err, ErrAlertChannelNotFound) {
		t.Errorf("expected ErrAlertChannelNotFound after delete, got %v", err)
	}
}

func TestMemoryObservabilityRepository_AlertIncidents(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryObservabilityRepository()

	// 1. Validation failure: empty project_id
	err := repo.CreateAlertIncident(ctx, &AlertIncident{
		Title: "Outage",
	})
	if !errors.Is(err, ErrEmptyProjectID) {
		t.Errorf("expected ErrEmptyProjectID, got %v", err)
	}

	now := time.Now().UTC()

	// 2. Create Incidents
	inc1 := &AlertIncident{
		ID:          "inc_001",
		ProjectID:   "proj_prod",
		TargetType:  "deployment",
		TargetID:    "dep_123",
		Title:       "Service Outage: Gateway Timeout 504",
		Message:     "Health check probe failed 3 consecutive times",
		Status:      "firing",
		TriggeredAt: now.Add(-10 * time.Minute),
	}
	if err := repo.CreateAlertIncident(ctx, inc1); err != nil {
		t.Fatalf("CreateAlertIncident inc1 failed: %v", err)
	}

	inc2 := &AlertIncident{
		ProjectID:   "proj_prod",
		TargetType:  "server",
		TargetID:    "srv_456",
		Title:       "High Memory Usage Alert",
		Message:     "RAM usage reached 95%",
		TriggeredAt: now.Add(-5 * time.Minute),
	}
	if err := repo.CreateAlertIncident(ctx, inc2); err != nil {
		t.Fatalf("CreateAlertIncident inc2 failed: %v", err)
	}
	if inc2.Status != "firing" {
		t.Errorf("expected default firing status, got %s", inc2.Status)
	}
	if !strings.HasPrefix(inc2.ID, "inc_") {
		t.Errorf("expected auto-generated inc_ ID, got %s", inc2.ID)
	}

	// 3. GetAlertIncidentByID
	fetched, err := repo.GetAlertIncidentByID(ctx, "inc_001")
	if err != nil {
		t.Fatalf("GetAlertIncidentByID failed: %v", err)
	}
	if fetched.TargetID != "dep_123" || fetched.Status != "firing" {
		t.Errorf("fetched incident mismatch: %+v", fetched)
	}

	_, err = repo.GetAlertIncidentByID(ctx, "inc_missing")
	if !errors.Is(err, ErrAlertIncidentNotFound) {
		t.Errorf("expected ErrAlertIncidentNotFound, got %v", err)
	}

	// 4. ListActiveAlertIncidents
	active, err := repo.ListActiveAlertIncidents(ctx, "proj_prod")
	if err != nil {
		t.Fatalf("ListActiveAlertIncidents failed: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active incidents, got %d", len(active))
	}

	// 5. ResolveAlertIncident
	resolvedAt := time.Now().UTC()
	if err := repo.ResolveAlertIncident(ctx, "inc_001", resolvedAt); err != nil {
		t.Fatalf("ResolveAlertIncident failed: %v", err)
	}

	resolved, _ := repo.GetAlertIncidentByID(ctx, "inc_001")
	if resolved.Status != "resolved" {
		t.Errorf("expected status resolved, got %s", resolved.Status)
	}
	if resolved.ResolvedAt == nil || !resolved.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("expected ResolvedAt=%v, got %v", resolvedAt, resolved.ResolvedAt)
	}

	// Active list should now only contain 1 incident
	activeAfter, _ := repo.ListActiveAlertIncidents(ctx, "proj_prod")
	if len(activeAfter) != 1 {
		t.Errorf("expected 1 active incident after resolving one, got %d", len(activeAfter))
	}
	if activeAfter[0].ID != inc2.ID {
		t.Errorf("expected remaining active incident to be inc2, got %s", activeAfter[0].ID)
	}

	// 6. ListAlertIncidentsByProjectID (includes both firing and resolved)
	allIncidents, err := repo.ListAlertIncidentsByProjectID(ctx, "proj_prod", 10)
	if err != nil {
		t.Fatalf("ListAlertIncidentsByProjectID failed: %v", err)
	}
	if len(allIncidents) != 2 {
		t.Errorf("expected 2 incidents in total history, got %d", len(allIncidents))
	}

	// Resolve non-existent incident
	err = repo.ResolveAlertIncident(ctx, "inc_missing", resolvedAt)
	if !errors.Is(err, ErrAlertIncidentNotFound) {
		t.Errorf("expected ErrAlertIncidentNotFound on resolving non-existent incident, got %v", err)
	}
}

func TestMemoryObservabilityRepository_ConcurrencyThreadSafety(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryObservabilityRepository()

	workers := 20
	iterations := 50
	if testing.Short() {
		workers = 5
		iterations = 10
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		workerID := w
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				depID := fmt.Sprintf("dep_%d_%d", workerID, i)
				hc := &HealthCheck{
					DeploymentID: depID,
					URL:          fmt.Sprintf("https://worker-%d-app-%d.cuan.app/healthz", workerID, i),
					Status:       "healthy",
				}
				_ = repo.CreateHealthCheck(ctx, hc)

				srvID := fmt.Sprintf("srv_%d", workerID)
				met := &ServerMetric{
					ServerID:      &srvID,
					CPUPercent:    float64(i),
					MemoryUsedMB:  int64(i * 10),
					MemoryTotalMB: 4096,
				}
				_ = repo.RecordServerMetric(ctx, met)

				ch := &AlertChannel{
					ProjectID: fmt.Sprintf("proj_%d", workerID),
					Type:      "webhook",
					Name:      fmt.Sprintf("Webhook %d-%d", workerID, i),
				}
				_ = repo.CreateAlertChannel(ctx, ch)

				inc := &AlertIncident{
					ProjectID:  fmt.Sprintf("proj_%d", workerID),
					TargetType: "deployment",
					TargetID:   depID,
					Title:      "Worker alert",
					Message:    "Simulated test",
				}
				_ = repo.CreateAlertIncident(ctx, inc)

				// Concurrent reads
				_, _ = repo.ListHealthChecks(ctx)
				_, _ = repo.ListServerMetrics(ctx, srvID, 5)
				_, _ = repo.ListAlertChannelsByProjectID(ctx, fmt.Sprintf("proj_%d", workerID))
				_, _ = repo.ListActiveAlertIncidents(ctx, fmt.Sprintf("proj_%d", workerID))
			}
		}()
	}

	wg.Wait()

	allHC, _ := repo.ListHealthChecks(ctx)
	if len(allHC) != workers*iterations {
		t.Errorf("expected %d health checks, got %d", workers*iterations, len(allHC))
	}
}

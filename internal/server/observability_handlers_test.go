package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/observability"
)

func setupObservabilityTestServer(t *testing.T) (*Server, *deployer.MemoryStore, *cuanDB.MemoryObservabilityRepository, *cuanDB.MemoryServerRepository) {
	cfg := &config.Config{
		AppMode:     "developer",
		Environment: "test",
		BaseDomain:  "cuan.test",
	}

	depStore := deployer.NewMemoryStore()
	obsRepo := cuanDB.NewMemoryObservabilityRepository()
	serverRepo := cuanDB.NewMemoryServerRepository()

	caddyCfg := deployer.DefaultCaddyConfig()
	caddyRouter := deployer.NewCaddyRouter(caddyCfg)
	portPool := deployer.NewPortPool()
	depStorage, _ := deployer.NewStorageEngine(t.TempDir())
	localDriver := deployer.NewLocalDriverWithRouter(depStorage, depStore, portPool, caddyRouter)
	driverFactory := deployer.NewDriverFactory(localDriver, nil, depStore)

	srv := NewServer(cfg, nil)
	srv.SetDeployerDriver(driverFactory)
	srv.SetObservabilityRepository(obsRepo)
	srv.SetServerRepository(serverRepo)

	// Configure mock metrics reader and mock SSH runner for deterministic testing
	mockReader := &observability.MockLocalMetricsReader{
		Metrics: &observability.SystemMetrics{
			CPUPercent:    15.0,
			MemoryUsedMB:  1024,
			MemoryTotalMB: 4096,
			DiskUsedGB:    20.0,
			DiskTotalGB:   100.0,
		},
	}
	mockSSH := &observability.MockSSHRunner{
		OutputFunc: func(ctx context.Context, server *cuanDB.Server, command string) (string, string, error) {
			sampleOutput := `===CUAN_TELEMETRY===
---CPU---
%Cpu(s): 14.2 us,  3.0 sy,  0.0 ni, 82.8 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
---MEM---
Mem:           8192        2048        5120         100        1024        6144
---DISK---
Filesystem     1M-blocks      Used Available Use% Mounted on
/dev/root         102400     40960     61440  40% /
===END===`
			return sampleOutput, "", nil
		},
	}

	metricsCollector := observability.NewMetricsCollector(
		obsRepo,
		observability.WithLocalReader(mockReader),
		observability.WithSSHRunner(mockSSH),
		observability.WithServerRepo(serverRepo),
	)
	srv.SetObservabilityMetricsCollector(metricsCollector)

	healthPoller := observability.NewHealthPoller(obsRepo)
	srv.SetObservabilityHealthPoller(healthPoller)

	dispatcher := observability.NewAlertDispatcher(
		obsRepo,
		observability.WithAlertKeyVault(srv.KeyVault()),
	)
	srv.SetObservabilityAlertDispatcher(dispatcher)

	return srv, depStore, obsRepo, serverRepo
}

func TestObservabilityEndpoints_DeploymentHealth(t *testing.T) {
	srv, depStore, obsRepo, _ := setupObservabilityTestServer(t)
	ctx := context.Background()

	// 1. Test 404 for non-existent deployment
	req404 := httptest.NewRequest(http.MethodGet, "/api/deployments/nonexistent-dep/health", nil)
	rec404 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent deployment, got %d", rec404.Code)
	}

	// 2. Create mock deployment
	mockTargetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer mockTargetServer.Close()

	dep := &deployer.Deployment{
		ID:           "dep-test-1",
		ProjectName:  "Test Project",
		Slug:         "test-proj",
		SourceType:   "zip",
		SourceRef:    "test.zip",
		Runtime:      "static",
		Status:       "running",
		InternalPort: 8080,
		Domain:       mockTargetServer.URL,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = depStore.CreateDeployment(ctx, dep)

	// Create health check record in obsRepo
	hc := &cuanDB.HealthCheck{
		DeploymentID:        dep.ID,
		URL:                 mockTargetServer.URL,
		Status:              observability.StatusHealthy,
		LastStatusCode:      200,
		ResponseTimeMs:      45,
		ConsecutiveFailures: 0,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	_ = obsRepo.CreateHealthCheck(ctx, hc)

	// 3. Test GET /api/deployments/{id}/health
	reqGet := httptest.NewRequest(http.MethodGet, "/api/deployments/"+dep.ID+"/health", nil)
	recGet := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("GET /api/deployments/{id}/health failed with %d: %s", recGet.Code, recGet.Body.String())
	}

	var resp DeploymentHealthResponse
	if err := json.Unmarshal(recGet.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal health response failed: %v", err)
	}

	if resp.DeploymentID != dep.ID {
		t.Errorf("expected deployment_id %s, got %s", dep.ID, resp.DeploymentID)
	}
	if resp.URL != mockTargetServer.URL {
		t.Errorf("expected url %s, got %s", mockTargetServer.URL, resp.URL)
	}
	if resp.Status != observability.StatusHealthy {
		t.Errorf("expected status healthy, got %s", resp.Status)
	}
	if resp.UptimePercent != 100.0 {
		t.Errorf("expected 100.0%% uptime, got %f", resp.UptimePercent)
	}

	// 4. Test POST /api/deployments/{id}/health/check-now
	reqCheckNow := httptest.NewRequest(http.MethodPost, "/api/deployments/"+dep.ID+"/health/check-now", nil)
	recCheckNow := httptest.NewRecorder()
	srv.Router().ServeHTTP(recCheckNow, reqCheckNow)

	if recCheckNow.Code != http.StatusOK {
		t.Fatalf("POST /api/deployments/{id}/health/check-now failed with %d: %s", recCheckNow.Code, recCheckNow.Body.String())
	}

	var checkNowResp map[string]interface{}
	if err := json.Unmarshal(recCheckNow.Body.Bytes(), &checkNowResp); err != nil {
		t.Fatalf("unmarshal check-now response failed: %v", err)
	}

	if checkNowResp["success"] != true {
		t.Errorf("expected success true, got %v", checkNowResp["success"])
	}
	if checkNowResp["status"] != observability.StatusHealthy {
		t.Errorf("expected status healthy, got %v", checkNowResp["status"])
	}
	if checkNowResp["last_status_code"] != float64(200) {
		t.Errorf("expected last_status_code 200, got %v", checkNowResp["last_status_code"])
	}

	// 5. Test v1 route aliases
	reqV1Get := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/"+dep.ID+"/health", nil)
	recV1Get := httptest.NewRecorder()
	srv.Router().ServeHTTP(recV1Get, reqV1Get)
	if recV1Get.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/deployments/{id}/health failed with %d", recV1Get.Code)
	}

	reqV1CheckNow := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/"+dep.ID+"/health/check-now", nil)
	recV1CheckNow := httptest.NewRecorder()
	srv.Router().ServeHTTP(recV1CheckNow, reqV1CheckNow)
	if recV1CheckNow.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/deployments/{id}/health/check-now failed with %d", recV1CheckNow.Code)
	}
}

func TestObservabilityEndpoints_ServerMetrics(t *testing.T) {
	srv, _, obsRepo, serverRepo := setupObservabilityTestServer(t)
	ctx := context.Background()

	// 1. Test 404 for non-existent server
	req404 := httptest.NewRequest(http.MethodGet, "/api/servers/nonexistent-server/metrics", nil)
	rec404 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent server, got %d", rec404.Code)
	}

	// 2. Create mock BYOS server
	mockServer := &cuanDB.Server{
		ID:           "srv-byos-1",
		Name:         "Dedicated Staging VPS",
		Host:         "192.168.1.100",
		Port:         22,
		Username:     "ubuntu",
		Status:       "active",
		EncryptedKey: "mock-key",
	}
	_ = serverRepo.Create(ctx, mockServer)

	// Pre-populate some metrics
	srvID := mockServer.ID
	m1 := &cuanDB.ServerMetric{
		ServerID:      &srvID,
		CPUPercent:    12.5,
		MemoryUsedMB:  2048,
		MemoryTotalMB: 8192,
		DiskUsedGB:    30.0,
		DiskTotalGB:   100.0,
		RecordedAt:    time.Now().Add(-10 * time.Minute),
	}
	m2 := &cuanDB.ServerMetric{
		ServerID:      &srvID,
		CPUPercent:    17.2,
		MemoryUsedMB:  2560,
		MemoryTotalMB: 8192,
		DiskUsedGB:    32.0,
		DiskTotalGB:   100.0,
		RecordedAt:    time.Now().Add(-5 * time.Minute),
	}
	_ = obsRepo.RecordServerMetric(ctx, m1)
	_ = obsRepo.RecordServerMetric(ctx, m2)

	// 3. Test GET /api/servers/{id}/metrics
	reqMetrics := httptest.NewRequest(http.MethodGet, "/api/servers/"+mockServer.ID+"/metrics?limit=10", nil)
	recMetrics := httptest.NewRecorder()
	srv.Router().ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Fatalf("GET /api/servers/{id}/metrics failed with %d: %s", recMetrics.Code, recMetrics.Body.String())
	}

	var res struct {
		ServerID string                   `json:"server_id"`
		Latest   *cuanDB.ServerMetricDTO  `json:"latest"`
		Metrics  []cuanDB.ServerMetricDTO `json:"metrics"`
		Count    int                      `json:"count"`
	}
	if err := json.Unmarshal(recMetrics.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal metrics response failed: %v", err)
	}

	if res.ServerID != mockServer.ID {
		t.Errorf("expected server_id %s, got %s", mockServer.ID, res.ServerID)
	}
	if res.Count != 2 {
		t.Errorf("expected count 2, got %d", res.Count)
	}
	if res.Latest == nil {
		t.Fatalf("expected non-nil latest metric")
	}
	if res.Latest.CPUPercent != 17.2 {
		t.Errorf("expected latest CPU 17.2, got %f", res.Latest.CPUPercent)
	}

	// 4. Test GET /api/servers/{id}/metrics?collect=true
	reqCollect := httptest.NewRequest(http.MethodGet, "/api/servers/"+mockServer.ID+"/metrics?collect=true", nil)
	recCollect := httptest.NewRecorder()
	srv.Router().ServeHTTP(recCollect, reqCollect)

	if recCollect.Code != http.StatusOK {
		t.Fatalf("GET /api/servers/{id}/metrics?collect=true failed with %d: %s", recCollect.Code, recCollect.Body.String())
	}

	var resAfterCollect struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(recCollect.Body.Bytes(), &resAfterCollect)
	if resAfterCollect.Count < 3 {
		t.Errorf("expected at least 3 metrics after on-demand collection, got %d", resAfterCollect.Count)
	}

	// 5. Test v1 route alias
	reqV1Metrics := httptest.NewRequest(http.MethodGet, "/api/v1/servers/"+mockServer.ID+"/metrics", nil)
	recV1Metrics := httptest.NewRecorder()
	srv.Router().ServeHTTP(recV1Metrics, reqV1Metrics)
	if recV1Metrics.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/servers/{id}/metrics failed with %d", recV1Metrics.Code)
	}
}

func TestObservabilityEndpoints_AlertChannels(t *testing.T) {
	srv, _, obsRepo, _ := setupObservabilityTestServer(t)

	// Mock server to receive 1-click test alert
	var testAlertReceived bool
	mockAlertServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testAlertReceived = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer mockAlertServer.Close()

	// Configure alert dispatcher HTTP client to use mockAlertServer's client
	srv.ObservabilityAlertDispatcher().SetHTTPClient(mockAlertServer.Client())

	projectID := "proj_alerts_e2e"

	// 1. Validation error: empty channel name
	badPayload := map[string]interface{}{
		"name":      "",
		"type":      "telegram",
		"bot_token": "123",
		"chat_id":   "456",
	}
	body, _ := json.Marshal(badPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/alert-channels", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty channel name, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Validation error: unsupported channel type
	badTypePayload := map[string]interface{}{
		"name": "Slack Channel",
		"type": "slack",
	}
	body, _ = json.Marshal(badTypePayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/alert-channels", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported channel type, got %d", rec.Code)
	}

	// 3. Validation error: missing telegram bot_token/chat_id
	badTg := map[string]interface{}{
		"name": "Tg Alert",
		"type": "telegram",
	}
	body, _ = json.Marshal(badTg)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/alert-channels", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid telegram config, got %d", rec.Code)
	}

	// 4. Create Telegram alert channel
	tgPayload := map[string]interface{}{
		"name":      "Team Telegram Ops",
		"type":      "telegram",
		"bot_token": "123456789:ABCDefgh_SecretToken",
		"chat_id":   "-100987654321",
	}
	body, _ = json.Marshal(tgPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/alert-channels", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/projects/{id}/alert-channels (telegram) failed with %d: %s", rec.Code, rec.Body.String())
	}
	var createdTg struct {
		Channel cuanDB.AlertChannelDTO `json:"channel"`
		ID      string                 `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &createdTg)
	tgID := createdTg.Channel.ID
	if tgID == "" {
		tgID = createdTg.ID
	}
	if tgID == "" {
		t.Fatalf("expected non-empty channel ID in create response")
	}

	// Verify token is masked in response
	rawResp := rec.Body.String()
	if strings.Contains(rawResp, "ABCDefgh_SecretToken") {
		t.Fatalf("CRITICAL: raw bot_token leaked in create response: %s", rawResp)
	}

	// 5. Create Discord alert channel
	discordPayload := map[string]interface{}{
		"name":        "Discord Production",
		"type":        "discord",
		"webhook_url": "https://discord.com/api/webhooks/12345/secret_discord_token",
	}
	body, _ = json.Marshal(discordPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/alert-channels", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST discord channel failed with %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret_discord_token") {
		t.Fatalf("CRITICAL: discord webhook token leaked in create response")
	}

	// 6. Create Webhook alert channel (points to mockAlertServer for 1-click test)
	whPayload := map[string]interface{}{
		"name":        "Generic Webhook Receiver",
		"type":        "webhook",
		"webhook_url": mockAlertServer.URL,
		"secret":      "webhook-secret-9999",
	}
	body, _ = json.Marshal(whPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/alert-channels", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST generic webhook failed with %d: %s", rec.Code, rec.Body.String())
	}
	var createdWh struct {
		Channel cuanDB.AlertChannelDTO `json:"channel"`
		ID      string                 `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &createdWh)
	whID := createdWh.Channel.ID
	if whID == "" {
		whID = createdWh.ID
	}

	// Verify encryption in DB: config_encrypted should be set and ciphertext should NOT contain plaintext secrets
	storedChannel, err := obsRepo.GetAlertChannelByID(context.Background(), whID)
	if err != nil {
		t.Fatalf("GetAlertChannelByID failed: %v", err)
	}
	if storedChannel.ConfigEncrypted == "" {
		t.Fatalf("expected ConfigEncrypted to be set in DB")
	}
	if strings.Contains(storedChannel.ConfigEncrypted, "webhook-secret-9999") {
		t.Fatalf("CRITICAL: ConfigEncrypted contains plaintext secret in database!")
	}

	// 7. GET /api/projects/{id}/alert-channels
	reqList := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/alert-channels", nil)
	recList := httptest.NewRecorder()
	srv.Router().ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("GET /api/projects/{id}/alert-channels failed with %d: %s", recList.Code, recList.Body.String())
	}
	var listResp struct {
		Channels []cuanDB.AlertChannelDTO `json:"channels"`
		Count    int                      `json:"count"`
	}
	_ = json.Unmarshal(recList.Body.Bytes(), &listResp)
	if listResp.Count != 3 {
		t.Errorf("expected 3 channels, got %d", listResp.Count)
	}
	// Verify all returned channels have masked credentials (zero leak)
	listJSON := recList.Body.String()
	if strings.Contains(listJSON, "ABCDefgh_SecretToken") || strings.Contains(listJSON, "secret_discord_token") || strings.Contains(listJSON, "webhook-secret-9999") {
		t.Fatalf("CRITICAL: credentials leaked in list response: %s", listJSON)
	}

	// 8. POST /api/alert-channels/{id}/test (1-click test alert)
	reqTest := httptest.NewRequest(http.MethodPost, "/api/alert-channels/"+whID+"/test", nil)
	recTest := httptest.NewRecorder()
	srv.Router().ServeHTTP(recTest, reqTest)
	if recTest.Code != http.StatusOK {
		t.Fatalf("POST /api/alert-channels/{id}/test failed with %d: %s", recTest.Code, recTest.Body.String())
	}
	if !testAlertReceived {
		t.Errorf("expected 1-click test alert to be received by mock alert server")
	}

	// 9. POST /api/alert-channels/nonexistent/test -> 404
	reqTest404 := httptest.NewRequest(http.MethodPost, "/api/alert-channels/nonexistent-id/test", nil)
	recTest404 := httptest.NewRecorder()
	srv.Router().ServeHTTP(recTest404, reqTest404)
	if recTest404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent test channel, got %d", recTest404.Code)
	}

	// 10. DELETE /api/alert-channels/{id}
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/alert-channels/"+tgID, nil)
	recDel := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("DELETE /api/alert-channels/{id} failed with %d: %s", recDel.Code, recDel.Body.String())
	}

	// Re-delete -> 404
	reqDel404 := httptest.NewRequest(http.MethodDelete, "/api/alert-channels/"+tgID, nil)
	recDel404 := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDel404, reqDel404)
	if recDel404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleting already deleted channel, got %d", recDel404.Code)
	}

	// 11. Test v1 route aliases
	reqV1List := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/alert-channels", nil)
	recV1List := httptest.NewRecorder()
	srv.Router().ServeHTTP(recV1List, reqV1List)
	if recV1List.Code != http.StatusOK {
		t.Errorf("GET /api/v1/projects/{id}/alert-channels failed with %d", recV1List.Code)
	}

	reqV1Test := httptest.NewRequest(http.MethodPost, "/api/v1/alert-channels/"+whID+"/test", nil)
	recV1Test := httptest.NewRecorder()
	srv.Router().ServeHTTP(recV1Test, reqV1Test)
	if recV1Test.Code != http.StatusOK {
		t.Errorf("POST /api/v1/alert-channels/{id}/test failed with %d", recV1Test.Code)
	}
}

func TestObservabilityEndpoints_Incidents(t *testing.T) {
	srv, _, obsRepo, _ := setupObservabilityTestServer(t)
	ctx := context.Background()

	now := time.Now().UTC()

	// Pre-populate incidents
	inc1 := &cuanDB.AlertIncident{
		ProjectID:   "proj_p1",
		TargetType:  "deployment",
		TargetID:    "dep_100",
		Title:       "Service Outage: Gateway 504",
		Message:     "3 consecutive probe failures",
		Status:      "firing",
		TriggeredAt: now.Add(-30 * time.Minute),
	}
	inc2 := &cuanDB.AlertIncident{
		ProjectID:   "proj_p1",
		TargetType:  "server",
		TargetID:    "srv_200",
		Title:       "High CPU Usage",
		Message:     "CPU exceeded 95%",
		Status:      "resolved",
		TriggeredAt: now.Add(-20 * time.Minute),
	}
	inc3 := &cuanDB.AlertIncident{
		ProjectID:   "proj_p2",
		TargetType:  "deployment",
		TargetID:    "dep_300",
		Title:       "Outage: 500 Internal Error",
		Message:     "Failure streak",
		Status:      "firing",
		TriggeredAt: now.Add(-10 * time.Minute),
	}

	_ = obsRepo.CreateAlertIncident(ctx, inc1)
	_ = obsRepo.CreateAlertIncident(ctx, inc2)
	_ = obsRepo.CreateAlertIncident(ctx, inc3)

	// 1. GET /api/incidents (all)
	reqAll := httptest.NewRequest(http.MethodGet, "/api/incidents", nil)
	recAll := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAll, reqAll)
	if recAll.Code != http.StatusOK {
		t.Fatalf("GET /api/incidents failed with %d: %s", recAll.Code, recAll.Body.String())
	}
	var resAll struct {
		Incidents []cuanDB.AlertIncidentDTO `json:"incidents"`
		Count     int                       `json:"count"`
	}
	_ = json.Unmarshal(recAll.Body.Bytes(), &resAll)
	if resAll.Count != 3 {
		t.Errorf("expected 3 total incidents, got %d", resAll.Count)
	}

	// 2. GET /api/incidents?project_id=proj_p1
	reqP1 := httptest.NewRequest(http.MethodGet, "/api/incidents?project_id=proj_p1", nil)
	recP1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(recP1, reqP1)
	if recP1.Code != http.StatusOK {
		t.Fatalf("GET /api/incidents?project_id=proj_p1 failed with %d", recP1.Code)
	}
	var resP1 struct {
		Incidents []cuanDB.AlertIncidentDTO `json:"incidents"`
		Count     int                       `json:"count"`
	}
	_ = json.Unmarshal(recP1.Body.Bytes(), &resP1)
	if resP1.Count != 2 {
		t.Errorf("expected 2 incidents for proj_p1, got %d", resP1.Count)
	}

	// 3. GET /api/incidents?target_id=srv_200
	reqTarget := httptest.NewRequest(http.MethodGet, "/api/incidents?target_id=srv_200", nil)
	recTarget := httptest.NewRecorder()
	srv.Router().ServeHTTP(recTarget, reqTarget)
	if recTarget.Code != http.StatusOK {
		t.Fatalf("GET /api/incidents?target_id=srv_200 failed with %d", recTarget.Code)
	}
	var resTarget struct {
		Incidents []cuanDB.AlertIncidentDTO `json:"incidents"`
		Count     int                       `json:"count"`
	}
	_ = json.Unmarshal(recTarget.Body.Bytes(), &resTarget)
	if resTarget.Count != 1 || resTarget.Incidents[0].TargetID != "srv_200" {
		t.Errorf("expected 1 incident for srv_200, got %d", resTarget.Count)
	}

	// 4. GET /api/incidents?status=firing
	reqFiring := httptest.NewRequest(http.MethodGet, "/api/incidents?status=firing", nil)
	recFiring := httptest.NewRecorder()
	srv.Router().ServeHTTP(recFiring, reqFiring)
	var resFiring struct {
		Incidents []cuanDB.AlertIncidentDTO `json:"incidents"`
		Count     int                       `json:"count"`
	}
	_ = json.Unmarshal(recFiring.Body.Bytes(), &resFiring)
	if resFiring.Count != 2 {
		t.Errorf("expected 2 firing incidents, got %d", resFiring.Count)
	}

	// 5. GET /api/v1/incidents (v1 alias)
	reqV1 := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	recV1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(recV1, reqV1)
	if recV1.Code != http.StatusOK {
		t.Errorf("GET /api/v1/incidents failed with %d", recV1.Code)
	}
}

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

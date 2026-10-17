package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/wordpress"
)

func setupTestServerWithWordPress(t *testing.T) (*Server, string) {
	tempDir, err := os.MkdirTemp("", "cuan-server-wp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		AuthSecret:  "test-secret-key-32-bytes-cuan!",
		DatabaseURL: "",
	}

	srv := NewServer(cfg, nil)

	// Configure mock mode orchestrator for deterministic testing
	wpRepo := cuanDB.NewMemoryWordPressRepository()
	portPool := deployer.NewPortPoolWithRange(9400, 9450)
	portPool.SetCheckSocket(false)

	orch := wordpress.NewOrchestrator(wordpress.Config{
		DBRepo:        wpRepo,
		PortPool:      portPool,
		VaultSecret:   "test-secret-key-32-bytes-cuan!",
		BaseDeployDir: filepath.Join(tempDir, "deployments"),
		CoreCacheDir:  filepath.Join(tempDir, "cache"),
		MockMode:      true,
	})

	srv.SetWordPressRepository(wpRepo)
	srv.SetWordPressOrchestrator(orch)

	return srv, tempDir
}

func TestWordPressEndpoints_Validation(t *testing.T) {
	srv, tempDir := setupTestServerWithWordPress(t)
	defer os.RemoveAll(tempDir)

	// Missing site_title
	body := map[string]interface{}{
		"admin_email":    "admin@test.com",
		"admin_username": "admin",
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/p_test/wordpress/deploy", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing site_title, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWordPressEndpoints_FullLifecycle(t *testing.T) {
	srv, tempDir := setupTestServerWithWordPress(t)
	defer os.RemoveAll(tempDir)

	projectID := "p_wp_lifecycle"

	// 1. Deploy
	deployPayload := WordPressDeployRequestBody{
		SiteTitle:          "Turnkey WP Blog",
		AdminEmail:         "founder@wpblog.io",
		AdminUsername:      "wpadmin",
		PermalinkStructure: "/%postname%/",
	}
	data, _ := json.Marshal(deployPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/wordpress/deploy", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on deploy, got %d: %s", w.Code, w.Body.String())
	}

	var deployRes wordpress.DeployResult
	if err := json.NewDecoder(w.Body).Decode(&deployRes); err != nil {
		t.Fatalf("failed to decode deploy response: %v", err)
	}

	if deployRes.Status != cuanDB.WPStatusRunning {
		t.Errorf("expected status 'running', got %s", deployRes.Status)
	}
	if deployRes.HealthStatus != cuanDB.WPHealthStatusHealthy {
		t.Errorf("expected health 'healthy', got %s", deployRes.HealthStatus)
	}
	if deployRes.Port < 9400 || deployRes.Port > 9450 {
		t.Errorf("allocated port %d outside range [9400, 9450]", deployRes.Port)
	}
	if deployRes.AdminPasswordPlain == "" {
		t.Errorf("expected admin_password_plain to be populated in creation result")
	}

	// 2. GET /api/projects/{id}/wordpress (sanitized DTO)
	getReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/wordpress", nil)
	getW := httptest.NewRecorder()
	srv.Router().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on get, got %d: %s", getW.Code, getW.Body.String())
	}

	var dto cuanDB.WordPressDeploymentDTO
	if err := json.NewDecoder(getW.Body).Decode(&dto); err != nil {
		t.Fatalf("failed to decode DTO: %v", err)
	}
	if dto.SiteTitle != "Turnkey WP Blog" {
		t.Errorf("expected site title 'Turnkey WP Blog', got %s", dto.SiteTitle)
	}
	if !dto.HasAdminPassword {
		t.Errorf("expected HasAdminPassword=true")
	}

	// Verify zero password leakage in raw JSON
	rawJSON := getW.Body.String()
	if bytes.Contains([]byte(rawJSON), []byte("admin_password_encrypted")) {
		t.Errorf("raw JSON leaked admin_password_encrypted field")
	}
	if bytes.Contains([]byte(rawJSON), []byte(deployRes.AdminPasswordPlain)) {
		t.Errorf("raw JSON leaked plain admin password")
	}

	// 3. GET /api/projects/{id}/wordpress/health
	healthReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/wordpress/health", nil)
	healthW := httptest.NewRecorder()
	srv.Router().ServeHTTP(healthW, healthReq)

	if healthW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on health check, got %d: %s", healthW.Code, healthW.Body.String())
	}

	// 4. POST /api/projects/{id}/wordpress/restart
	restartReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/wordpress/restart", nil)
	restartW := httptest.NewRecorder()
	srv.Router().ServeHTTP(restartW, restartReq)

	if restartW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on restart, got %d: %s", restartW.Code, restartW.Body.String())
	}

	// 5. POST /api/projects/{id}/wordpress/stop
	stopReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/wordpress/stop", nil)
	stopW := httptest.NewRecorder()
	srv.Router().ServeHTTP(stopW, stopReq)

	if stopW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on stop, got %d: %s", stopW.Code, stopW.Body.String())
	}

	// 6. DELETE /api/projects/{id}/wordpress
	delReq := httptest.NewRequest(http.MethodDelete, "/api/projects/"+projectID+"/wordpress", nil)
	delW := httptest.NewRecorder()
	srv.Router().ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on delete, got %d: %s", delW.Code, delW.Body.String())
	}

	// 7. Verify subsequent GET returns 404
	afterGetReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/wordpress", nil)
	afterGetW := httptest.NewRecorder()
	srv.Router().ServeHTTP(afterGetW, afterGetReq)

	if afterGetW.Code != http.StatusNotFound {
		t.Errorf("expected 404 after deletion, got %d", afterGetW.Code)
	}
}

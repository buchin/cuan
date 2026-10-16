package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/deployer"
)

func setupTestServerWithDeployer(t *testing.T) (*Server, *deployer.LocalDriver, string) {
	tempDir, err := os.MkdirTemp("", "cuan-deployer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		Environment: "test",
		BaseDomain:  "cuan.localhost",
		AuthSecret:  "test-secret",
	}

	storage, err := deployer.NewStorageEngine(filepath.Join(tempDir, "deployments"))
	if err != nil {
		t.Fatalf("failed to create storage engine: %v", err)
	}
	store := deployer.NewMemoryStore()
	pool := deployer.NewPortPool()

	caddyCfg := deployer.CaddyConfig{
		AdminURL:   "http://127.0.0.1:9999", // mock / disabled
		BaseDomain: "cuan.localhost",
	}
	router := deployer.NewCaddyRouter(caddyCfg)
	driver := deployer.NewLocalDriverWithRouter(storage, store, pool, router)

	srv := NewServer(cfg, nil)
	srv.SetDeployerDriver(driver)

	return srv, driver, tempDir
}

func createZipArchive(files map[string]string) ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestDeployerHandlers_ListEmpty(t *testing.T) {
	srv, _, tempDir := setupTestServerWithDeployer(t)
	defer os.RemoveAll(tempDir)

	req := httptest.NewRequest("GET", "/api/deployments", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Deployments []deployer.Deployment `json:"deployments"`
		Count       int                   `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if len(res.Deployments) != 0 || res.Count != 0 {
		t.Errorf("expected 0 deployments, got %d", res.Count)
	}
}

func TestDeployerHandlers_UploadZip_StaticSuccess(t *testing.T) {
	srv, _, tempDir := setupTestServerWithDeployer(t)
	defer os.RemoveAll(tempDir)

	zipData, err := createZipArchive(map[string]string{
		"index.html": "<!DOCTYPE html><html><body><h1>Hello Cuan</h1></body></html>",
	})
	if err != nil {
		t.Fatalf("create zip archive: %v", err)
	}

	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)

	part, err := mw.CreateFormFile("file", "my-portfolio.zip")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(zipData); err != nil {
		t.Fatalf("write zip part: %v", err)
	}

	_ = mw.WriteField("project_name", "My Static Portfolio")
	_ = mw.WriteField("slug", "my-portfolio")
	_ = mw.WriteField("env_vars", `{"APP_ENV":"production"}`)
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/api/deployments/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var res deployer.DeploymentResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Deployment == nil {
		t.Fatal("expected non-nil deployment in result")
	}

	if res.Deployment.ProjectName != "My Static Portfolio" {
		t.Errorf("expected project name 'My Static Portfolio', got '%s'", res.Deployment.ProjectName)
	}

	if res.Deployment.Slug != "my-portfolio" {
		t.Errorf("expected slug 'my-portfolio', got '%s'", res.Deployment.Slug)
	}

	if res.DetectedRuntime != deployer.RuntimeStatic {
		t.Errorf("expected detected runtime 'static', got '%s'", res.DetectedRuntime)
	}
}

func TestDeployerHandlers_UploadZip_ZipSlipRejected(t *testing.T) {
	srv, _, tempDir := setupTestServerWithDeployer(t)
	defer os.RemoveAll(tempDir)

	// Create malicious zip with path traversal
	zipData, err := createZipArchive(map[string]string{
		"../../evil.txt": "compromised",
	})
	if err != nil {
		t.Fatalf("create evil zip archive: %v", err)
	}

	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)

	part, err := mw.CreateFormFile("file", "malicious.zip")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(zipData); err != nil {
		t.Fatalf("write evil zip: %v", err)
	}
	_ = mw.WriteField("project_name", "Evil App")
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/api/deployments/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for ZipSlip, got %d: %s", rec.Code, rec.Body.String())
	}

	if !bytes.Contains(rec.Body.Bytes(), []byte("security violation")) {
		t.Errorf("expected security violation error message, got: %s", rec.Body.String())
	}
}

func TestDeployerHandlers_CreateGitDeployment(t *testing.T) {
	srv, _, tempDir := setupTestServerWithDeployer(t)
	defer os.RemoveAll(tempDir)

	payload := map[string]interface{}{
		"project_name": "Test Git App",
		"repo_url":     "https://github.com/buchin/cuan.git",
		"branch":       "main",
		"runtime":      "nodejs",
		"env_vars": map[string]string{
			"NODE_ENV": "production",
		},
	}
	jsonBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/deployments/git", bytes.NewReader(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var res deployer.DeploymentResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Deployment == nil || res.Deployment.ProjectName != "Test Git App" {
		t.Errorf("unexpected deployment result: %+v", res.Deployment)
	}

	if res.Deployment.SourceType != deployer.SourceTypeGit {
		t.Errorf("expected source_type 'git', got '%s'", res.Deployment.SourceType)
	}
}

func TestDeployerHandlers_Lifecycle(t *testing.T) {
	srv, _, tempDir := setupTestServerWithDeployer(t)
	defer os.RemoveAll(tempDir)

	// 1. Upload a static project
	zipData, err := createZipArchive(map[string]string{
		"index.html": "<!DOCTYPE html><html><body><h1>Lifecycle Test</h1></body></html>",
	})
	if err != nil {
		t.Fatalf("create zip archive: %v", err)
	}

	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "lifecycle.zip")
	_, _ = part.Write(zipData)
	_ = mw.WriteField("project_name", "Lifecycle Project")
	_ = mw.WriteField("slug", "lifecycle-test")
	_ = mw.Close()

	reqUpload := httptest.NewRequest("POST", "/api/deployments/upload", body)
	reqUpload.Header.Set("Content-Type", mw.FormDataContentType())
	recUpload := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUpload, reqUpload)

	if recUpload.Code != http.StatusCreated {
		t.Fatalf("upload failed: %d %s", recUpload.Code, recUpload.Body.String())
	}

	var uploadRes deployer.DeploymentResult
	_ = json.NewDecoder(recUpload.Body).Decode(&uploadRes)
	depID := uploadRes.Deployment.ID

	// 2. Get deployment details & health
	reqGet := httptest.NewRequest("GET", "/api/deployments/"+depID, nil)
	recGet := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("get deployment failed: %d %s", recGet.Code, recGet.Body.String())
	}

	// 3. Start deployment
	reqStart := httptest.NewRequest("POST", "/api/deployments/"+depID+"/start", nil)
	recStart := httptest.NewRecorder()
	srv.Router().ServeHTTP(recStart, reqStart)

	if recStart.Code != http.StatusOK {
		t.Fatalf("start deployment failed: %d %s", recStart.Code, recStart.Body.String())
	}

	var startRes struct {
		Status     string              `json:"status"`
		Deployment deployer.Deployment `json:"deployment"`
	}
	_ = json.NewDecoder(recStart.Body).Decode(&startRes)

	if startRes.Status != "running" || startRes.Deployment.Status != deployer.StatusRunning {
		t.Errorf("expected running status, got: %+v", startRes)
	}

	time.Sleep(100 * time.Millisecond)

	// 4. Check Health
	reqHealth := httptest.NewRequest("GET", "/api/deployments/"+depID+"/health", nil)
	recHealth := httptest.NewRecorder()
	srv.Router().ServeHTTP(recHealth, reqHealth)

	if recHealth.Code != http.StatusOK {
		t.Fatalf("health check failed: %d %s", recHealth.Code, recHealth.Body.String())
	}

	// 5. Get Logs
	reqLogs := httptest.NewRequest("GET", "/api/deployments/"+depID+"/logs?limit=50", nil)
	recLogs := httptest.NewRecorder()
	srv.Router().ServeHTTP(recLogs, reqLogs)

	if recLogs.Code != http.StatusOK {
		t.Fatalf("get logs failed: %d %s", recLogs.Code, recLogs.Body.String())
	}

	var logsRes struct {
		Logs  []deployer.LogEntry `json:"logs"`
		Count int                 `json:"count"`
	}
	_ = json.NewDecoder(recLogs.Body).Decode(&logsRes)
	if logsRes.Count == 0 {
		t.Logf("Note: logs count is %d (system logs expected)", logsRes.Count)
	}

	// 6. Restart deployment
	reqRestart := httptest.NewRequest("POST", "/api/deployments/"+depID+"/restart", nil)
	recRestart := httptest.NewRecorder()
	srv.Router().ServeHTTP(recRestart, reqRestart)

	if recRestart.Code != http.StatusOK {
		t.Fatalf("restart deployment failed: %d %s", recRestart.Code, recRestart.Body.String())
	}

	// 7. Stop deployment
	reqStop := httptest.NewRequest("POST", "/api/deployments/"+depID+"/stop", nil)
	recStop := httptest.NewRecorder()
	srv.Router().ServeHTTP(recStop, reqStop)

	if recStop.Code != http.StatusOK {
		t.Fatalf("stop deployment failed: %d %s", recStop.Code, recStop.Body.String())
	}

	var stopRes struct {
		Status     string              `json:"status"`
		Deployment deployer.Deployment `json:"deployment"`
	}
	_ = json.NewDecoder(recStop.Body).Decode(&stopRes)
	if stopRes.Status != "stopped" || stopRes.Deployment.Status != deployer.StatusStopped {
		t.Errorf("expected stopped status, got: %+v", stopRes)
	}

	// 8. Delete deployment
	reqDelete := httptest.NewRequest("DELETE", "/api/deployments/"+depID, nil)
	recDelete := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDelete, reqDelete)

	if recDelete.Code != http.StatusOK {
		t.Fatalf("delete deployment failed: %d %s", recDelete.Code, recDelete.Body.String())
	}

	// 9. Verify 404 after delete
	reqGetAfter := httptest.NewRequest("GET", "/api/deployments/"+depID, nil)
	recGetAfter := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGetAfter, reqGetAfter)

	if recGetAfter.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", recGetAfter.Code)
	}
}

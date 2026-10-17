package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/project"
)

type mockCommandExecutor struct {
	mu           sync.Mutex
	executedCmds []string
	failOnStep   string
	failError    error
}

func newMockCommandExecutor() *mockCommandExecutor {
	return &mockCommandExecutor{
		executedCmds: make([]string, 0),
	}
}

func (m *mockCommandExecutor) Execute(ctx context.Context, dir string, env []string, stdout io.Writer, stderr io.Writer, name string, args ...string) error {
	m.mu.Lock()
	cmdStr := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	m.executedCmds = append(m.executedCmds, cmdStr)
	failOn := m.failOnStep
	failErr := m.failError
	m.mu.Unlock()

	if stdout != nil {
		_, _ = stdout.Write([]byte(fmt.Sprintf("Mock execution of %s succeeded\n", name)))
	}

	if failOn != "" && strings.Contains(cmdStr, failOn) {
		if stderr != nil {
			_, _ = stderr.Write([]byte(fmt.Sprintf("Mock error executing %s\n", name)))
		}
		if failErr != nil {
			return failErr
		}
		return errors.New("command failed with mock error")
	}

	return nil
}

type mockProcessLauncher struct {
	mu           sync.Mutex
	launchedCmds []string
	nextPID      int
}

func newMockProcessLauncher() *mockProcessLauncher {
	return &mockProcessLauncher{
		nextPID: 2000,
	}
}

func (m *mockProcessLauncher) Start(ctx context.Context, dir string, env []string, stdout, stderr io.Writer, name string, args ...string) (*exec.Cmd, int, error) {
	m.mu.Lock()
	m.nextPID++
	pid := m.nextPID
	cmdStr := fmt.Sprintf("%s %s", name, args)
	m.launchedCmds = append(m.launchedCmds, cmdStr)
	m.mu.Unlock()

	cmd := exec.Command("sleep", "60")
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	return cmd, pid, nil
}

type mockRouter struct {
	mu           sync.Mutex
	routes       map[string]int
	customRoutes map[string]deployer.CustomDomainTarget
}

func newMockRouter() *mockRouter {
	return &mockRouter{
		routes:       make(map[string]int),
		customRoutes: make(map[string]deployer.CustomDomainTarget),
	}
}

func (m *mockRouter) RegisterRoute(ctx context.Context, slug string, internalPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes[slug] = internalPort
	return nil
}

func (m *mockRouter) UnregisterRoute(ctx context.Context, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.routes, slug)
	return nil
}

func (m *mockRouter) FormatDomain(slug string) string {
	return fmt.Sprintf("%s.cuan.dojo.cc", slug)
}

func (m *mockRouter) BaseDomain() string {
	return "cuan.dojo.cc"
}

func (m *mockRouter) GetRouteStatus(ctx context.Context, slug string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.routes[slug]
	return exists, nil
}

func (m *mockRouter) Reconcile(ctx context.Context, activeDeployments []*deployer.Deployment) error {
	return nil
}

func (m *mockRouter) AddCustomDomainRoute(ctx context.Context, domain string, target deployer.CustomDomainTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customRoutes[domain] = target
	return nil
}

func (m *mockRouter) RemoveCustomDomainRoute(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.customRoutes, domain)
	return nil
}

func (m *mockRouter) GetCustomDomainRouteStatus(ctx context.Context, domain string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.customRoutes[domain]
	return exists, nil
}

func (m *mockRouter) ConfigureOnDemandTLS(ctx context.Context, askURL string) error {
	return nil
}

func setupTestServerForScriptDeploy(t *testing.T) (*Server, *mockCommandExecutor, *mockProcessLauncher, string) {
	t.Helper()
	tempBase, err := os.MkdirTemp("", "cuan-srv-script-deploy-*")
	if err != nil {
		t.Fatalf("create temp base: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempBase)
	})

	cfg := &config.Config{
		AuthSecret: "test-auth-secret-32-chars-long!",
	}
	srv := NewServer(cfg, nil)

	mockExec := newMockCommandExecutor()
	runner := deployer.NewScriptRunner(mockExec)
	srv.SetScriptRunner(runner)

	portPool := deployer.NewPortPoolWithRange(9400, 9499)
	portPool.SetCheckSocket(false)
	router := newMockRouter()
	mockLauncher := newMockProcessLauncher()

	supervisor := deployer.NewProcessSupervisor(deployer.ProcessSupervisorConfig{
		PortPool:          portPool,
		Router:            router,
		Repo:              srv.ScriptDeploymentRepository(),
		ProvisionedDBRepo: srv.ProvisionedDatabaseRepository(),
		Launcher:          mockLauncher,
		ProjectBaseDir:    tempBase,
		VaultSecret:       cfg.AuthSecret,
	})
	srv.SetProcessSupervisor(supervisor)

	return srv, mockExec, mockLauncher, tempBase
}

func createDummyScriptProject(t *testing.T, srv *Server, projectID, baseDir string, stack string) *project.ProjectResponse {
	t.Helper()
	projDir := filepath.Join(baseDir, projectID)
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	switch stack {
	case "laravel":
		_ = os.WriteFile(filepath.Join(projDir, "composer.json"), []byte(`{"name":"laravel/app"}`), 0644)
		_ = os.WriteFile(filepath.Join(projDir, "artisan"), []byte("#!/usr/bin/env php"), 0755)
		_ = os.MkdirAll(filepath.Join(projDir, "public"), 0755)
	case "node":
		_ = os.WriteFile(filepath.Join(projDir, "package.json"), []byte(`{"scripts":{"start":"node index.js","build":"vite build"}}`), 0644)
		_ = os.WriteFile(filepath.Join(projDir, "index.js"), []byte("console.log('Server');"), 0644)
	default:
		_ = os.WriteFile(filepath.Join(projDir, "index.html"), []byte("<h1>Static App</h1>"), 0644)
	}

	ctx := context.Background()
	p := &project.Project{
		ID:        projectID,
		Name:      "Test Script Application",
		LocalPath: projDir,
	}
	if err := srv.ProjectService().Store().CreateProject(ctx, p); err != nil {
		t.Fatalf("create dummy project: %v", err)
	}
	return p.ToResponse()
}

func TestScriptDeployEndpoint_Success(t *testing.T) {
	srv, mockExec, _, baseDir := setupTestServerForScriptDeploy(t)
	projectID := "proj_script_laravel_1"
	createDummyScriptProject(t, srv, projectID, baseDir, "laravel")

	skipWait := true
	reqBody := ScriptDeployRequestBody{
		Stack:          "laravel",
		HealthPath:     "/",
		TimeoutSeconds: 60,
		SkipHealthWait: &skipWait,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/script-deploy", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Deployment  cuanDB.ScriptDeploymentDTO `json:"deployment"`
		BuildResult deployer.ScriptBuildResult `json:"build_result"`
		Process     map[string]interface{}     `json:"process"`
		Message     string                     `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Deployment.ProjectID != projectID {
		t.Errorf("expected project_id %s, got %s", projectID, resp.Deployment.ProjectID)
	}
	if resp.Deployment.BuildStatus != cuanDB.BuildStatusRunning {
		t.Errorf("expected build_status running, got %s", resp.Deployment.BuildStatus)
	}
	if resp.Deployment.Port < 9400 || resp.Deployment.Port > 9499 {
		t.Errorf("allocated port %d outside expected range", resp.Deployment.Port)
	}
	if !resp.BuildResult.Success {
		t.Errorf("expected build result success = true")
	}

	// Verify executed commands included composer and artisan
	foundComposer := false
	for _, cmd := range mockExec.executedCmds {
		if strings.Contains(cmd, "composer install") {
			foundComposer = true
		}
	}
	if !foundComposer {
		t.Errorf("expected composer install to have been executed")
	}
}

func TestScriptDeployEndpoint_BuildFailure(t *testing.T) {
	srv, mockExec, _, baseDir := setupTestServerForScriptDeploy(t)
	projectID := "proj_script_fail"
	createDummyScriptProject(t, srv, projectID, baseDir, "laravel")

	mockExec.failOnStep = "composer install"
	mockExec.failError = errors.New("composer out of memory")

	reqBody := ScriptDeployRequestBody{
		Stack: "laravel",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/script-deploy", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422 Unprocessable Entity, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != "build pipeline failed" {
		t.Errorf("expected 'build pipeline failed' error, got %v", resp["error"])
	}

	// Verify deployment state in DB marked failed
	dep, err := srv.ScriptDeploymentRepository().GetDeploymentByProjectID(context.Background(), projectID)
	if err != nil || dep == nil {
		t.Fatalf("failed to retrieve deployment: %v", err)
	}
	if dep.BuildStatus != cuanDB.BuildStatusFailed {
		t.Errorf("expected status failed in DB, got %s", dep.BuildStatus)
	}
}

func TestScriptDeployLogsEndpoint_JSONAndSSE(t *testing.T) {
	srv, _, _, baseDir := setupTestServerForScriptDeploy(t)
	projectID := "proj_script_logs_test"
	createDummyScriptProject(t, srv, projectID, baseDir, "html")

	streamer := srv.ProcessSupervisor().GetOrCreateStreamer(projectID)
	streamer.Append(context.Background(), deployer.LogEntry{
		DeploymentID: projectID,
		Stream:       deployer.StreamStdout,
		Message:      "Build step started",
		Timestamp:    time.Now().UTC(),
	})
	streamer.Append(context.Background(), deployer.LogEntry{
		DeploymentID: projectID,
		Stream:       deployer.StreamStdout,
		Message:      "Build step completed",
		Timestamp:    time.Now().UTC(),
	})

	// 1. Plain JSON snapshot
	reqJSON := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/script-deploy/logs?limit=50", nil)
	wJSON := httptest.NewRecorder()
	srv.Router().ServeHTTP(wJSON, reqJSON)

	if wJSON.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", wJSON.Code, wJSON.Body.String())
	}

	var jsonResp struct {
		Logs  []deployer.LogEntry `json:"logs"`
		Count int                 `json:"count"`
	}
	if err := json.Unmarshal(wJSON.Body.Bytes(), &jsonResp); err != nil {
		t.Fatalf("failed to parse logs json: %v", err)
	}
	if jsonResp.Count != 2 {
		t.Errorf("expected 2 log entries, got %d", jsonResp.Count)
	}

	// 2. SSE Streaming
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	reqSSE := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/script-deploy/logs?stream=true", nil).WithContext(ctx)
	reqSSE.Header.Set("Accept", "text/event-stream")
	wSSE := httptest.NewRecorder()

	srv.Router().ServeHTTP(wSSE, reqSSE)

	if ct := wSSE.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}
	bodyStr := wSSE.Body.String()
	if !strings.Contains(bodyStr, "data:") {
		t.Errorf("expected SSE formatted data lines, got: %s", bodyStr)
	}
}

func TestScriptDeployLogsEndpoint_WebSocket(t *testing.T) {
	srv, _, _, baseDir := setupTestServerForScriptDeploy(t)
	projectID := "proj_script_ws_test"
	createDummyScriptProject(t, srv, projectID, baseDir, "html")

	streamer := srv.ProcessSupervisor().GetOrCreateStreamer(projectID)
	streamer.Append(context.Background(), deployer.LogEntry{
		DeploymentID: projectID,
		Stream:       deployer.StreamStdout,
		Message:      "Initial line for WebSocket test",
		Timestamp:    time.Now().UTC(),
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/projects/" + projectID + "/script-deploy/logs"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v (response: %v)", err, resp)
	}
	defer conn.Close()

	// Read initial message
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial message from WebSocket: %v", err)
	}

	var initPayload struct {
		Type string              `json:"type"`
		Logs []deployer.LogEntry `json:"logs"`
	}
	if err := json.Unmarshal(message, &initPayload); err != nil {
		t.Fatalf("failed to decode initial WS message: %v", err)
	}
	if initPayload.Type != "init" {
		t.Errorf("expected message type 'init', got '%s'", initPayload.Type)
	}
	if len(initPayload.Logs) == 0 {
		t.Errorf("expected at least 1 log entry in initial payload")
	}
}

func TestScriptDeployRestartEndpoint(t *testing.T) {
	srv, _, _, baseDir := setupTestServerForScriptDeploy(t)
	projectID := "proj_script_restart"
	createDummyScriptProject(t, srv, projectID, baseDir, "html")

	// Deploy first
	skipWait := true
	reqBody := ScriptDeployRequestBody{
		Stack:          "html",
		SkipHealthWait: &skipWait,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/script-deploy", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("deploy failed with status %d: %s", w.Code, w.Body.String())
	}

	// Trigger Restart
	reqRestart := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/script-deploy/restart", nil)
	wRestart := httptest.NewRecorder()
	srv.Router().ServeHTTP(wRestart, reqRestart)

	if wRestart.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", wRestart.Code, wRestart.Body.String())
	}

	var resp struct {
		Message string                 `json:"message"`
		Process map[string]interface{} `json:"process"`
	}
	if err := json.Unmarshal(wRestart.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse restart response: %v", err)
	}
	if resp.Message != "Service restarted successfully" {
		t.Errorf("expected 'Service restarted successfully', got '%s'", resp.Message)
	}
}

func TestScriptDeployStatusEndpoint(t *testing.T) {
	srv, _, _, baseDir := setupTestServerForScriptDeploy(t)
	projectID := "proj_script_status"
	createDummyScriptProject(t, srv, projectID, baseDir, "html")

	// Deploy first
	skipWait := true
	reqBody := ScriptDeployRequestBody{
		Stack:          "html",
		SkipHealthWait: &skipWait,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/script-deploy", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("deploy failed with status %d: %s", w.Code, w.Body.String())
	}

	// Query Status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/script-deploy/status", nil)
	wStatus := httptest.NewRecorder()
	srv.Router().ServeHTTP(wStatus, reqStatus)

	if wStatus.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", wStatus.Code, wStatus.Body.String())
	}

	var resp struct {
		Deployment cuanDB.ScriptDeploymentDTO `json:"deployment"`
		Process    deployer.SupervisedStatus  `json:"process"`
	}
	if err := json.Unmarshal(wStatus.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse status response: %v", err)
	}

	if resp.Deployment.ProjectID != projectID {
		t.Errorf("expected project_id %s, got %s", projectID, resp.Deployment.ProjectID)
	}
	if resp.Process.Status != cuanDB.BuildStatusRunning {
		t.Errorf("expected running process status, got %s", resp.Process.Status)
	}
}

func TestScriptDeployLegacyRoutes(t *testing.T) {
	srv, _, _, baseDir := setupTestServerForScriptDeploy(t)
	projectID := "proj_script_legacy"
	createDummyScriptProject(t, srv, projectID, baseDir, "html")

	// POST /api/projects/{id}/script-deploy
	skipWait := true
	reqBody := ScriptDeployRequestBody{
		Stack:          "html",
		SkipHealthWait: &skipWait,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/script-deploy", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("legacy route POST failed: %d: %s", w.Code, w.Body.String())
	}

	// GET /api/projects/{id}/script-deploy/status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/script-deploy/status", nil)
	wStatus := httptest.NewRecorder()
	srv.Router().ServeHTTP(wStatus, reqStatus)
	if wStatus.Code != http.StatusOK {
		t.Fatalf("legacy route GET status failed: %d: %s", wStatus.Code, wStatus.Body.String())
	}
}

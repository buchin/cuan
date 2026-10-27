package deployer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
)

// MockProcessLauncher implements ProcessLauncher for fast, reliable unit tests.
type MockProcessLauncher struct {
	mu           sync.Mutex
	LaunchedCmds []string
	NextPID      int
	ShouldCrash  bool
	CrashAfter   time.Duration
}

func NewMockProcessLauncher() *MockProcessLauncher {
	return &MockProcessLauncher{
		NextPID: 1000,
	}
}

func (m *MockProcessLauncher) Start(ctx context.Context, dir string, env []string, stdout, stderr io.Writer, name string, args ...string) (*exec.Cmd, int, error) {
	m.mu.Lock()
	m.NextPID++
	pid := m.NextPID
	cmdStr := fmt.Sprintf("%s %s", name, args)
	m.LaunchedCmds = append(m.LaunchedCmds, cmdStr)
	crash := m.ShouldCrash
	crashAfter := m.CrashAfter
	m.mu.Unlock()

	// Use a sleep command that can be managed
	var cmd *exec.Cmd
	if crash {
		// Command that exits with error after short delay
		cmd = exec.Command("bash", "-c", fmt.Sprintf("sleep %.2f && exit 1", crashAfter.Seconds()))
	} else {
		cmd = exec.Command("sleep", "60")
	}

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

// MockRouter records dynamic route registrations and unregistrations.
type MockRouter struct {
	mu           sync.Mutex
	Routes       map[string]int
	CustomRoutes map[string]CustomDomainTarget
}

func NewMockRouter() *MockRouter {
	return &MockRouter{
		Routes:       make(map[string]int),
		CustomRoutes: make(map[string]CustomDomainTarget),
	}
}

func (m *MockRouter) RegisterRoute(ctx context.Context, slug string, internalPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Routes[slug] = internalPort
	return nil
}

func (m *MockRouter) UnregisterRoute(ctx context.Context, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Routes, slug)
	return nil
}

func (m *MockRouter) FormatDomain(slug string) string {
	return fmt.Sprintf("%s.cuan.dojo.cc", slug)
}

func (m *MockRouter) BaseDomain() string {
	return "cuan.dojo.cc"
}

func (m *MockRouter) GetRouteStatus(ctx context.Context, slug string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.Routes[slug]
	return exists, nil
}

func (m *MockRouter) Reconcile(ctx context.Context, activeDeployments []*Deployment) error {
	return nil
}

func (m *MockRouter) AddCustomDomainRoute(ctx context.Context, domain string, target CustomDomainTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CustomRoutes[domain] = target
	return nil
}

func (m *MockRouter) RemoveCustomDomainRoute(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.CustomRoutes, domain)
	return nil
}

func (m *MockRouter) GetCustomDomainRouteStatus(ctx context.Context, domain string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.CustomRoutes[domain]
	return exists, nil
}

func (m *MockRouter) ConfigureOnDemandTLS(ctx context.Context, askURL string) error {
	return nil
}

func TestProcessSupervisor_DynamicPortAllocationAndRoute(t *testing.T) {
	repo := cuanDB.NewMemoryScriptDeploymentRepository()
	portPool := NewPortPoolWithRange(9100, 9150)
	portPool.SetCheckSocket(false)
	mockRouter := NewMockRouter()
	mockLauncher := NewMockProcessLauncher()

	sup := NewProcessSupervisor(ProcessSupervisorConfig{
		PortPool: portPool,
		Router:   mockRouter,
		Repo:     repo,
		Launcher: mockLauncher,
	})

	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<h1>App</h1>"), 0644)

	req1 := SupervisorStartRequest{
		ProjectID:      "proj_alloc_1",
		ProjectDir:     tmpDir,
		SkipHealthWait: true,
	}

	proc1, err := sup.StartProcess(context.Background(), req1)
	if err != nil {
		t.Fatalf("failed to start process 1: %v", err)
	}
	defer func() { _ = sup.StopProcess(context.Background(), "proj_alloc_1") }()

	if proc1.Port < 9100 || proc1.Port > 9150 {
		t.Fatalf("allocated port %d outside pool range 9100-9150", proc1.Port)
	}

	// Verify Caddy route registered
	if port, exists := mockRouter.Routes["proj_alloc_1"]; !exists || port != proc1.Port {
		t.Errorf("expected Caddy route for proj_alloc_1 pointing to %d, got %d (exists=%v)", proc1.Port, port, exists)
	}

	// Start second process and ensure non-conflicting port
	req2 := SupervisorStartRequest{
		ProjectID:      "proj_alloc_2",
		ProjectDir:     tmpDir,
		SkipHealthWait: true,
	}

	proc2, err := sup.StartProcess(context.Background(), req2)
	if err != nil {
		t.Fatalf("failed to start process 2: %v", err)
	}
	defer func() { _ = sup.StopProcess(context.Background(), "proj_alloc_2") }()

	if proc2.Port == proc1.Port {
		t.Fatalf("port collision detected! Both processes got port %d", proc1.Port)
	}

	// Verify repo updated
	dep1, err := repo.GetDeploymentByProjectID(context.Background(), "proj_alloc_1")
	if err != nil || dep1 == nil {
		t.Fatalf("failed to get deployment 1 from repo: %v", err)
	}
	if dep1.Port != proc1.Port {
		t.Errorf("expected repo port %d, got %d", proc1.Port, dep1.Port)
	}
	if dep1.BuildStatus != cuanDB.BuildStatusRunning {
		t.Errorf("expected build_status running, got %s", dep1.BuildStatus)
	}
}

func TestProcessSupervisor_StopAndRestart(t *testing.T) {
	repo := cuanDB.NewMemoryScriptDeploymentRepository()
	portPool := NewPortPoolWithRange(9200, 9250)
	portPool.SetCheckSocket(false)
	mockRouter := NewMockRouter()
	mockLauncher := NewMockProcessLauncher()

	sup := NewProcessSupervisor(ProcessSupervisorConfig{
		PortPool: portPool,
		Router:   mockRouter,
		Repo:     repo,
		Launcher: mockLauncher,
	})

	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<h1>App</h1>"), 0644)

	req := SupervisorStartRequest{
		ProjectID:      "proj_restart_test",
		ProjectDir:     tmpDir,
		SkipHealthWait: true,
	}

	proc, err := sup.StartProcess(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	originalPort := proc.Port

	// Stop process
	err = sup.StopProcess(context.Background(), "proj_restart_test")
	if err != nil {
		t.Fatalf("failed to stop process: %v", err)
	}

	// Port should be released in pool
	if portPool.IsLeased(originalPort) {
		t.Errorf("expected port %d to be released from pool", originalPort)
	}

	// Route should be removed from Caddy
	if _, exists := mockRouter.Routes["proj_restart_test"]; exists {
		t.Errorf("expected route for proj_restart_test to be removed from Caddy")
	}

	// DB status should be stopped
	dep, err := repo.GetDeploymentByProjectID(context.Background(), "proj_restart_test")
	if err != nil || dep == nil {
		t.Fatalf("failed to retrieve stopped deployment: %v", err)
	}
	if dep.BuildStatus != cuanDB.BuildStatusStopped {
		t.Errorf("expected status stopped, got %s", dep.BuildStatus)
	}

	// Restart process
	restarted, err := sup.RestartProcess(context.Background(), "proj_restart_test")
	if err != nil {
		t.Fatalf("failed to restart process: %v", err)
	}
	defer func() { _ = sup.StopProcess(context.Background(), "proj_restart_test") }()

	if restarted.Status != cuanDB.BuildStatusRunning {
		t.Errorf("expected restarted status running, got %s", restarted.Status)
	}
}

func TestProcessSupervisor_CrashDetection(t *testing.T) {
	repo := cuanDB.NewMemoryScriptDeploymentRepository()
	portPool := NewPortPoolWithRange(9300, 9350)
	portPool.SetCheckSocket(false)
	mockRouter := NewMockRouter()

	mockLauncher := NewMockProcessLauncher()
	mockLauncher.ShouldCrash = true
	mockLauncher.CrashAfter = 50 * time.Millisecond

	sup := NewProcessSupervisor(ProcessSupervisorConfig{
		PortPool: portPool,
		Router:   mockRouter,
		Repo:     repo,
		Launcher: mockLauncher,
	})

	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<h1>App</h1>"), 0644)

	req := SupervisorStartRequest{
		ProjectID:      "proj_crash_test",
		ProjectDir:     tmpDir,
		SkipHealthWait: true,
	}

	proc, err := sup.StartProcess(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	crashedPort := proc.Port

	// Wait for crash monitor to detect exit
	select {
	case <-proc.Done:
	case <-time.After(2 * time.Second):
		t.Fatalf("process did not exit within timeout")
	}

	// Wait brief moment for crash handler to finish updating state
	time.Sleep(100 * time.Millisecond)

	// Status should be failed/crashed
	proc.mu.Lock()
	status := proc.Status
	health := proc.Health
	proc.mu.Unlock()

	if status != cuanDB.BuildStatusFailed {
		t.Errorf("expected status failed after crash, got %s", status)
	}
	if health != cuanDB.HealthStatusUnhealthy {
		t.Errorf("expected health unhealthy after crash, got %s", health)
	}

	// Port should be automatically released on crash
	if portPool.IsLeased(crashedPort) {
		t.Errorf("expected port %d to be released after crash", crashedPort)
	}

	// DB record should reflect failed/unhealthy
	dep, err := repo.GetDeploymentByProjectID(context.Background(), "proj_crash_test")
	if err != nil || dep == nil {
		t.Fatalf("failed to retrieve crashed deployment: %v", err)
	}
	if dep.BuildStatus != cuanDB.BuildStatusFailed {
		t.Errorf("expected DB status failed, got %s", dep.BuildStatus)
	}
}

func TestPollHTTPHealth_SuccessAndRetry(t *testing.T) {
	var callCount int32

	// Mock server that returns 503 on first 2 calls, then 200 OK
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			http.Error(w, "warming up...", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Application Ready"))
	}))
	defer ts.Close()

	outcome, err := PollHTTPHealth(context.Background(), ts.URL,
		WithHealthInterval(50*time.Millisecond),
		WithHealthTimeout(2*time.Second),
		WithHTTPClient(ts.Client()),
	)

	if err != nil {
		t.Fatalf("expected health check to succeed, got: %v", err)
	}

	if !outcome.Healthy {
		t.Errorf("expected outcome.Healthy = true")
	}

	if outcome.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", outcome.StatusCode)
	}

	if outcome.Attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", outcome.Attempts)
	}
}

func TestPollHTTPHealth_Timeout(t *testing.T) {
	// Server always returning 500
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	outcome, err := PollHTTPHealth(context.Background(), ts.URL,
		WithHealthInterval(30*time.Millisecond),
		WithHealthTimeout(150*time.Millisecond),
		WithHTTPClient(ts.Client()),
	)

	if err == nil {
		t.Fatalf("expected health check timeout error, got nil")
	}

	if outcome.Healthy {
		t.Errorf("expected outcome.Healthy = false")
	}
}

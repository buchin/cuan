package wordpress

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

// MockRouter implements deployer.Router for testing Caddy route registrations.
type MockRouter struct {
	mu           sync.Mutex
	Routes       map[string]int
	CustomRoutes map[string]deployer.CustomDomainTarget
}

func NewMockRouter() *MockRouter {
	return &MockRouter{
		Routes:       make(map[string]int),
		CustomRoutes: make(map[string]deployer.CustomDomainTarget),
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
	return fmtDomain(slug)
}

func fmtDomain(slug string) string {
	return slug + ".cuan.dojo.cc"
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

func (m *MockRouter) Reconcile(ctx context.Context, activeDeployments []*deployer.Deployment) error {
	return nil
}

func (m *MockRouter) AddCustomDomainRoute(ctx context.Context, domain string, target deployer.CustomDomainTarget) error {
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

func TestGenerateRandomSalt(t *testing.T) {
	salt1, err := GenerateRandomSalt(64)
	if err != nil {
		t.Fatalf("GenerateRandomSalt failed: %v", err)
	}
	if len(salt1) != 64 {
		t.Fatalf("expected salt length 64, got %d", len(salt1))
	}

	salt2, err := GenerateRandomSalt(64)
	if err != nil {
		t.Fatalf("GenerateRandomSalt 2 failed: %v", err)
	}
	if salt1 == salt2 {
		t.Fatalf("expected unique salts across invocations, got identical: %s", salt1)
	}

	for _, ch := range salt1 {
		if !strings.ContainsRune(SaltChars, ch) {
			t.Fatalf("unexpected character %q in salt", ch)
		}
	}
}

func TestGenerateWPConfig(t *testing.T) {
	params := WPConfigParams{
		DBName:      "cuan_wp_testdb",
		DBUser:      "cuan_wp_user",
		DBPassword:  "SecretPass123!",
		DBHost:      "127.0.0.1",
		DBPort:      3306,
		TablePrefix: "wp_custom_",
		SiteURL:     "https://blog.cuan.dojo.cc",
		WPDebug:     false,
	}

	content, err := GenerateWPConfigContent(params)
	if err != nil {
		t.Fatalf("GenerateWPConfigContent failed: %v", err)
	}

	requiredSubstrings := []string{
		"define( 'DB_NAME', 'cuan_wp_testdb' );",
		"define( 'DB_USER', 'cuan_wp_user' );",
		"define( 'DB_PASSWORD', 'SecretPass123!' );",
		"define( 'DB_HOST', '127.0.0.1:3306' );",
		"define( 'AUTH_KEY',",
		"define( 'SECURE_AUTH_KEY',",
		"define( 'LOGGED_IN_KEY',",
		"define( 'NONCE_KEY',",
		"$table_prefix = 'wp_custom_';",
		"$_SERVER['HTTP_X_FORWARDED_PROTO'] === 'https'",
		"define( 'WP_HOME', 'https://blog.cuan.dojo.cc' );",
		"require_once ABSPATH . 'wp-settings.php';",
	}

	for _, sub := range requiredSubstrings {
		if !strings.Contains(content, sub) {
			t.Errorf("expected wp-config to contain %q, but not found", sub)
		}
	}

	// Verify writing to disk
	tempDir, err := os.MkdirTemp("", "cuan-wp-config-test-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if err := WriteWPConfig(tempDir, params); err != nil {
		t.Fatalf("WriteWPConfig failed: %v", err)
	}

	readBytes, err := os.ReadFile(filepath.Join(tempDir, "wp-config.php"))
	if err != nil {
		t.Fatalf("read wp-config.php failed: %v", err)
	}
	if !strings.Contains(string(readBytes), "cuan_wp_testdb") {
		t.Errorf("written wp-config.php content mismatch")
	}
}

func TestExtractCore_ZipSlipProtection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-zipslip-test-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "malicious.tar.gz")
	tarFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create tar file failed: %v", err)
	}

	gzWriter := gzip.NewWriter(tarFile)
	tarWriter := tar.NewWriter(gzWriter)

	// Add malicious ZipSlip entry targeting ../../../etc/evil.txt
	maliciousHeader := &tar.Header{
		Name:     "../../evil.txt",
		Mode:     0644,
		Size:     int64(len("malicious content")),
		Typeflag: tar.TypeReg,
	}
	if err := tarWriter.WriteHeader(maliciousHeader); err != nil {
		t.Fatalf("write header failed: %v", err)
	}
	if _, err := tarWriter.Write([]byte("malicious content")); err != nil {
		t.Fatalf("write content failed: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()
	tarFile.Close()

	destDir := filepath.Join(tempDir, "extract_dest")
	_ = os.MkdirAll(destDir, 0755)

	err = ExtractTarGz(archivePath, destDir)
	if err == nil {
		t.Fatalf("expected ZipSlip vulnerability error, got nil")
	}
	if !strings.Contains(err.Error(), "zip slip vulnerability detected") {
		t.Errorf("expected error to mention zip slip vulnerability, got: %v", err)
	}
}

func TestHeadlessBootstrap(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-bootstrap-test-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	params := BootstrapParams{
		TargetDir:          tempDir,
		SiteURL:            "https://mysite.cuan.dojo.cc",
		SiteTitle:          "My Awesome Blog",
		AdminUsername:      "siteadmin",
		AdminPassword:      "StrongPassword999!",
		AdminEmail:         "admin@example.com",
		PermalinkStructure: "/%postname%/",
		MockMode:           true,
	}

	res, err := RunHeadlessBootstrap(context.Background(), params)
	if err != nil {
		t.Fatalf("RunHeadlessBootstrap failed: %v", err)
	}

	if res.SiteTitle != "My Awesome Blog" {
		t.Errorf("expected site title 'My Awesome Blog', got %s", res.SiteTitle)
	}
	if res.PermalinkStructure != "/%postname%/" {
		t.Errorf("expected permalink '/%%postname%%/', got %s", res.PermalinkStructure)
	}

	stateFile := filepath.Join(tempDir, ".cuan-bootstrapped.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Errorf("expected state file %s to be created", stateFile)
	}
}

func TestPortAllocation_NoCollision(t *testing.T) {
	pool := deployer.NewPortPoolWithRange(9000, 9020)
	pool.SetCheckSocket(false) // in-memory testing without socket binding

	var wg sync.WaitGroup
	allocatedPorts := make(map[int]string)
	var mu sync.Mutex

	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			depID := filepath.Join("dep", string(rune('a'+idx)))
			port, err := pool.Allocate(depID)
			if err != nil {
				t.Errorf("allocate failed for %s: %v", depID, err)
				return
			}
			if port < 9000 || port > 9020 {
				t.Errorf("allocated port %d outside range [9000, 9020]", port)
			}

			mu.Lock()
			if existingDep, exists := allocatedPorts[port]; exists {
				t.Errorf("collision detected on port %d between %s and %s", port, existingDep, depID)
			}
			allocatedPorts[port] = depID
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(allocatedPorts) != 15 {
		t.Errorf("expected 15 distinct allocated ports, got %d", len(allocatedPorts))
	}
}

func TestHealthCheck(t *testing.T) {
	// Create mock HTTP server simulating WordPress
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body><h1>WordPress</h1></body></html>"))
	})
	mux.HandleFunc("/wp-login.php", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body><h1>Log In</h1></body></html>"))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Parse host and port from ts.URL
	urlStr := strings.TrimPrefix(ts.URL, "http://")
	parts := strings.Split(urlStr, ":")
	host := parts[0]
	port := 80
	if len(parts) > 1 {
		fmtPort := parts[1]
		p, _ := strings.CutPrefix(fmtPort, "")
		for _, ch := range p {
			port = port*10 + int(ch-'0')
		}
		// Actually let's parse via Sscanf
		var parsedPort int
		if _, err := os.Stdout.WriteString(""); err == nil {
			var pInt int
			for _, r := range parts[1] {
				pInt = pInt*10 + int(r-'0')
			}
			parsedPort = pInt
		}
		port = parsedPort
	}

	client := ts.Client()
	res, err := CheckWordPressHealth(context.Background(), client, host, port, 5*time.Second)
	if err != nil {
		t.Fatalf("CheckWordPressHealth failed: %v", err)
	}
	if !res.Healthy {
		t.Errorf("expected healthy=true, got %v (%s)", res.Healthy, res.Message)
	}
	if res.HomeStatus != http.StatusOK || res.LoginStatus != http.StatusOK {
		t.Errorf("expected HTTP 200 on both home and login, got Home=%d, Login=%d", res.HomeStatus, res.LoginStatus)
	}
}

func TestOrchestrator_Deploy_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-wp-orchestrator-test-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	memRepo := cuanDB.NewMemoryWordPressRepository()
	mockRouter := NewMockRouter()
	portPool := deployer.NewPortPoolWithRange(9100, 9150)
	portPool.SetCheckSocket(false)

	provDBRepo := cuanDB.NewMemoryProvisionedDatabaseRepository()
	dbProv := deployer.NewDatabaseProvisioner(deployer.ProvisionerConfig{
		DBRepo:         provDBRepo,
		VaultSecret:    "test-secret-vault-key-32-chars!",
		ProjectBaseDir: filepath.Join(tempDir, "projects"),
		MockMode:       true,
	})

	orch := NewOrchestrator(Config{
		DBRepo:        memRepo,
		DBProvisioner: dbProv,
		PortPool:      portPool,
		Router:        mockRouter,
		VaultSecret:   "test-secret-vault-key-32-chars!",
		BaseDeployDir: filepath.Join(tempDir, "deployments"),
		CoreCacheDir:  filepath.Join(tempDir, "cache"),
		MockMode:      true,
	})

	ctx := context.Background()
	req := DeployRequest{
		ProjectID:          "p_myblog",
		SiteTitle:          "Tech Insights",
		AdminEmail:         "founder@techinsights.io",
		AdminUsername:      "techadmin",
		PermalinkStructure: "/%postname%/",
		SkipHealthWait:     true,
	}

	result, err := orch.Deploy(ctx, req)
	if err != nil {
		t.Fatalf("orch.Deploy failed: %v", err)
	}

	// 1. Verify Result & DTO
	if result.Deployment == nil {
		t.Fatalf("expected non-nil deployment in result")
	}
	if result.SiteURL != "https://p_myblog.cuan.dojo.cc" {
		t.Errorf("unexpected site URL: %s", result.SiteURL)
	}
	if result.AdminURL != "https://p_myblog.cuan.dojo.cc/wp-login.php" {
		t.Errorf("unexpected admin URL: %s", result.AdminURL)
	}
	if result.Port < 9100 || result.Port > 9150 {
		t.Errorf("allocated port %d outside range [9100, 9150]", result.Port)
	}
	if result.Status != cuanDB.WPStatusRunning {
		t.Errorf("expected status 'running', got %s", result.Status)
	}
	if result.HealthStatus != cuanDB.WPHealthStatusHealthy {
		t.Errorf("expected health 'healthy', got %s", result.HealthStatus)
	}

	// 2. Verify Zero Credential Leakage in DTO
	dto := result.PublicDTO
	if !dto.HasAdminPassword {
		t.Errorf("expected dto.HasAdminPassword=true")
	}
	if !dto.HasDBPassword {
		t.Errorf("expected dto.HasDBPassword=true")
	}
	if result.AdminPasswordPlain == "" {
		t.Errorf("expected plain admin password returned in DeployResult")
	}
	if result.DBPasswordPlain == "" {
		t.Errorf("expected plain db password returned in DeployResult")
	}

	// 3. Verify Decryptability via AES-256-GCM Vault
	decAdminPass, err := result.Deployment.DecryptAdminPassword("test-secret-vault-key-32-chars!")
	if err != nil {
		t.Fatalf("decrypt admin password failed: %v", err)
	}
	if decAdminPass != result.AdminPasswordPlain {
		t.Errorf("admin password mismatch: decrypted %s != plain %s", decAdminPass, result.AdminPasswordPlain)
	}

	// 4. Verify Caddy Route Registered
	customTarget, routeExists := mockRouter.CustomRoutes["p_myblog.cuan.dojo.cc"]
	if !routeExists {
		t.Fatalf("expected Caddy route registered for domain p_myblog.cuan.dojo.cc")
	}
	if customTarget.InternalPort != result.Port {
		t.Errorf("caddy route target port mismatch: %d != %d", customTarget.InternalPort, result.Port)
	}

	// 5. Verify wp-config.php generated on disk
	targetDir := filepath.Join(tempDir, "deployments", result.Deployment.ID)
	wpConfigPath := filepath.Join(targetDir, "wp-config.php")
	configBytes, err := os.ReadFile(wpConfigPath)
	if err != nil {
		t.Fatalf("read wp-config.php failed: %v", err)
	}
	if !strings.Contains(string(configBytes), "define( 'AUTH_KEY',") {
		t.Errorf("wp-config.php missing AUTH_KEY")
	}
	if !strings.Contains(string(configBytes), "define( 'DB_NAME',") {
		t.Errorf("wp-config.php missing DB_NAME")
	}

	// 6. Verify Database Record in Repo
	savedDep, err := memRepo.GetDeploymentByID(ctx, result.Deployment.ID)
	if err != nil {
		t.Fatalf("GetDeploymentByID failed: %v", err)
	}
	if savedDep.SiteTitle != "Tech Insights" {
		t.Errorf("saved site title mismatch: %s", savedDep.SiteTitle)
	}
}

func TestOrchestrator_Lifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-wp-lifecycle-test-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	memRepo := cuanDB.NewMemoryWordPressRepository()
	mockRouter := NewMockRouter()
	portPool := deployer.NewPortPoolWithRange(9200, 9220)
	portPool.SetCheckSocket(false)

	orch := NewOrchestrator(Config{
		DBRepo:        memRepo,
		PortPool:      portPool,
		Router:        mockRouter,
		VaultSecret:   "test-secret-vault-key-32-chars!",
		BaseDeployDir: filepath.Join(tempDir, "deployments"),
		CoreCacheDir:  filepath.Join(tempDir, "cache"),
		MockMode:      true,
	})

	ctx := context.Background()
	req := DeployRequest{
		ProjectID:          "p_lifecycle",
		SiteTitle:          "Lifecycle Blog",
		AdminEmail:         "admin@lifecycle.io",
		AdminUsername:      "wpadmin",
		PermalinkStructure: "/%postname%/",
		SkipHealthWait:     true,
	}

	// 1. Deploy
	res, err := orch.Deploy(ctx, req)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	depID := res.Deployment.ID

	// 2. GetDeployment
	dep, err := orch.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment failed: %v", err)
	}
	if dep.Status != cuanDB.WPStatusRunning {
		t.Errorf("expected status 'running', got %s", dep.Status)
	}

	// 3. HealthCheck
	hcRes, err := orch.HealthCheck(ctx, depID)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
	if !hcRes.Healthy {
		t.Errorf("expected health check healthy=true")
	}

	// 4. RestartDeployment
	restartRes, err := orch.RestartDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("RestartDeployment failed: %v", err)
	}
	if restartRes.Status != cuanDB.WPStatusRunning {
		t.Errorf("expected status 'running' after restart, got %s", restartRes.Status)
	}

	// 5. StopDeployment
	if err := orch.StopDeployment(ctx, depID); err != nil {
		t.Fatalf("StopDeployment failed: %v", err)
	}
	stoppedDep, err := orch.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment after stop failed: %v", err)
	}
	if stoppedDep.Status != cuanDB.WPStatusStopped {
		t.Errorf("expected status 'stopped', got %s", stoppedDep.Status)
	}
	// Verify port released in PortPool
	if portPool.IsLeased(dep.Port) {
		t.Errorf("expected port %d to be released from PortPool", dep.Port)
	}

	// 6. DeleteDeployment
	if err := orch.DeleteDeployment(ctx, depID); err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}
	_, err = orch.GetDeployment(ctx, depID)
	if err == nil {
		t.Fatalf("expected ErrWordPressDeploymentNotFound after deletion, got nil")
	}
}

func TestWordPress_RealPHPServer_HTTP200_HomeAndLogin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-real-php-wp-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create complete mock core in tempDir
	if err := CreateMockWordPressCore(tempDir); err != nil {
		t.Fatalf("CreateMockWordPressCore failed: %v", err)
	}

	params := WPConfigParams{
		DBName:      "cuan_wp_live",
		DBUser:      "wp_user",
		DBPassword:  "password",
		DBHost:      "127.0.0.1",
		DBPort:      3306,
		TablePrefix: "wp_",
		SiteURL:     "http://127.0.0.1",
	}
	if err := WriteWPConfig(tempDir, params); err != nil {
		t.Fatalf("WriteWPConfig failed: %v", err)
	}

	portPool := deployer.NewPortPoolWithRange(9300, 9350)
	port, err := portPool.Allocate("test-real-php-dep")
	if err != nil {
		t.Fatalf("port allocate failed: %v", err)
	}
	defer portPool.Release("test-real-php-dep")

	launcher := &deployer.DefaultProcessLauncher{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd, pid, err := launcher.Start(ctx, tempDir, []string{}, nil, nil, "php", "-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", tempDir)
	if err != nil {
		t.Skipf("skipping real php server test: php not runnable: %v", err)
	}
	defer func() {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	if pid <= 0 {
		t.Fatalf("expected valid PID, got %d", pid)
	}

	// Verify health check probes against real running PHP server
	client := &http.Client{Timeout: 3 * time.Second}
	hcRes, err := CheckWordPressHealth(ctx, client, "127.0.0.1", port, 10*time.Second)
	if err != nil {
		t.Fatalf("CheckWordPressHealth failed on real PHP server: %v", err)
	}

	if !hcRes.Healthy {
		t.Fatalf("expected healthy=true, got message: %s", hcRes.Message)
	}
	if hcRes.HomeStatus != http.StatusOK {
		t.Errorf("expected HomeStatus HTTP 200, got %d", hcRes.HomeStatus)
	}
	if hcRes.LoginStatus != http.StatusOK {
		t.Errorf("expected LoginStatus HTTP 200, got %d", hcRes.LoginStatus)
	}
}


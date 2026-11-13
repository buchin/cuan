package deployer

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

// helper to build in-memory zip archive
func buildZip(t *testing.T, files map[string]string) ([]byte, int64) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content for %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	return buf.Bytes(), int64(buf.Len())
}

// TestZipSlipProtection verifies that ZIP archives containing path traversal payloads
// (e.g. ../../etc/passwd, absolute paths, backslash escapes) are strictly rejected with ErrZipSlip.
func TestZipSlipProtection(t *testing.T) {
	maliciousPayloads := []struct {
		name        string
		entryPath   string
		description string
	}{
		{
			name:        "ParentTraversal",
			entryPath:   "../../etc/passwd",
			description: "Classic relative path traversal to /etc/passwd",
		},
		{
			name:        "NestedParentTraversal",
			entryPath:   "app/nested/../../../../tmp/evil.sh",
			description: "Nested folder traversal escaping extraction root",
		},
		{
			name:        "AbsoluteUnixPath",
			entryPath:   "/etc/shadow",
			description: "Absolute unix path",
		},
		{
			name:        "WindowsBackslashTraversal",
			entryPath:   "..\\..\\windows\\system32\\calc.exe",
			description: "Windows-style backslash traversal",
		},
		{
			name:        "DotDotOnly",
			entryPath:   "..",
			description: "Direct parent reference",
		},
	}

	for _, tc := range maliciousPayloads {
		t.Run(tc.name, func(t *testing.T) {
			destDir := t.TempDir()
			canaryFile := filepath.Join(filepath.Dir(destDir), "canary.txt")
			_ = os.WriteFile(canaryFile, []byte("safe"), 0644)

			zipData, size := buildZip(t, map[string]string{
				tc.entryPath: "MALICIOUS_PAYLOAD",
			})

			readerAt := bytes.NewReader(zipData)
			err := ExtractZip(readerAt, size, destDir)

			if err == nil {
				t.Fatalf("[%s] expected ExtractZip to fail for %s, but got nil", tc.name, tc.entryPath)
			}

			if !errors.Is(err, ErrZipSlip) {
				t.Fatalf("[%s] expected error wrapping ErrZipSlip, got: %v", tc.name, err)
			}

			// Ensure nothing was extracted outside destDir
			canaryContent, err := os.ReadFile(canaryFile)
			if err != nil || string(canaryContent) != "safe" {
				t.Fatalf("[%s] canary file outside extraction target was tampered with!", tc.name)
			}
		})
	}

	t.Run("SafeZipExtractsCleanly", func(t *testing.T) {
		destDir := t.TempDir()
		zipData, size := buildZip(t, map[string]string{
			"index.html":       "<h1>Safe App</h1>",
			"css/style.css":    "body { background: #000; }",
			"js/app.js":        "console.log('safe');",
			"nested/deep/a.txt": "deep content",
		})

		readerAt := bytes.NewReader(zipData)
		err := ExtractZip(readerAt, size, destDir)
		if err != nil {
			t.Fatalf("unexpected error extracting safe zip: %v", err)
		}

		// Verify files exist
		indexContent, err := os.ReadFile(filepath.Join(destDir, "index.html"))
		if err != nil || string(indexContent) != "<h1>Safe App</h1>" {
			t.Fatalf("index.html missing or corrupted")
		}

		cssContent, err := os.ReadFile(filepath.Join(destDir, "css", "style.css"))
		if err != nil || string(cssContent) != "body { background: #000; }" {
			t.Fatalf("css/style.css missing or corrupted")
		}
	})
}

// TestStackAutoDetection verifies accurate detection of PHP, Node.js, and Static HTML runtimes.
func TestStackAutoDetection(t *testing.T) {
	testCases := []struct {
		name            string
		files           map[string]string
		expectedRuntime string
	}{
		{
			name: "PHP_IndexPHP",
			files: map[string]string{
				"index.php": "<?php echo 'Hello PHP'; ?>",
			},
			expectedRuntime: RuntimePHP,
		},
		{
			name: "PHP_WordPress_Config",
			files: map[string]string{
				"wp-config.php": "<?php define('DB_NAME', 'wp'); ?>",
				"functions.php": "<?php // wp theme ?>",
			},
			expectedRuntime: RuntimePHP,
		},
		{
			name: "NodeJS_PackageJSON",
			files: map[string]string{
				"package.json": `{"name":"my-node-app","scripts":{"start":"node index.js"}}`,
				"index.js":     "console.log('Node Server');",
			},
			expectedRuntime: RuntimeNodeJS,
		},
		{
			name: "Static_IndexHTML",
			files: map[string]string{
				"index.html": "<!DOCTYPE html><html><body>Static Web</body></html>",
				"style.css":  "body { color: red; }",
			},
			expectedRuntime: RuntimeStatic,
		},
		{
			name: "Custom_Fallback",
			files: map[string]string{
				"main.go":   "package main\nfunc main() {}",
				"README.md": "# My Custom Go Project",
			},
			expectedRuntime: RuntimeCustom,
		},
		{
			name: "NestedRoot_PHP_Archive",
			files: map[string]string{
				"my-wordpress-theme/index.php": "<?php // nested ?>",
				"my-wordpress-theme/style.css": "/* theme */",
			},
			expectedRuntime: RuntimePHP,
		},
		{
			name: "NestedRoot_NodeJS_Archive",
			files: map[string]string{
				"my-tool/package.json": `{"name":"tool"}`,
				"my-tool/server.js":    `console.log("running");`,
			},
			expectedRuntime: RuntimeNodeJS,
		},
		{
			name: "NestedRoot_Static_Archive",
			files: map[string]string{
				"portfolio-v1/index.html": `<h1>Portfolio</h1>`,
			},
			expectedRuntime: RuntimeStatic,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			destDir := t.TempDir()
			zipData, size := buildZip(t, tc.files)

			err := ExtractZip(bytes.NewReader(zipData), size, destDir)
			if err != nil {
				t.Fatalf("[%s] failed to extract test zip: %v", tc.name, err)
			}

			detected, err := DetectRuntime(destDir)
			if err != nil {
				t.Fatalf("[%s] DetectRuntime error: %v", tc.name, err)
			}

			if detected != tc.expectedRuntime {
				t.Errorf("[%s] expected runtime %s, got %s", tc.name, tc.expectedRuntime, detected)
			}
		})
	}
}

// TestDeployerMigrations verifies that up and down migrations for Deployer (000008)
// load cleanly, declare the full required schema, and preserve 100% backward compatibility
// with Sprint 1 & Sprint 2 database tables.
func TestDeployerMigrations(t *testing.T) {
	migs, err := db.LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations() unexpected error: %v", err)
	}

	if len(migs) < 8 {
		t.Fatalf("expected at least 8 migrations, got %d", len(migs))
	}

	deployerMig := migs[7]
	if deployerMig.Version != "000008" {
		t.Fatalf("expected 8th migration version 000008, got %s", deployerMig.Version)
	}
	if deployerMig.Name != "deployer" {
		t.Fatalf("expected migration name 'deployer', got '%s'", deployerMig.Name)
	}

	upSQL := deployerMig.UpSQL
	downSQL := deployerMig.DownSQL

	// 1. Verify deployments table schema requirements
	requiredDeploymentCols := []string{
		"CREATE TABLE IF NOT EXISTS deployments",
		"id VARCHAR(64) PRIMARY KEY",
		"project_name VARCHAR(255) NOT NULL",
		"slug VARCHAR(128) NOT NULL UNIQUE",
		"source_type VARCHAR(32) NOT NULL DEFAULT 'zip'",
		"source_ref TEXT",
		"runtime VARCHAR(32) NOT NULL DEFAULT 'custom'",
		"status VARCHAR(32) NOT NULL DEFAULT 'pending'",
		"internal_port INTEGER DEFAULT 0",
		"domain VARCHAR(255) NOT NULL DEFAULT ''",
		"env_vars JSONB NOT NULL DEFAULT '{}'::jsonb",
		"process_pid INTEGER DEFAULT 0",
		"created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
		"updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
	}

	for _, col := range requiredDeploymentCols {
		if !strings.Contains(upSQL, col) {
			t.Errorf("UpSQL missing expected deployments declaration chunk: %q", col)
		}
	}

	// 2. Verify deployment_logs table schema requirements
	requiredLogCols := []string{
		"CREATE TABLE IF NOT EXISTS deployment_logs",
		"id BIGSERIAL PRIMARY KEY",
		"deployment_id VARCHAR(64) NOT NULL REFERENCES deployments(id) ON DELETE CASCADE",
		"stream VARCHAR(16) NOT NULL DEFAULT 'stdout'",
		"message TEXT NOT NULL",
		"timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()",
	}

	for _, col := range requiredLogCols {
		if !strings.Contains(upSQL, col) {
			t.Errorf("UpSQL missing expected deployment_logs declaration chunk: %q", col)
		}
	}

	// 3. Verify indexes
	requiredIndexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_deployments_slug ON deployments(slug);",
		"CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status);",
		"CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_timestamp ON deployment_logs(deployment_id, timestamp);",
	}

	for _, idx := range requiredIndexes {
		if !strings.Contains(upSQL, idx) {
			t.Errorf("UpSQL missing required index: %q", idx)
		}
	}

	// 4. Verify DownSQL rollback statements
	if !strings.Contains(downSQL, "DROP TABLE IF EXISTS deployment_logs CASCADE;") {
		t.Errorf("DownSQL missing drop of deployment_logs")
	}
	if !strings.Contains(downSQL, "DROP TABLE IF EXISTS deployments CASCADE;") {
		t.Errorf("DownSQL missing drop of deployments")
	}

	// 5. Backward Compatibility Audit:
	// Verify that UpSQL and DownSQL DO NOT drop or mutate Sprint 1 & Sprint 2 tables
	sprint1And2Tables := []string{
		"owners",
		"owner_sessions",
		"projects",
		"tasks",
		"task_attempts",
		"agent_logs",
		"runners",
		"runner_leases",
		"sponsorship_batches",
		"sponsors",
		"sponsorship_orders",
		"waiting_list_leads",
		"sponsorship_activities",
	}

	for _, tbl := range sprint1And2Tables {
		dropStatement := "DROP TABLE"
		if strings.Contains(strings.ToUpper(upSQL), dropStatement+" IF EXISTS "+strings.ToUpper(tbl)) ||
			strings.Contains(strings.ToUpper(upSQL), dropStatement+" "+strings.ToUpper(tbl)) {
			t.Errorf("CRITICAL VIOLATION: UpSQL attempts to drop historical table %s", tbl)
		}

		if strings.Contains(strings.ToUpper(downSQL), dropStatement+" IF EXISTS "+strings.ToUpper(tbl)) ||
			strings.Contains(strings.ToUpper(downSQL), dropStatement+" "+strings.ToUpper(tbl)) {
			t.Errorf("CRITICAL VIOLATION: DownSQL attempts to drop historical table %s", tbl)
		}
	}

	// 6. Verify DryRun output integrates 000008
	dryUp, err := db.DryRun("up")
	if err != nil {
		t.Fatalf("DryRun(up) error: %v", err)
	}
	if !strings.Contains(dryUp, "[UP] Migration: 000008 (deployer)") {
		t.Errorf("DryRun(up) missing 000008 deployer header")
	}

	dryDown, err := db.DryRun("down")
	if err != nil {
		t.Fatalf("DryRun(down) error: %v", err)
	}
	if !strings.Contains(dryDown, "[DOWN] Migration: 000008 (deployer)") {
		t.Errorf("DryRun(down) missing 000008 deployer header")
	}
}

// TestLocalDriverLifecycle verifies Deploy, Start, Stop, Restart, and GetLogs using LocalDriver.
func TestLocalDriverLifecycle(t *testing.T) {
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("skipping PHP execution test; php binary not found in PATH")
	}
	tempDir := t.TempDir()
	storage, err := NewStorageEngine(filepath.Join(tempDir, "deployments"))
	if err != nil {
		t.Fatalf("NewStorageEngine error: %v", err)
	}

	store := NewMemoryStore()
	driver := NewLocalDriver(storage, store)
	ctx := context.Background()

	// 1. Deploy PHP project via ZIP
	zipData, size := buildZip(t, map[string]string{
		"index.php": "<?php echo 'Hello from Cuan Deployer'; ?>",
	})

	req := DeploymentRequest{
		ProjectName:  "Test WordPress Site",
		Slug:         "test-wp",
		SourceType:   SourceTypeZip,
		ZipReader:    bytes.NewReader(zipData),
		ZipSize:      size,
		InternalPort: 9001,
		Domain:       "test-wp.cuan.dojo.cc",
		EnvVars: map[string]interface{}{
			"APP_ENV": "production",
		},
	}

	res, err := driver.Deploy(ctx, req)
	if err != nil {
		t.Fatalf("Deploy error: %v", err)
	}

	if res.DetectedRuntime != RuntimePHP {
		t.Errorf("expected detected runtime %s, got %s", RuntimePHP, res.DetectedRuntime)
	}
	if res.Deployment.Status != StatusPending {
		t.Errorf("expected initial status %s, got %s", StatusPending, res.Deployment.Status)
	}

	depID := res.Deployment.ID

	// Verify extracted files in data/deployments/<id>/app/
	extractedIndex := filepath.Join(storage.GetAppDir(depID), "index.php")
	if !FileExists(extractedIndex) {
		t.Fatalf("extracted index.php not found at %s", extractedIndex)
	}

	// 2. Start deployment
	if err := driver.Start(ctx, depID); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	startedDep, err := store.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment error: %v", err)
	}
	if startedDep.Status != StatusRunning {
		t.Errorf("expected status %s, got %s", StatusRunning, startedDep.Status)
	}

	// 3. Stop deployment
	if err := driver.Stop(ctx, depID); err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	stoppedDep, err := store.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment error: %v", err)
	}
	if stoppedDep.Status != StatusStopped {
		t.Errorf("expected status %s, got %s", StatusStopped, stoppedDep.Status)
	}

	// 4. Restart deployment
	if err := driver.Restart(ctx, depID); err != nil {
		t.Fatalf("Restart error: %v", err)
	}

	restartedDep, err := store.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment error: %v", err)
	}
	if restartedDep.Status != StatusRunning {
		t.Errorf("expected status %s, got %s", StatusRunning, restartedDep.Status)
	}

	// 5. Inspect logs
	logs, err := driver.GetLogs(ctx, depID, 50)
	if err != nil {
		t.Fatalf("GetLogs error: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected logs to be recorded, got 0")
	}

	// Check that system events were recorded
	var foundInit, foundStart, foundStop bool
	for _, l := range logs {
		if strings.Contains(l.Message, "initialized") {
			foundInit = true
		}
		if strings.Contains(l.Message, "started") {
			foundStart = true
		}
		if strings.Contains(l.Message, "stopped") {
			foundStop = true
		}
	}

	if !foundInit || !foundStart || !foundStop {
		t.Errorf("missing expected lifecycle logs: init=%v, start=%v, stop=%v", foundInit, foundStart, foundStop)
	}

	_ = driver.Stop(ctx, depID)
}

// TestStorageEngine_Isolation verifies that deployments are completely isolated in their subdirectories.
func TestStorageEngine_Isolation(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewStorageEngine(filepath.Join(tempDir, "deployments"))
	if err != nil {
		t.Fatalf("NewStorageEngine error: %v", err)
	}

	dep1 := "dep_alpha"
	dep2 := "dep_beta"

	dir1, err := storage.PrepareDeploymentDir(dep1)
	if err != nil {
		t.Fatalf("PrepareDeploymentDir(%s) error: %v", dep1, err)
	}

	dir2, err := storage.PrepareDeploymentDir(dep2)
	if err != nil {
		t.Fatalf("PrepareDeploymentDir(%s) error: %v", dep2, err)
	}

	// Write file in dep1
	file1 := filepath.Join(dir1, "app.txt")
	_ = os.WriteFile(file1, []byte("alpha data"), 0644)

	// Verify dep2 does not have app.txt
	file2 := filepath.Join(dir2, "app.txt")
	if FileExists(file2) {
		t.Fatalf("cross-contamination: dep2 contains file from dep1")
	}

	// Remove dep1
	if err := storage.DeleteDeploymentDir(dep1); err != nil {
		t.Fatalf("DeleteDeploymentDir error: %v", err)
	}

	if DirExists(dir1) {
		t.Fatalf("dep1 directory still exists after deletion")
	}
	if !DirExists(dir2) {
		t.Fatalf("dep2 directory was unexpectedly removed")
	}
}

// TestPortPool_ZeroCollision tests concurrent port allocation to guarantee thread safety,
// anti-collision, real socket availability, and port pool exhaustion handling.
func TestPortPool_ZeroCollision(t *testing.T) {
	pool := NewPortPool()
	concurrency := 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	portsChan := make(chan int, concurrency)
	errorsChan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		depID := fmt.Sprintf("dep_concurrent_%03d", i)
		go func(id string) {
			defer wg.Done()
			port, err := pool.Allocate(id)
			if err != nil {
				errorsChan <- err
				return
			}
			portsChan <- port
		}(depID)
	}

	wg.Wait()
	close(portsChan)
	close(errorsChan)

	if len(errorsChan) > 0 {
		for err := range errorsChan {
			t.Errorf("concurrent allocate error: %v", err)
		}
		t.FailNow()
	}

	// Verify all allocated ports are unique (zero collision) and in range 9000-9999
	seen := make(map[int]bool)
	for port := range portsChan {
		if port < DefaultMinPort || port > DefaultMaxPort {
			t.Errorf("allocated port %d outside range [%d, %d]", port, DefaultMinPort, DefaultMaxPort)
		}
		if seen[port] {
			t.Fatalf("PORT COLLISION DETECTED: port %d allocated more than once!", port)
		}
		seen[port] = true
	}

	if len(seen) != concurrency {
		t.Fatalf("expected %d unique ports allocated, got %d", concurrency, len(seen))
	}

	// Verify IsLeased
	for port := range seen {
		if !pool.IsLeased(port) {
			t.Errorf("expected port %d to be marked as leased", port)
		}
	}

	// Release all ports
	for i := 0; i < concurrency; i++ {
		depID := fmt.Sprintf("dep_concurrent_%03d", i)
		port, released := pool.Release(depID)
		if !released {
			t.Errorf("failed to release port for %s", depID)
		}
		if pool.IsLeased(port) {
			t.Errorf("port %d still marked as leased after release", port)
		}
	}

	if len(pool.ActiveLeases()) != 0 {
		t.Errorf("expected active leases to be 0 after full release, got %d", len(pool.ActiveLeases()))
	}

	// Test pool exhaustion with a small pool range
	t.Run("PoolExhaustionHandling", func(t *testing.T) {
		smallPool := NewPortPoolWithRange(9500, 9502) // exactly 3 ports
		smallPool.SetCheckSocket(false)               // test logical pool boundary

		p1, err := smallPool.Allocate("dep_1")
		if err != nil || p1 != 9500 {
			t.Fatalf("alloc dep_1 failed: %v, port=%d", err, p1)
		}
		p2, err := smallPool.Allocate("dep_2")
		if err != nil || p2 != 9501 {
			t.Fatalf("alloc dep_2 failed: %v, port=%d", err, p2)
		}
		p3, err := smallPool.Allocate("dep_3")
		if err != nil || p3 != 9502 {
			t.Fatalf("alloc dep_3 failed: %v, port=%d", err, p3)
		}

		// 4th allocate must return ErrPortPoolExhausted
		_, err = smallPool.Allocate("dep_4")
		if !errors.Is(err, ErrPortPoolExhausted) {
			t.Fatalf("expected ErrPortPoolExhausted, got %v", err)
		}

		// Release dep_2, then allocate dep_4 should succeed
		smallPool.Release("dep_2")
		p4, err := smallPool.Allocate("dep_4")
		if err != nil || p4 != 9501 {
			t.Fatalf("alloc dep_4 after release failed: %v, port=%d", err, p4)
		}
	})
}

// TestLocalDriver_Lifecycle tests the full lifecycle (Start -> Running -> Health -> Stop -> Stopped)
// across Static, PHP, and Node.js runtimes, including dynamic port allocation, HTTP responsiveness,
// and graceful process termination.
func TestLocalDriver_Lifecycle(t *testing.T) {
	ctx := context.Background()

	t.Run("StaticRuntime_Lifecycle", func(t *testing.T) {
		tempDir := t.TempDir()
		storage, err := NewStorageEngine(filepath.Join(tempDir, "deployments"))
		if err != nil {
			t.Fatalf("NewStorageEngine: %v", err)
		}
		store := NewMemoryStore()
		driver := NewLocalDriver(storage, store)

		zipData, size := buildZip(t, map[string]string{
			"index.html": "<!DOCTYPE html><html><body><h1>Cuan Static Runner Test</h1></body></html>",
			"test.txt":   "static file content",
		})

		req := DeploymentRequest{
			ProjectName: "Static Landing",
			Slug:        "static-landing",
			SourceType:  SourceTypeZip,
			ZipReader:   bytes.NewReader(zipData),
			ZipSize:     size,
		}

		res, err := driver.Deploy(ctx, req)
		if err != nil {
			t.Fatalf("Deploy error: %v", err)
		}
		if res.DetectedRuntime != RuntimeStatic {
			t.Fatalf("expected runtime %s, got %s", RuntimeStatic, res.DetectedRuntime)
		}
		depID := res.Deployment.ID

		// 1. Start: transitions to Running, allocates port in 9000-9999, spawns process PID > 0
		if err := driver.Start(ctx, depID); err != nil {
			t.Fatalf("Start error: %v", err)
		}

		dep, err := store.GetDeployment(ctx, depID)
		if err != nil {
			t.Fatalf("GetDeployment error: %v", err)
		}
		if dep.Status != StatusRunning {
			t.Errorf("expected status %s, got %s", StatusRunning, dep.Status)
		}
		if dep.ProcessPID <= 0 {
			t.Errorf("expected ProcessPID > 0, got %d", dep.ProcessPID)
		}
		if dep.InternalPort < DefaultMinPort || dep.InternalPort > DefaultMaxPort {
			t.Errorf("expected InternalPort in range [%d, %d], got %d", DefaultMinPort, DefaultMaxPort, dep.InternalPort)
		}

		// 2. Health: process alive, port responding, responds to HTTP GET
		hs, err := driver.Health(ctx, depID)
		if err != nil {
			t.Fatalf("Health error: %v", err)
		}
		if !hs.Alive {
			t.Errorf("expected health Alive=true, got false (err: %s)", hs.Error)
		}
		if hs.PID != dep.ProcessPID {
			t.Errorf("health PID mismatch: expected %d, got %d", dep.ProcessPID, hs.PID)
		}

		// Verify HTTP response from the running static server
		httpURL := fmt.Sprintf("http://127.0.0.1:%d/index.html", dep.InternalPort)
		client := &http.Client{Timeout: 2 * time.Second}
		resp, httpErr := client.Get(httpURL)
		if httpErr != nil {
			t.Fatalf("HTTP GET %s failed: %v", httpURL, httpErr)
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(bodyBytes), "Cuan Static Runner Test") {
			t.Errorf("unexpected HTTP response body: %s", string(bodyBytes))
		}

		// 3. Restart: stops and starts cleanly
		if err := driver.Restart(ctx, depID); err != nil {
			t.Fatalf("Restart error: %v", err)
		}
		restartedDep, _ := store.GetDeployment(ctx, depID)
		if restartedDep.Status != StatusRunning || restartedDep.ProcessPID <= 0 {
			t.Errorf("restart failed: status=%s, pid=%d", restartedDep.Status, restartedDep.ProcessPID)
		}

		// 4. Stop: terminates process, releases port, sets status 'stopped'
		if err := driver.Stop(ctx, depID); err != nil {
			t.Fatalf("Stop error: %v", err)
		}

		stoppedDep, _ := store.GetDeployment(ctx, depID)
		if stoppedDep.Status != StatusStopped {
			t.Errorf("expected status %s, got %s", StatusStopped, stoppedDep.Status)
		}
		if stoppedDep.ProcessPID != 0 {
			t.Errorf("expected ProcessPID=0, got %d", stoppedDep.ProcessPID)
		}

		// Verify Health after stop reports not alive
		hsAfterStop, _ := driver.Health(ctx, depID)
		if hsAfterStop != nil && hsAfterStop.Alive {
			t.Errorf("expected health Alive=false after stop, got true")
		}

		// Verify port is released in pool
		if driver.PortPool().IsLeased(dep.InternalPort) {
			t.Errorf("expected port %d to be released from pool", dep.InternalPort)
		}
	})

	t.Run("PHPRuntime_Lifecycle", func(t *testing.T) {
		if _, err := exec.LookPath("php"); err != nil {
			t.Skip("skipping PHP execution test; php binary not found in PATH")
		}
		tempDir := t.TempDir()
		storage, err := NewStorageEngine(filepath.Join(tempDir, "deployments"))
		if err != nil {
			t.Fatalf("NewStorageEngine: %v", err)
		}
		store := NewMemoryStore()
		driver := NewLocalDriver(storage, store)

		zipData, size := buildZip(t, map[string]string{
			"index.php": "<?php echo 'PHP Runner Lifecycle OK'; ?>",
		})

		req := DeploymentRequest{
			ProjectName: "PHP Script",
			Slug:        "php-script",
			SourceType:  SourceTypeZip,
			ZipReader:   bytes.NewReader(zipData),
			ZipSize:     size,
		}

		res, err := driver.Deploy(ctx, req)
		if err != nil {
			t.Fatalf("Deploy error: %v", err)
		}
		if res.DetectedRuntime != RuntimePHP {
			t.Fatalf("expected runtime %s, got %s", RuntimePHP, res.DetectedRuntime)
		}
		depID := res.Deployment.ID

		// Start PHP process
		if err := driver.Start(ctx, depID); err != nil {
			t.Fatalf("Start PHP error: %v", err)
		}

		dep, _ := store.GetDeployment(ctx, depID)
		if dep.Status != StatusRunning || dep.ProcessPID <= 0 {
			t.Fatalf("PHP not running: status=%s, pid=%d", dep.Status, dep.ProcessPID)
		}

		// Verify health & HTTP response
		hs, err := driver.Health(ctx, depID)
		if err != nil || !hs.Alive {
			t.Fatalf("Health check failed for PHP runner: %v, hs=%+v", err, hs)
		}

		httpURL := fmt.Sprintf("http://127.0.0.1:%d/index.php", dep.InternalPort)
		client := &http.Client{Timeout: 2 * time.Second}
		resp, httpErr := client.Get(httpURL)
		if httpErr != nil {
			t.Fatalf("HTTP GET PHP %s failed: %v", httpURL, httpErr)
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(bodyBytes), "PHP Runner Lifecycle OK") {
			t.Errorf("unexpected PHP response: %s", string(bodyBytes))
		}

		// Stop PHP process
		if err := driver.Stop(ctx, depID); err != nil {
			t.Fatalf("Stop PHP error: %v", err)
		}

		stoppedDep, _ := store.GetDeployment(ctx, depID)
		if stoppedDep.Status != StatusStopped {
			t.Errorf("expected stopped status, got %s", stoppedDep.Status)
		}
	})

	t.Run("NodeJSRuntime_Lifecycle", func(t *testing.T) {
		tempDir := t.TempDir()
		storage, err := NewStorageEngine(filepath.Join(tempDir, "deployments"))
		if err != nil {
			t.Fatalf("NewStorageEngine: %v", err)
		}
		store := NewMemoryStore()
		driver := NewLocalDriver(storage, store)

		serverJS := `
const http = require('http');
const port = process.env.PORT || 9000;
const server = http.createServer((req, res) => {
  res.writeHead(200, {'Content-Type': 'text/plain'});
  res.end('Node Runner Lifecycle OK');
});
server.listen(port, '127.0.0.1');
`
		zipData, size := buildZip(t, map[string]string{
			"package.json": `{"name":"node-app","scripts":{"start":"node server.js"}}`,
			"server.js":    serverJS,
		})

		req := DeploymentRequest{
			ProjectName: "Node App",
			Slug:        "node-app",
			SourceType:  SourceTypeZip,
			ZipReader:   bytes.NewReader(zipData),
			ZipSize:     size,
		}

		res, err := driver.Deploy(ctx, req)
		if err != nil {
			t.Fatalf("Deploy error: %v", err)
		}
		if res.DetectedRuntime != RuntimeNodeJS {
			t.Fatalf("expected runtime %s, got %s", RuntimeNodeJS, res.DetectedRuntime)
		}
		depID := res.Deployment.ID

		// Start Node.js process
		if err := driver.Start(ctx, depID); err != nil {
			t.Fatalf("Start Node error: %v", err)
		}

		dep, _ := store.GetDeployment(ctx, depID)
		if dep.Status != StatusRunning || dep.ProcessPID <= 0 {
			t.Fatalf("Node not running: status=%s, pid=%d", dep.Status, dep.ProcessPID)
		}

		// Verify health & HTTP response
		hs, err := driver.Health(ctx, depID)
		if err != nil || !hs.Alive {
			t.Fatalf("Health check failed for Node runner: %v, hs=%+v", err, hs)
		}

		httpURL := fmt.Sprintf("http://127.0.0.1:%d/", dep.InternalPort)
		client := &http.Client{Timeout: 2 * time.Second}
		resp, httpErr := client.Get(httpURL)
		if httpErr != nil {
			t.Fatalf("HTTP GET Node %s failed: %v", httpURL, httpErr)
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(bodyBytes), "Node Runner Lifecycle OK") {
			t.Errorf("unexpected Node response: %s", string(bodyBytes))
		}

		// Stop Node.js process
		if err := driver.Stop(ctx, depID); err != nil {
			t.Fatalf("Stop Node error: %v", err)
		}

		stoppedDep, _ := store.GetDeployment(ctx, depID)
		if stoppedDep.Status != StatusStopped {
			t.Errorf("expected stopped status, got %s", stoppedDep.Status)
		}
	})
}

// TestLogStreamer verifies that process stdout and stderr streams are captured in real-time,
// stored in both the in-memory circular buffer and persistent database store, and broadcasted
// to active subscriber channels.
func TestLogStreamer(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// Create dummy deployment in store to satisfy foreign key in append
	dep := &Deployment{
		ID:          "dep_stream_test",
		ProjectName: "Stream Test",
		Slug:        "stream-test",
		Runtime:     RuntimeStatic,
		Status:      StatusRunning,
	}
	if err := store.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("CreateDeployment error: %v", err)
	}

	streamer := NewLogStreamer("dep_stream_test", store, 20)
	defer streamer.Close()

	// 1. Subscribe to live stream channel
	subCh, unsub := streamer.Subscribe()
	defer unsub()

	// 2. Stream simulated stdout lines
	stdoutR, stdoutW := io.Pipe()
	go func() {
		defer stdoutW.Close()
		for i := 1; i <= 5; i++ {
			_, _ = fmt.Fprintf(stdoutW, "stdout log line %d\n", i)
		}
	}()

	if err := streamer.Stream(ctx, StreamStdout, stdoutR); err != nil {
		t.Fatalf("Stream stdout: %v", err)
	}

	// 3. Stream simulated stderr lines
	stderrR, stderrW := io.Pipe()
	go func() {
		defer stderrW.Close()
		for i := 1; i <= 3; i++ {
			_, _ = fmt.Fprintf(stderrW, "stderr error line %d\n", i)
		}
	}()

	if err := streamer.Stream(ctx, StreamStderr, stderrR); err != nil {
		t.Fatalf("Stream stderr: %v", err)
	}

	// 4. Verify subscriber received all 8 entries
	receivedCount := 0
	timeout := time.After(1 * time.Second)
	for receivedCount < 8 {
		select {
		case entry := <-subCh:
			receivedCount++
			if entry.DeploymentID != "dep_stream_test" {
				t.Errorf("unexpected deployment ID in entry: %s", entry.DeploymentID)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for subscriber entries: got %d/8", receivedCount)
		}
	}

	// 5. Verify in-memory buffer via GetRecent
	recentLogs := streamer.GetRecent(10)
	if len(recentLogs) != 8 {
		t.Fatalf("expected 8 recent logs in buffer, got %d", len(recentLogs))
	}

	stdoutFound := 0
	stderrFound := 0
	for _, l := range recentLogs {
		if l.Stream == StreamStdout {
			stdoutFound++
		} else if l.Stream == StreamStderr {
			stderrFound++
		}
	}
	if stdoutFound != 5 || stderrFound != 3 {
		t.Errorf("buffer stream count mismatch: stdout=%d, stderr=%d", stdoutFound, stderrFound)
	}

	// 6. Verify persistence in Store
	persistedLogs, err := store.GetLogs(ctx, "dep_stream_test", 50)
	if err != nil {
		t.Fatalf("store.GetLogs: %v", err)
	}
	if len(persistedLogs) != 8 {
		t.Fatalf("expected 8 persisted logs in store, got %d", len(persistedLogs))
	}
}


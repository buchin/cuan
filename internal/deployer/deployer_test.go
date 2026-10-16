package deployer

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	logs, err := driver.GetLogs(ctx, depID, 10)
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

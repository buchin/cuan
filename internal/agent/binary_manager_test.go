package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryManager_DetectionAndInstall(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-binary-mgr-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewBinaryManager(tempDir)

	// 1. Initially missing
	status := mgr.DetectHarness(HarnessClaude)
	if status.Installed {
		t.Errorf("expected claude to not be installed initially in %s", tempDir)
	}
	if status.Source != "missing" && status.Source != "system_path" {
		t.Errorf("unexpected initial source: %s", status.Source)
	}

	// 2. 1-Click Bootstrap Install
	ctx := context.Background()
	installedStat, err := mgr.InstallBinary(ctx, HarnessClaude)
	if err != nil {
		t.Fatalf("InstallBinary(claude) failed: %v", err)
	}
	if !installedStat.Installed {
		t.Errorf("expected installed to be true after bootstrap")
	}
	if installedStat.Source != "user_local_bin" {
		t.Errorf("expected source user_local_bin, got %s", installedStat.Source)
	}

	// Check file on disk
	binPath := filepath.Join(tempDir, "claude")
	fi, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("stat failed on installed binary: %v", err)
	}
	if fi.Mode()&0111 == 0 {
		t.Errorf("expected executable permissions on %s, got mode %v", binPath, fi.Mode())
	}

	// 3. Re-detect harness
	recheck := mgr.DetectHarness(HarnessClaude)
	if !recheck.Installed || recheck.Source != "user_local_bin" {
		t.Errorf("expected re-detected binary to be installed from user_local_bin, got %+v", recheck)
	}
	if !strings.Contains(recheck.Version, "claude-code") {
		t.Errorf("unexpected version string: %s", recheck.Version)
	}

	// 4. Install Codex & AGY
	for _, h := range []string{HarnessCodex, HarnessAGY} {
		stat, err := mgr.InstallBinary(ctx, h)
		if err != nil {
			t.Fatalf("InstallBinary(%s) failed: %v", h, err)
		}
		if !stat.Installed {
			t.Errorf("expected %s to be installed", h)
		}
	}

	all := mgr.DetectAllHarnesses()
	if len(all) != 3 {
		t.Errorf("expected 3 harnesses detected, got %d", len(all))
	}
}

func TestBinaryManager_HeadlessRunAndTimeout(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-binary-run-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewBinaryManager(tempDir)
	ctx := context.Background()

	// Install Claude runner
	if _, err := mgr.InstallBinary(ctx, HarnessClaude); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Normal execution
	res, err := mgr.RunHeadless(ctx, HeadlessRunRequest{
		Harness: HarnessClaude,
		Prompt:  "Hello from Agent test",
		Secrets: []string{"SUPER_SECRET_1234"},
	})
	if err != nil {
		t.Fatalf("RunHeadless failed: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Output, "Hello from Agent test") {
		t.Errorf("expected output to contain prompt, got %q", res.Output)
	}

	// Test hanging process with timeout safety
	hangingScript := `#!/usr/bin/env sh
if [ "$1" = "--version" ]; then
  echo "hanging-cli 1.0.0"
  exit 0
fi
sleep 10
exit 0
`
	hangingPath := filepath.Join(tempDir, "codex")
	if err := os.WriteFile(hangingPath, []byte(hangingScript), 0755); err != nil {
		t.Fatalf("write hanging script failed: %v", err)
	}

	timeoutCtx := context.Background()
	shortTimeout := 150 * time.Millisecond
	_, err = mgr.RunHeadless(timeoutCtx, HeadlessRunRequest{
		Harness: HarnessCodex,
		Prompt:  "This will time out",
		Timeout: shortTimeout,
	})
	if err != ErrExecutionTimeout {
		t.Errorf("expected ErrExecutionTimeout, got %v", err)
	}
}

func TestBinaryManager_ExecuteWithFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-binary-fallback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewBinaryManager(tempDir)
	ctx := context.Background()

	// Mock BYOK Client
	byok := NewUniversalBYOKProvider(ProviderDeepSeek)
	byok.SetOfflineMock(true)

	// Harness 'claude' is not installed in tempDir and we ensure missing
	res, err := mgr.ExecuteWithFallback(ctx, HeadlessRunRequest{
		Harness: "uninstalled-harness",
		Prompt:  "What is the capital of Indonesia?",
	}, byok, "mock_key")

	if err != nil {
		t.Fatalf("ExecuteWithFallback failed: %v", err)
	}
	if !res.FallbackUsed {
		t.Errorf("expected FallbackUsed to be true")
	}
	if !strings.Contains(res.Output, "Mock Response") {
		t.Errorf("expected BYOK fallback response, got %q", res.Output)
	}
}

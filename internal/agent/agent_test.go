package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaskSecrets(t *testing.T) {
	secrets := []string{"ghp_secretToken1234567890", "super-secret-password"}
	input := "Connected using token ghp_secretToken1234567890 and password super-secret-password successfully."
	masked := MaskSecrets(input, secrets)

	if strings.Contains(masked, "ghp_secretToken1234567890") {
		t.Errorf("token was not masked: %s", masked)
	}
	if strings.Contains(masked, "super-secret-password") {
		t.Errorf("password was not masked: %s", masked)
	}
	if !strings.Contains(masked, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output: %s", masked)
	}
}

func TestAGYAdapter_Basics(t *testing.T) {
	adapter := fixtureAGYAdapter(t, "printf 'agy-fixture 1.0\\n'\n")

	if adapter.Name() != "agy" {
		t.Errorf("expected adapter name 'agy', got '%s'", adapter.Name())
	}

	version, err := adapter.Version()
	if err != nil {
		t.Fatalf("adapter.Version() failed: %v", err)
	}
	if version == "" {
		t.Errorf("expected non-empty version string")
	}
	t.Logf("Detected AGY Adapter version: %s", version)
}

func TestAGYAdapter_SpikeFixtureEditAndSecrets(t *testing.T) {
	adapter := fixtureAGYAdapter(t, "printf 'spike-edited-by-agy' > fixture.txt\nprintf 'SENSITIVE_SECRET_TOKEN_9999\\n'\n")
	result := verifyAGYFixtureEditAndSecrets(t, adapter)
	if !strings.Contains(result.Stdout, "[REDACTED]") {
		t.Fatal("fixture output did not exercise secret redaction")
	}
}

// Provider-backed execution is intentionally separate from ordinary package
// tests: an installed CLI must never silently consume another agent's quota.
func TestAGYAdapter_LiveSpikeFixtureEditAndSecrets(t *testing.T) {
	if os.Getenv("CUAN_TEST_LIVE_AGY") != "1" {
		t.Skip("explicit live provider opt-in required: CUAN_TEST_LIVE_AGY=1")
	}
	adapter, err := NewAGYAdapter(os.Getenv("CUAN_TEST_LIVE_AGY_BINARY"))
	if err != nil {
		t.Fatal(err)
	}
	verifyAGYFixtureEditAndSecrets(t, adapter)
}

func verifyAGYFixtureEditAndSecrets(t *testing.T, adapter *AGYAdapter) *ExecutionResult {
	t.Helper()

	// Create temporary fixture git repo
	tmpDir, err := os.MkdirTemp("", "cuan-spike-fixture-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Init git repo
	if err := exec.Command("git", "-C", tmpDir, "init").Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	fixtureFile := filepath.Join(tmpDir, "fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte("initial-content\n"), 0644); err != nil {
		t.Fatalf("write fixture file failed: %v", err)
	}

	secretValue := "SENSITIVE_SECRET_TOKEN_9999"

	req := ExecutionRequest{
		TaskID:     "spike-task-01",
		Prompt:     "Edit the file fixture.txt and change its content to exactly 'spike-edited-by-agy'. Do not mention SENSITIVE_SECRET_TOKEN_9999.",
		WorkingDir: tmpDir,
		Secrets:    []string{secretValue},
		Timeout:    90 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	res, err := adapter.Execute(ctx, req)
	if err != nil {
		t.Fatalf("adapter.Execute failed: %v", err)
	}

	t.Logf("Spike Result: Status=%s, ExitCode=%d, Duration=%v, PeakRSS=%d bytes",
		res.Status, res.ExitCode, res.Duration, res.PeakRSSBytes)

	// Verify secrets are redacted from output
	if strings.Contains(res.Stdout, secretValue) || strings.Contains(res.Stderr, secretValue) {
		t.Errorf("secret leaked in stdout/stderr!")
	}

	// Verify duration and memory measured
	if res.Duration <= 0 {
		t.Errorf("expected duration > 0, got %v", res.Duration)
	}

	// Read fixture file to verify edit occurred
	updated, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatalf("failed to read fixture file: %v", err)
	}
	t.Logf("Updated fixture content: %q", string(updated))
	if !strings.Contains(string(updated), "spike-edited-by-agy") {
		t.Errorf("expected fixture file to be edited by agent, got %q", string(updated))
	}
	return res
}

func TestAGYAdapter_Cancelation(t *testing.T) {
	adapter := fixtureAGYAdapter(t, "exec sleep 30\n")

	tmpDir, err := os.MkdirTemp("", "cuan-cancel-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel almost immediately after start
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	req := ExecutionRequest{
		TaskID:     "cancel-task-01",
		Prompt:     "Analyze all system files in depth and produce a 1000-line report on kernel configurations.",
		WorkingDir: tmpDir,
	}

	start := time.Now()
	res, err := adapter.Execute(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error on cancelation, got: %v", err)
	}

	t.Logf("Cancelation test finished in %v: Status=%s, ExitCode=%d", elapsed, res.Status, res.ExitCode)

	if res.Status != "CANCELED" {
		t.Errorf("expected Status CANCELED, got %s", res.Status)
	}
	if res.ExitCode != -1 {
		t.Errorf("expected ExitCode -1 for canceled task, got %d", res.ExitCode)
	}
}

func fixtureAGYAdapter(t *testing.T, body string) *AGYAdapter {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "agy-fixture")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nset -eu\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewAGYAdapter(binary)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

package terminal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindShell(t *testing.T) {
	sh := FindShell("")
	if sh == "" {
		t.Fatal("expected non-empty shell path")
	}
	if !strings.HasSuffix(sh, "bash") && !strings.HasSuffix(sh, "sh") {
		t.Fatalf("expected bash or sh, got: %s", sh)
	}
}

func TestSanitizeEnvironment(t *testing.T) {
	custom := []string{
		"MY_CUSTOM_VAR=hello",
		"AWS_SECRET_ACCESS_KEY=supersecret",
		"DATABASE_URL=postgres://user:pass@localhost:5432/db",
		"RESEND_KEY=re_12345",
		"SAFE_FEATURE_FLAG=true",
		"COMMAND_CODE_API_KEY=cmd_test_key_123",
	}

	env := SanitizeEnvironment(custom, "task-123", "/tmp/workspace-123")

	joined := strings.Join(env, ";")

	// Verify required safe variables are present
	if !strings.Contains(joined, "TASK_ID=task-123") {
		t.Errorf("missing TASK_ID in sanitized env: %s", joined)
	}
	if !strings.Contains(joined, "TERM=xterm-256color") {
		t.Errorf("missing TERM in sanitized env: %s", joined)
	}
	if !strings.Contains(joined, "PWD=/tmp/workspace-123") {
		t.Errorf("missing PWD in sanitized env: %s", joined)
	}
	if !strings.Contains(joined, "MY_CUSTOM_VAR=hello") {
		t.Errorf("missing safe custom variable in sanitized env: %s", joined)
	}
	if !strings.Contains(joined, "SAFE_FEATURE_FLAG=true") {
		t.Errorf("missing SAFE_FEATURE_FLAG in sanitized env: %s", joined)
	}
	if !strings.Contains(joined, "COMMAND_CODE_API_KEY=cmd_test_key_123") {
		t.Errorf("missing delegated COMMAND_CODE_API_KEY in sanitized env: %s", joined)
	}

	// Verify sensitive variables are stripped out
	if strings.Contains(joined, "supersecret") || strings.Contains(joined, "AWS_SECRET_ACCESS_KEY") {
		t.Errorf("SECURITY LEAK: AWS_SECRET_ACCESS_KEY found in sanitized env: %s", joined)
	}
	if strings.Contains(joined, "DATABASE_URL") {
		t.Errorf("SECURITY LEAK: DATABASE_URL found in sanitized env: %s", joined)
	}
	if strings.Contains(joined, "RESEND_KEY") {
		t.Errorf("SECURITY LEAK: RESEND_KEY found in sanitized env: %s", joined)
	}
}

func TestSpawnPTY_BasicAndIO(t *testing.T) {
	tempDir := t.TempDir()

	cfg := PTYConfig{
		TaskID:      "test-task-1",
		WorkDir:     tempDir,
		InitialCols: 80,
		InitialRows: 24,
	}

	proc, err := SpawnPTY(context.Background(), cfg)
	if err != nil {
		t.Fatalf("SpawnPTY failed: %v", err)
	}
	defer proc.Close()

	if proc.PID() <= 0 {
		t.Errorf("expected positive PID, got %d", proc.PID())
	}
	if proc.TaskID() != "test-task-1" {
		t.Errorf("expected taskID test-task-1, got %s", proc.TaskID())
	}

	// Write simple echo command
	cmdStr := "echo 'CUAN_PTY_OK'\n"
	_, err = proc.Write([]byte(cmdStr))
	if err != nil {
		t.Fatalf("write to pty: %v", err)
	}

	// Read output until marker is received or timeout
	outputBuf := make([]byte, 4096)
	found := false
	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		n, rErr := proc.Read(outputBuf)
		if n > 0 {
			s := string(outputBuf[:n])
			if strings.Contains(s, "CUAN_PTY_OK") {
				found = true
				break
			}
		}
		if rErr != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !found {
		t.Fatalf("did not observe 'CUAN_PTY_OK' in PTY output within timeout")
	}
}

func TestSpawnPTY_ResizeWindow(t *testing.T) {
	tempDir := t.TempDir()

	cfg := PTYConfig{
		TaskID:      "test-task-resize",
		WorkDir:     tempDir,
		InitialCols: 80,
		InitialRows: 24,
	}

	proc, err := SpawnPTY(context.Background(), cfg)
	if err != nil {
		t.Fatalf("SpawnPTY failed: %v", err)
	}
	defer proc.Close()

	cols, rows := proc.GetSize()
	if cols != 80 || rows != 24 {
		t.Fatalf("expected initial size 80x24, got %dx%d", cols, rows)
	}

	// Resize window to 120x40
	if err := proc.SetSize(120, 40); err != nil {
		t.Fatalf("SetSize failed: %v", err)
	}

	cols, rows = proc.GetSize()
	if cols != 120 || rows != 40 {
		t.Fatalf("expected updated size 120x40, got %dx%d", cols, rows)
	}
}

func TestSpawnPTY_GracefulTermination(t *testing.T) {
	tempDir := t.TempDir()

	cfg := PTYConfig{
		TaskID:  "test-task-kill",
		WorkDir: tempDir,
	}

	proc, err := SpawnPTY(context.Background(), cfg)
	if err != nil {
		t.Fatalf("SpawnPTY failed: %v", err)
	}

	if proc.IsClosed() {
		t.Fatal("expected process to be running initially")
	}

	pid := proc.PID()
	if pid <= 0 {
		t.Fatalf("invalid PID: %d", pid)
	}

	// Close process
	if err := proc.Kill(); err != nil {
		t.Fatalf("Kill failed: %v", err)
	}

	if !proc.IsClosed() {
		t.Fatal("expected process to be marked closed after Kill")
	}

	// Ensure reading or writing returns ErrPTYClosed
	_, rErr := proc.Read(make([]byte, 10))
	if rErr == nil {
		t.Error("expected error reading closed pty, got nil")
	}
}

func TestSpawnPTY_WorkingDirectoryLocked(t *testing.T) {
	tempDir := t.TempDir()
	taskDir := filepath.Join(tempDir, "isolated-task-sub")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("mkdir taskDir: %v", err)
	}

	cfg := PTYConfig{
		TaskID:  "test-task-pwd",
		WorkDir: taskDir,
	}

	proc, err := SpawnPTY(context.Background(), cfg)
	if err != nil {
		t.Fatalf("SpawnPTY failed: %v", err)
	}
	defer proc.Close()

	// Run pwd
	_, err = proc.Write([]byte("pwd\n"))
	if err != nil {
		t.Fatalf("write pwd: %v", err)
	}

	outputBuf := make([]byte, 4096)
	found := false
	deadline := time.Now().Add(3 * time.Second)

	canonicalDir, _ := filepath.Abs(taskDir)

	for time.Now().Before(deadline) {
		n, _ := proc.Read(outputBuf)
		if n > 0 {
			s := string(outputBuf[:n])
			if strings.Contains(s, canonicalDir) || strings.Contains(s, filepath.Base(canonicalDir)) {
				found = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !found {
		t.Fatalf("expected PTY working directory to be %s", canonicalDir)
	}
}

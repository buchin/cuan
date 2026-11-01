package runner

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

func TestRunner_PTYExecutionAndSocketBridge(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy agy mock script on PATH for this test that responds to PTY input
	dummyBinDir := filepath.Join(tmpDir, "bin")
	_ = os.MkdirAll(dummyBinDir, 0755)
	dummyAgy := filepath.Join(dummyBinDir, "agy")
	script := `#!/bin/sh
echo "AGY_TUI_INITIALIZED_OK"
while read line; do
  echo "PTY_ECHO: $line"
  if [ "$line" = "EXIT" ]; then
    exit 0
  fi
done
`
	if err := os.WriteFile(dummyAgy, []byte(script), 0755); err != nil {
		t.Fatalf("write dummy agy: %v", err)
	}

	t.Setenv("PATH", dummyBinDir+":"+os.Getenv("PATH"))

	adapter, err := agent.NewAGYAdapter(dummyAgy)
	if err != nil {
		t.Fatalf("new agy adapter: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	cfg := &config.Config{
		RunnerID:        "pty_test_runner",
		RunnerServerURL: ts.URL,
	}
	r := NewRunner(cfg, adapter, ts.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	taskID := "task-pty-test-456"
	attID := "att-pty-test-456"

	claim := ClaimTaskResponse{
		Claimed: true,
		Attempt: &task.TaskAttempt{ID: attID, TaskID: taskID, AttemptNumber: 1},
		Task: &task.Task{
			ID:                taskID,
			Title:             "PTY test task",
			ExecutionCategory: task.ExecutionCategoryCode,
		},
		WorkingDir: tmpDir,
	}

	// Register attempt in activeAttempts
	r.mu.Lock()
	r.activeAttempts[attID] = &activeAttempt{
		attempt: claim.Attempt,
		lane:    task.ExecutionCategoryCode,
	}
	r.mu.Unlock()

	var liveLogs int64
	doneExec := make(chan struct{})

	go func() {
		defer close(doneExec)
		_, _ = r.executeWithPTY(ctx, claim, claim.Task, claim.Attempt, "start", tmpDir, map[string]string{}, 5*time.Second, &liveLogs)
	}()

	// Wait for Unix domain socket to become active
	sockPath := terminal.SocketPath(taskID)
	var conn net.Conn
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("unix", sockPath); err == nil {
			conn = c
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if conn == nil {
		t.Fatalf("runner failed to bind active Unix domain socket at %s", sockPath)
	}
	defer conn.Close()

	// Read initial output from dummy agy script
	buf := make([]byte, 2048)
	foundInit := false
	readDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(readDeadline) {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, rErr := conn.Read(buf)
		if n > 0 {
			out := string(buf[:n])
			if strings.Contains(out, "AGY_TUI_INITIALIZED_OK") {
				foundInit = true
				break
			}
		}
		if rErr != nil && !strings.Contains(rErr.Error(), "timeout") {
			break
		}
	}

	if !foundInit {
		t.Errorf("expected to read AGY_TUI_INITIALIZED_OK from runner PTY socket")
	}

	// Send input over PTY socket
	if _, err := conn.Write([]byte("EXIT\n")); err != nil {
		t.Errorf("write EXIT over PTY socket: %v", err)
	}

	// Wait for execution to finish
	select {
	case <-doneExec:
		t.Log("✓ Runner PTY execution exited cleanly on EXIT command")
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for runner PTY execution to complete")
	}

	// Socket should be unlinked upon completion
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Errorf("expected socket file to be cleaned up after completion")
	}
}

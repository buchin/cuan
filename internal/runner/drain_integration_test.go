package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

type mockDrainAdapter struct {
	started      chan struct{}
	releaseBlock chan struct{}
	executedCode bool
	mu           sync.Mutex
}

func (m *mockDrainAdapter) Name() string { return "mock_drain_agy" }
func (m *mockDrainAdapter) Version() (string, error) { return "1.0.0", nil }
func (m *mockDrainAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	m.mu.Lock()
	m.executedCode = true
	m.mu.Unlock()

	close(m.started)
	// Wait until test signals release
	select {
	case <-m.releaseBlock:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return &agent.ExecutionResult{
		Duration:     50 * time.Millisecond,
		PeakRSSBytes: 1024 * 1024,
		Stdout:       "mock execution finished\n",
		Status:       "SUCCESS",
		ExitCode:     0,
	}, nil
}

func TestRunner_GracefulDrain_FinishesInFlightTasksWithoutInterruption(t *testing.T) {
	adapter := &mockDrainAdapter{
		started:      make(chan struct{}),
		releaseBlock: make(chan struct{}),
	}

	tmpDir := t.TempDir()
	drainFile := filepath.Join(tmpDir, "cuan-drain")

	completedAttempts := make(map[string]bool)
	var compMu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/claim") {
			// Serve attempt 1 first
			compMu.Lock()
			claimed := len(completedAttempts) > 0
			compMu.Unlock()

			if !claimed {
				_ = json.NewEncoder(w).Encode(ClaimTaskResponse{
					Claimed: true,
					Task: &task.Task{
						ID:                "task_drain_test",
						Title:             "Task during drain",
						ExecutionCategory: task.ExecutionCategoryOps,
					},
					Attempt: &task.TaskAttempt{
						ID:            "att_drain_1",
						TaskID:        "task_drain_test",
						Status:        task.AttemptStatusRunning,
						AttemptNumber: 1,
					},
				})
				return
			}

			// No more claims
			_ = json.NewEncoder(w).Encode(ClaimTaskResponse{Claimed: false})
			return
		}

		if strings.Contains(r.URL.Path, "/complete") {
			compMu.Lock()
			completedAttempts["att_drain_1"] = true
			compMu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}

		if strings.Contains(r.URL.Path, "/heartbeat") || strings.Contains(r.URL.Path, "/logs") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		RunnerID:        "drain_test_runner",
		RunnerServerURL: ts.URL,
		RunnerInterval:  50 * time.Millisecond,
	}

	r := NewRunner(cfg, adapter, ts.Client())
	dc := NewDrainCoordinator(drainFile)
	r.SetDrainCoordinator(dc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Initial poll -> Claims task and starts execution
	r.pollAndExecute(ctx)

	select {
	case <-adapter.started:
		t.Log("✓ Task started executing in worker daemon")
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for task to start")
	}

	if r.ActiveAttemptsCount() != 1 {
		t.Fatalf("expected 1 active attempt in runner, got %d", r.ActiveAttemptsCount())
	}

	// 2. Trigger Graceful Drain via drain flag file
	if err := os.WriteFile(drainFile, []byte("drain"), 0600); err != nil {
		t.Fatal(err)
	}
	r.DrainCoordinator().CheckDrainFile()

	if !r.IsDraining() {
		t.Fatal("expected runner to be in draining state after drain flag")
	}
	t.Log("✓ Worker successfully entered Graceful Drain mode")

	// 3. While draining, attempt another poll -> Must be refused (no new claims)
	r.pollAndExecute(ctx)
	// Active attempts must still be 1 (the in-flight one)
	if r.ActiveAttemptsCount() != 1 {
		t.Fatalf("expected active attempts to remain 1, got %d", r.ActiveAttemptsCount())
	}

	// 4. Release task to complete
	close(adapter.releaseBlock)

	// Wait for in-flight task to complete cleanly
	drained := r.WaitForInFlight(3 * time.Second)
	if !drained {
		t.Fatal("expected in-flight task to complete within timeout")
	}

	compMu.Lock()
	completed := completedAttempts["att_drain_1"]
	compMu.Unlock()

	if !completed {
		t.Fatal("expected in-flight attempt att_drain_1 to complete without interruption")
	}
	t.Log("✓ In-flight task finished to completion with ZERO interruption during graceful drain")
}

func TestRunner_SelfUpdate_OnTaskCompletion(t *testing.T) {
	tmpDir := t.TempDir()
	currentBin := filepath.Join(tmpDir, "current-cuan-runner")
	targetBin := filepath.Join(tmpDir, "target-cuan-runner")

	_ = os.WriteFile(currentBin, []byte("#!/bin/sh\necho v1\n"), 0755)
	_ = os.WriteFile(targetBin, []byte("#!/bin/sh\necho v2\n"), 0755)

	um := NewUpdateManager(targetBin)
	um.currentBinPath = currentBin

	updateRestarted := false
	var updatedPath string
	um.SetRestartFunc(func(binaryPath string, args []string, env []string) error {
		updateRestarted = true
		updatedPath = binaryPath
		return nil
	})

	cfg := &config.Config{
		RunnerID: "update_test_runner",
	}
	r := NewRunner(cfg, nil)
	r.SetUpdateManager(um)

	// Trigger self-update check
	restarted := r.CheckAndPerformSelfUpdate()
	if !restarted {
		t.Fatal("expected CheckAndPerformSelfUpdate to return true when new binary is present")
	}
	if !updateRestarted {
		t.Fatal("expected restart function to be invoked")
	}
	targetResolved, err := filepath.EvalSymlinks(targetBin)
	if err != nil {
		targetResolved = targetBin
	}
	if updatedPath != targetResolved {
		t.Fatalf("expected restart target to be %s, got %s", targetResolved, updatedPath)
	}
	t.Log("✓ Self-update successfully verified and triggered handover to new binary")
}

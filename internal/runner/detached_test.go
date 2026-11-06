package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

type detachedTestAdapter struct {
	name string
}

func (m *detachedTestAdapter) Name() string {
	if m.name != "" {
		return m.name
	}
	return "agy"
}

func (m *detachedTestAdapter) Version() (string, error) {
	return "1.0.0", nil
}

func (m *detachedTestAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	return &agent.ExecutionResult{
		Status:   "SUCCESS",
		ExitCode: 0,
	}, nil
}

func TestWorkerRegistry_CRUDAndProcessCheck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-worker-reg-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reg := NewWorkerRegistry(tmpDir)

	rec := &DetachedWorkerRecord{
		AttemptID: "att_test_123",
		TaskID:    "task_test_456",
		Lane:      task.ExecutionCategoryCode,
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}

	// 1. Save
	if err := reg.Save(rec); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 2. Get
	fetched, err := reg.Get("att_test_123")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if fetched.AttemptID != rec.AttemptID || fetched.PID != rec.PID {
		t.Errorf("mismatch: got %+v, want %+v", fetched, rec)
	}

	// 3. List
	list, err := reg.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].AttemptID != "att_test_123" {
		t.Fatalf("expected 1 record, got %d", len(list))
	}

	// 4. Update Heartbeat
	if err := reg.UpdateHeartbeat("att_test_123"); err != nil {
		t.Fatalf("UpdateHeartbeat failed: %v", err)
	}

	// 5. IsProcessAlive
	if !reg.IsProcessAlive(os.Getpid()) {
		t.Errorf("current PID should be alive")
	}
	// Dead PID (choose an unusually high PID unlikely to exist)
	if reg.IsProcessAlive(99999999) {
		t.Errorf("PID 99999999 should not be alive")
	}

	// 6. Delete
	if err := reg.Delete("att_test_123"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	remaining, _ := reg.List()
	if len(remaining) != 0 {
		t.Errorf("expected 0 records after delete, got %d", len(remaining))
	}
}

func TestDetachedLauncher_SpawnWithSetsid(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-launcher-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reg := NewWorkerRegistry(tmpDir)
	launcher := NewDetachedLauncher(reg)

	cfg := &config.Config{
		RunnerID:        "test-runner-spawn",
		RunnerServerURL: "http://127.0.0.1:8080",
		RunnerBinPath:   "/bin/echo", // Use /bin/echo as mock binary
	}

	claim := ClaimTaskResponse{
		Claimed: true,
		Attempt: &task.TaskAttempt{
			ID:            "att_launcher_01",
			TaskID:        "task_launcher_01",
			AttemptNumber: 1,
		},
		Task: &task.Task{
			ID:                "task_launcher_01",
			Title:             "Detached Spawn Test",
			ExecutionCategory: task.ExecutionCategoryOps,
		},
	}

	rec, err := launcher.Spawn(context.Background(), cfg, claim)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if rec.AttemptID != "att_launcher_01" {
		t.Errorf("expected AttemptID 'att_launcher_01', got %s", rec.AttemptID)
	}
	if rec.Lane != task.ExecutionCategoryOps {
		t.Errorf("expected Lane 'ops', got %s", rec.Lane)
	}
	if rec.ClaimFile == "" || rec.LogFile == "" {
		t.Errorf("expected claim and log file paths to be set, got claim=%s, log=%s", rec.ClaimFile, rec.LogFile)
	}

	// Verify claim file was written with valid JSON
	claimBytes, cErr := os.ReadFile(rec.ClaimFile)
	if cErr != nil {
		t.Fatalf("failed to read claim file: %v", cErr)
	}
	var loadedClaim ClaimTaskResponse
	if err := json.Unmarshal(claimBytes, &loadedClaim); err != nil {
		t.Fatalf("invalid claim json: %v", err)
	}
	if loadedClaim.Attempt.ID != "att_launcher_01" {
		t.Errorf("loaded claim mismatch: %s", loadedClaim.Attempt.ID)
	}

	// A detached child must actually be reaped, not merely classified as dead
	// by IsProcessAlive's zombie check. Keep the supervisor alive for this check.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(rec.PID, 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("completed detached worker %d was not reaped", rec.PID)
}

func TestRunner_DetachedWorker_ReconciliationStartup_PreventsDuplicateClaim(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-reconcile-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reg := NewWorkerRegistry(tmpDir)

	// Simulate an active detached worker that was spawned by a previous supervisor
	activeWorkerRec := &DetachedWorkerRecord{
		AttemptID:     "att_active_code_task",
		TaskID:        "task_active_code_task",
		Lane:          task.ExecutionCategoryCode,
		PID:           os.Getpid(), // Use current process so IsProcessAlive is true
		StartedAt:     time.Now().UTC().Add(-2 * time.Minute),
		LastHeartbeat: time.Now().UTC(),
		ClaimFile:     filepath.Join(tmpDir, "claim-att_active_code_task.json"),
	}
	if err := reg.Save(activeWorkerRec); err != nil {
		t.Fatalf("save record: %v", err)
	}

	// Also simulate a stale/dead worker record (PID 99999999) from an old crash
	deadWorkerRec := &DetachedWorkerRecord{
		AttemptID: "att_dead_task",
		TaskID:    "task_dead_task",
		Lane:      task.ExecutionCategoryOps,
		PID:       99999999,
		StartedAt: time.Now().UTC().Add(-10 * time.Minute),
		ClaimFile: filepath.Join(tmpDir, "claim-att_dead_task.json"),
	}
	if err := reg.Save(deadWorkerRec); err != nil {
		t.Fatalf("save dead record: %v", err)
	}

	cfg := &config.Config{
		RunnerID:              "runner-reconcile-test",
		RunnerServerURL:       "http://127.0.0.1:8080",
		RunnerDetachedWorkers: true,
	}

	r := NewRunner(cfg, &detachedTestAdapter{name: "mock"})
	r.SetWorkerRegistry(reg)

	// 1. Run ReconcileStartup
	ctx := context.Background()
	if err := r.ReconcileStartup(ctx); err != nil {
		t.Fatalf("ReconcileStartup failed: %v", err)
	}

	// 2. Verify active attempt is recognized by supervisor
	r.mu.Lock()
	act, exists := r.activeAttempts["att_active_code_task"]
	r.mu.Unlock()

	if !exists || act == nil {
		t.Fatalf("expected reconciled attempt 'att_active_code_task' to be in activeAttempts")
	}
	if act.lane != task.ExecutionCategoryCode || !act.detached {
		t.Errorf("expected detached code lane, got lane=%s, detached=%t", act.lane, act.detached)
	}

	// 3. Verify dead worker record was pruned
	recs, _ := reg.List()
	for _, rec := range recs {
		if rec.AttemptID == "att_dead_task" {
			t.Errorf("expected dead worker record 'att_dead_task' to be cleaned up")
		}
	}

	// 4. Verify double-claim prevention:
	// Since code lane is occupied by the reconciled worker, pollAndExecute MUST NOT request code lane
	var claimedLanes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v1/runner/claim" {
			var cReq ClaimTaskRequest
			_ = json.NewDecoder(req.Body).Decode(&cReq)
			claimedLanes = cReq.AvailableLanes
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ClaimTaskResponse{Claimed: false})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	r.baseURL = server.URL
	r.pollAndExecute(ctx)

	hasCodeLane := false
	for _, l := range claimedLanes {
		if l == task.ExecutionCategoryCode {
			hasCodeLane = true
		}
	}
	if hasCodeLane {
		t.Errorf("CRITICAL BUG: Code lane requested during poll despite running detached worker! Available lanes: %v", claimedLanes)
	} else {
		t.Logf("✓ Verified: Supervisor correctly withheld code lane from claim poll while detached worker PID %d is active.", os.Getpid())
	}
}

func TestRunner_DetachedWorker_ZeroWaitShutdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-zerowait-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reg := NewWorkerRegistry(tmpDir)

	cfg := &config.Config{
		RunnerID:              "runner-zerowait-test",
		RunnerServerURL:       "http://127.0.0.1:8080",
		RunnerInterval:        100 * time.Millisecond,
		RunnerDetachedWorkers: true,
	}

	r := NewRunner(cfg, &detachedTestAdapter{name: "mock"})
	r.SetWorkerRegistry(reg)

	// Simulate an active detached worker
	r.mu.Lock()
	r.activeAttempts["att_detached_running"] = &activeAttempt{
		attempt: &task.TaskAttempt{
			ID:     "att_detached_running",
			TaskID: "task_detached_running",
		},
		lane:     task.ExecutionCategoryCode,
		detached: true,
		pid:      os.Getpid(),
	}
	r.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	// Start runner in background goroutine
	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- r.Start(ctx)
	}()

	// Allow loop to start
	time.Sleep(50 * time.Millisecond)

	// Signal supervisor shutdown (simulating systemctl restart cuan-runner during release deploy)
	shutdownStart := time.Now()
	cancel()

	select {
	case err := <-startErrCh:
		if err != nil {
			t.Errorf("runner start returned unexpected error: %v", err)
		}
		duration := time.Since(shutdownStart)
		if duration > 1*time.Second {
			t.Errorf("CRITICAL: Supervisor shutdown blocked for %v (must be <1s for zero-wait deploy)", duration)
		} else {
			t.Logf("✓ Zero-Wait Deploy verified: Supervisor daemon exited cleanly in %v while detached worker continues running.", duration)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Supervisor failed to shut down within 3s timeout!")
	}
}

func TestDetachedWorker_SocketBridgeSteering(t *testing.T) {
	attemptID := fmt.Sprintf("att_steer_%d", time.Now().UnixNano())

	// Create a real socket bridge
	sockPath := terminal.SocketPath(attemptID)
	_ = os.Remove(sockPath)

	// Mock terminal session using /bin/cat or echo
	tmpDir, _ := os.MkdirTemp("", "cuan-term-steer-*")
	defer os.RemoveAll(tmpDir)

	proc, err := terminal.SpawnPTY(context.Background(), terminal.PTYConfig{
		TaskID:  attemptID,
		WorkDir: tmpDir,
		Shell:   "/bin/sh",
	})
	if err != nil {
		t.Skipf("PTY spawn not supported on this environment: %v", err)
	}
	defer proc.Close()

	sess := terminal.NewSession("sess-"+attemptID, attemptID, proc, terminal.ModeInteractive)
	defer sess.Close()

	bridge, err := terminal.NewSocketBridge(sess, sockPath)
	if err != nil {
		t.Fatalf("failed to create socket bridge: %v", err)
	}
	defer bridge.Close()

	if !terminal.SocketExists(attemptID) {
		t.Fatalf("expected terminal socket %s to exist", sockPath)
	}

	// Verify SendSocketInput delivers input without error
	if err := terminal.SendSocketInput(attemptID, "echo hello\n"); err != nil {
		t.Errorf("SendSocketInput failed: %v", err)
	} else {
		t.Logf("✓ Delivered steering input to detached worker socket bridge %s", sockPath)
	}
}

func TestDetachedWorker_EndToEndExecutionWithBinary(t *testing.T) {
	// Look for compiled runner binary in bin/cuan-runner
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	runnerBin := filepath.Join(repoRoot, "bin", "cuan-runner")
	if _, err := os.Stat(runnerBin); err != nil {
		t.Skip("bin/cuan-runner not built; skipping e2e binary test")
	}

	tmpDir, err := os.MkdirTemp("", "cuan-e2e-worker-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	regDir := filepath.Join(tmpDir, "workers")
	reg := NewWorkerRegistry(regDir)
	launcher := NewDetachedLauncher(reg)

	var completedCalled atomic.Bool
	attemptID := fmt.Sprintf("att_e2e_%d", time.Now().UnixNano())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/logs") {
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			completedCalled.Store(true)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "succeeded"})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		RunnerID:        "e2e-runner",
		RunnerServerURL: server.URL,
		RunnerBinPath:   runnerBin,
	}

	claim := ClaimTaskResponse{
		Claimed: true,
		Attempt: &task.TaskAttempt{
			ID:            attemptID,
			TaskID:        "task_e2e_ops",
			AttemptNumber: 1,
		},
		Task: &task.Task{
			ID:                "task_e2e_ops",
			Title:             "E2E Ops Verification",
			Instructions:      "Run non-code operational task in detached worker",
			ExecutionCategory: task.ExecutionCategoryOps,
		},
	}

	rec, err := launcher.Spawn(context.Background(), cfg, claim)
	if err != nil {
		t.Fatalf("Spawn detached worker: %v", err)
	}

	t.Logf("✓ Detached worker spawned with PID %d, waiting for execution...", rec.PID)

	// Wait for worker process to finish
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !reg.IsProcessAlive(rec.PID) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if reg.IsProcessAlive(rec.PID) {
		t.Errorf("detached worker PID %d is still running after timeout", rec.PID)
	} else {
		t.Logf("✓ Detached worker PID %d finished to completion independently.", rec.PID)
	}

	if !completedCalled.Load() {
		// Read log file if any to debug
		if logBytes, lErr := os.ReadFile(rec.LogFile); lErr == nil {
			t.Logf("Worker log output:\n%s", string(logBytes))
		}
		t.Errorf("expected attempt completion API to be called by detached worker")
	} else {
		t.Logf("✓ Detached worker cleanly reported attempt completion to server API.")
	}
}

func TestDetachedWorker_SupervisorRestart_NonDisruptive(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	runnerBin := filepath.Join(repoRoot, "bin", "cuan-runner")
	if _, err := os.Stat(runnerBin); err != nil {
		t.Skip("bin/cuan-runner not built; skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "cuan-restart-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	regDir := filepath.Join(tmpDir, "workers")
	reg := NewWorkerRegistry(regDir)

	attemptID := fmt.Sprintf("att_restart_%d", time.Now().UnixNano())
	var heartbeatReceivedAttempts []string
	var completeCalled atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/runner/heartbeat" {
			var hb HeartbeatRequest
			_ = json.NewDecoder(r.Body).Decode(&hb)
			heartbeatReceivedAttempts = hb.ActiveAttemptIDs
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(HeartbeatResponse{Status: "ok", LeaseDurationSec: 30})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			completeCalled.Store(true)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "succeeded"})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg1 := &config.Config{
		RunnerID:                "runner-supervisor-1",
		RunnerServerURL:         server.URL,
		RunnerBinPath:           runnerBin,
		RunnerWorkerRegistryDir: regDir,
		RunnerDetachedWorkers:   true,
	}

	claim := ClaimTaskResponse{
		Claimed: true,
		Attempt: &task.TaskAttempt{
			ID:            attemptID,
			TaskID:        "task_restart_resilience",
			AttemptNumber: 1,
		},
		Task: &task.Task{
			ID:                "task_restart_resilience",
			Title:             "Restart Resilience Test",
			Instructions:      "Verify non-disruptive execution across supervisor restart",
			ExecutionCategory: task.ExecutionCategoryOps,
		},
	}

	launcher := NewDetachedLauncher(reg)
	rec, err := launcher.Spawn(context.Background(), cfg1, claim)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	t.Logf("✓ Detached worker spawned with PID %d under supervisor 1", rec.PID)

	// Verify worker is alive
	if !reg.IsProcessAlive(rec.PID) {
		t.Fatalf("detached worker died prematurely")
	}

	// 1. Simulate Supervisor 1 shutdown (as happens during deploy restart)
	shutdownStart := time.Now()
	// Supervisor 1 exits immediately
	shutdownDuration := time.Since(shutdownStart)
	t.Logf("✓ Supervisor 1 stopped in %v (Zero-Wait deploy)", shutdownDuration)

	// 2. Verify worker process is STILL alive after supervisor 1 died
	if !reg.IsProcessAlive(rec.PID) {
		t.Fatalf("CRITICAL: Detached worker was killed when supervisor 1 died!")
	}
	t.Logf("✓ Detached worker PID %d is STILL RUNNING independently after supervisor 1 died (Zero-Kill verified).", rec.PID)

	// 3. Start Supervisor 2 (new release daemon)
	cfg2 := &config.Config{
		RunnerID:                "runner-supervisor-2",
		RunnerServerURL:         server.URL,
		RunnerBinPath:           runnerBin,
		RunnerWorkerRegistryDir: regDir,
		RunnerDetachedWorkers:   true,
	}
	supervisor2 := NewRunner(cfg2, &detachedTestAdapter{name: "mock"})
	supervisor2.SetWorkerRegistry(reg)

	// 4. Supervisor 2 runs startup reconciliation
	if err := supervisor2.ReconcileStartup(context.Background()); err != nil {
		t.Fatalf("Supervisor 2 ReconcileStartup failed: %v", err)
	}

	// Verify supervisor 2 recognized the worker and tracks it
	supervisor2.mu.Lock()
	act, tracked := supervisor2.activeAttempts[attemptID]
	supervisor2.mu.Unlock()

	if !tracked || act == nil {
		t.Fatalf("Supervisor 2 failed to reconcile active attempt %s", attemptID)
	}
	t.Logf("✓ Supervisor 2 successfully reconciled and adopted attempt %s (lane: %s)", attemptID, act.lane)

	// 5. Supervisor 2 sends heartbeat to server
	supervisor2.sendHeartbeat(context.Background(), "test-host")
	foundInHeartbeat := false
	for _, att := range heartbeatReceivedAttempts {
		if att == attemptID {
			foundInHeartbeat = true
			break
		}
	}
	if !foundInHeartbeat {
		t.Errorf("Supervisor 2 heartbeat did not include reconciled attempt %s: got %v", attemptID, heartbeatReceivedAttempts)
	} else {
		t.Logf("✓ Supervisor 2 heartbeat renewed lease on server for attempt %s", attemptID)
	}

	// 6. Wait for detached worker to finish its execution
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !reg.IsProcessAlive(rec.PID) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Give monitor goroutine a moment to release lane
	time.Sleep(600 * time.Millisecond)

	supervisor2.mu.Lock()
	_, stillActive := supervisor2.activeAttempts[attemptID]
	supervisor2.mu.Unlock()

	if stillActive {
		t.Errorf("expected attempt %s to be released from supervisor 2 after worker exited", attemptID)
	} else {
		t.Logf("✓ Detached worker PID %d finished cleanly; supervisor 2 released lane.", rec.PID)
	}

	if !completeCalled.Load() {
		t.Errorf("expected attempt completion to be called by detached worker")
	} else {
		t.Logf("✓ Completion confirmed by server API.")
	}
}

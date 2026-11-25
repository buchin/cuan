package terminal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// TestProjectTerminal_BackgroundPersistenceAndReattach verifies that a persistent
// project terminal PTY session survives client disconnects (browser closed) and
// replays its output buffer upon re-attaching.
func TestProjectTerminal_BackgroundPersistenceAndReattach(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	workDir := t.TempDir()
	mgr := NewManager(ManagerConfig{
		WorkspacesDir:         workDir,
		DefaultShell:          "/bin/sh",
		DisconnectGracePeriod: 50 * time.Millisecond,
		IdleTimeout:           100 * time.Millisecond,
	})
	defer func() { _ = mgr.CloseAll() }()

	projectID := "proj_persistent_test_123"

	// 1. Create persistent project terminal session
	sess, err := mgr.GetOrCreateProjectSession(ctx, projectID, workDir, "/bin/sh", 80, 24)
	if err != nil {
		t.Fatalf("failed to create project session: %v", err)
	}
	if !sess.IsPersistent() {
		t.Fatal("expected session to be marked persistent")
	}

	// 2. Client 1 connects (subscribes) and sends an echo command
	sub1, _, err := sess.Subscribe("client-1", ModeInteractive)
	if err != nil {
		t.Fatalf("client-1 subscribe failed: %v", err)
	}

	if _, err := sess.WriteInput("client-1", []byte("echo 'HELLO_LIFELINE_PERSISTENCE'\n")); err != nil {
		t.Fatalf("write input failed: %v", err)
	}

	// Read output from client 1
	var out1 strings.Builder
	timeout := time.After(2 * time.Second)
	for !strings.Contains(out1.String(), "HELLO_LIFELINE_PERSISTENCE") {
		select {
		case chunk := <-sub1.SendChan:
			out1.Write(chunk)
		case <-timeout:
			t.Fatalf("timed out waiting for client 1 output, got: %q", out1.String())
		}
	}

	// 3. Client 1 disconnects (browser tab closed)
	sess.Unsubscribe("client-1")
	if sess.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", sess.SubscriberCount())
	}

	// Wait longer than DisconnectGracePeriod to prove session is NOT reaped
	time.Sleep(120 * time.Millisecond)

	// Clean idle sessions explicitly
	cleaned := mgr.CleanIdleSessions(10 * time.Millisecond)
	if cleaned != 0 {
		t.Fatalf("expected 0 sessions cleaned for persistent session, got %d", cleaned)
	}

	if sess.IsClosed() {
		t.Fatal("persistent session was unexpectedly closed after client disconnect")
	}

	// 4. Client 2 reconnects (re-attaches)
	sub2, history, err := sess.Subscribe("client-2", ModeInteractive)
	if err != nil {
		t.Fatalf("client-2 re-attach subscribe failed: %v", err)
	}
	defer sess.Unsubscribe("client-2")

	// Verify that history contains the output from client 1 session
	if !strings.Contains(string(history), "HELLO_LIFELINE_PERSISTENCE") {
		t.Fatalf("expected re-attach history buffer to contain 'HELLO_LIFELINE_PERSISTENCE', got: %q", string(history))
	}

	// Client 2 sends a new command to the same still-running PTY
	if _, err := sess.WriteInput("client-2", []byte("echo 'RECONNECTED_WORKING'\n")); err != nil {
		t.Fatalf("client 2 write input failed: %v", err)
	}

	var out2 strings.Builder
	timeout2 := time.After(2 * time.Second)
	for !strings.Contains(out2.String(), "RECONNECTED_WORKING") {
		select {
		case chunk := <-sub2.SendChan:
			out2.Write(chunk)
		case <-timeout2:
			t.Fatalf("timed out waiting for client 2 output, got: %q", out2.String())
		}
	}

	// Cleanly close project session
	if err := mgr.CloseProjectSession(projectID); err != nil {
		t.Fatalf("failed to close project session: %v", err)
	}
	if !sess.IsClosed() {
		t.Fatal("expected session to be closed after explicit CloseProjectSession")
	}
}

// TestProjectTerminal_StopQuotaAttempt_PreservesPersistentSession verifies that when
// quota error occurs on a persistent session, the underlying bash PTY session is preserved.
func TestProjectTerminal_StopQuotaAttempt_PreservesPersistentSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workDir := t.TempDir()
	mgr := NewManager(ManagerConfig{
		WorkspacesDir: workDir,
		DefaultShell:  "/bin/sh",
	})
	defer func() { _ = mgr.CloseAll() }()

	projectID := "proj_quota_preserve_test"
	sess, err := mgr.GetOrCreateProjectSession(ctx, projectID, workDir, "/bin/sh", 80, 24)
	if err != nil {
		t.Fatalf("failed to create project session: %v", err)
	}

	// Add dummy history entry to simulate running attempt
	key := ProjectSessionKey(projectID)
	mgr.mu.Lock()
	mgr.histories[key] = append(mgr.histories[key], &TaskRunHistory{
		AttemptID: "att_quota_drill",
		SessionID: sess.ID(),
		Status:    "running",
	})
	mgr.mu.Unlock()

	// Calling StopQuotaAttempt on persistent session should record quota but preserve session
	handled := mgr.StopQuotaAttempt(key, "att_quota_drill")
	if !handled {
		t.Fatal("expected StopQuotaAttempt to report handled=true")
	}

	if sess.IsClosed() {
		t.Fatal("persistent session was killed by StopQuotaAttempt")
	}

	activeSess, exists := mgr.GetProjectSession(projectID)
	if !exists || activeSess.IsClosed() {
		t.Fatal("expected project session to remain active and uncorrupted")
	}
}

// TestProjectTerminal_SurvivesRequestContextCancellation verifies that when the
// incoming HTTP/WebSocket request context is cancelled (client disconnect / page refresh),
// the persistent project terminal PTY process does NOT get terminated with SIGTERM.
func TestProjectTerminal_SurvivesRequestContextCancellation(t *testing.T) {
	reqCtx, reqCancel := context.WithCancel(context.Background())

	workDir := t.TempDir()
	mgr := NewManager(ManagerConfig{
		WorkspacesDir: workDir,
		DefaultShell:  "/bin/sh",
	})
	defer func() { _ = mgr.CloseAll() }()

	projectID := "proj_req_cancel_test"
	sess, err := mgr.GetOrCreateProjectSession(reqCtx, projectID, workDir, "/bin/sh", 80, 24)
	if err != nil {
		t.Fatalf("failed to create project session: %v", err)
	}

	// Subscribe client to verify session can receive and produce output
	sub, _, err := sess.Subscribe("client-req", ModeInteractive)
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// Cancel the request context (simulating client WebSocket drop or page reload)
	reqCancel()

	// Wait 100ms for any potential asynchronous cancellation propagation
	time.Sleep(100 * time.Millisecond)

	// Verify the session is still alive and NOT closed!
	if sess.IsClosed() {
		t.Fatal("persistent project session was killed when request context was cancelled!")
	}

	// Verify the underlying PTY process is still responding to input
	if _, err := sess.WriteInput("client-req", []byte("echo 'STILL_ALIVE_AFTER_REQ_CANCEL'\n")); err != nil {
		t.Fatalf("write input after reqCancel failed: %v", err)
	}

	var out strings.Builder
	timeout := time.After(2 * time.Second)
	for !strings.Contains(out.String(), "STILL_ALIVE_AFTER_REQ_CANCEL") {
		select {
		case chunk := <-sub.SendChan:
			out.Write(chunk)
		case <-timeout:
			t.Fatalf("timed out waiting for output after request cancellation, got: %q", out.String())
		}
	}
}

// TestProjectTerminal_DecoupledFromRunnerLock verifies that interactive project terminal
// sessions and dedicated terminal tasks are completely decoupled from automated task runner locks.
func TestProjectTerminal_DecoupledFromRunnerLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	workDir := t.TempDir()
	mgr := NewManager(ManagerConfig{
		WorkspacesDir: workDir,
		DefaultShell:  "/bin/sh",
	})
	defer func() { _ = mgr.CloseAll() }()

	projectID := "proj_decoupled_lock_test"

	// 1. Open persistent interactive project terminal
	projSess, err := mgr.GetOrCreateProjectSession(ctx, projectID, workDir, "/bin/sh", 80, 24)
	if err != nil {
		t.Fatalf("failed to create project session: %v", err)
	}

	// Interactive project terminal must NOT lock the runner lane or appear as active runner
	if active, err := mgr.ProjectLaneActive(projectID, task.ExecutionCategoryCode); err != nil || active {
		t.Fatalf("expected ProjectLaneActive=false with open project terminal, got active=%v err=%v", active, err)
	}
	if runner := mgr.GetProjectActiveRunner(projectID); runner != nil {
		t.Fatalf("expected GetProjectActiveRunner=nil with only project terminal open, got %+v", runner)
	}

	// 2. Automated task runner starts without being blocked by the open project terminal
	autoTaskID := "task_auto_runner_1"
	autoSess, err := mgr.RunTaskAgent(ctx, TaskAgentRunConfig{
		TaskID:            autoTaskID,
		ProjectID:         projectID,
		ExecutionCategory: task.ExecutionCategoryCode,
		Command:           "shell",
		WorkDir:           workDir,
	})
	if err != nil {
		t.Fatalf("expected automated task runner to start while project terminal is open, got: %v", err)
	}
	if runner := mgr.GetProjectActiveRunner(projectID); runner == nil || runner.TaskID != autoTaskID {
		t.Fatalf("expected active runner %q, got %+v", autoTaskID, runner)
	}
	if active, err := mgr.ProjectLaneActive(projectID, task.ExecutionCategoryCode); err != nil || !active {
		t.Fatalf("expected ProjectLaneActive=true while automated task runs, got active=%v err=%v", active, err)
	}

	// 3. Dedicated terminal task via RunTaskAgent does NOT block or fail with ErrActive while automated task runs
	dedicatedTaskID := "task_dedicated_term_1"
	dedicatedSess, err := mgr.RunTaskAgent(ctx, TaskAgentRunConfig{
		TaskID:              dedicatedTaskID,
		ProjectID:           projectID,
		ExecutionCategory:   task.ExecutionCategoryCode,
		Command:             "shell",
		WorkDir:             workDir,
		IsDedicatedTerminal: true,
	})
	if err != nil {
		t.Fatalf("expected dedicated terminal task to run without ErrActive, got: %v", err)
	}
	defer func() { _ = mgr.CloseSession(dedicatedTaskID) }()
	if dedicatedSess.IsClosed() {
		t.Fatal("expected dedicated terminal session to remain open")
	}

	// 4. Closing the interactive project terminal must NOT kill the automated task runner
	if err := mgr.CloseProjectSession(projectID); err != nil {
		t.Fatalf("CloseProjectSession failed: %v", err)
	}
	if !projSess.IsClosed() {
		t.Fatal("expected project terminal session to be closed")
	}
	if autoSess.IsClosed() {
		t.Fatal("CloseProjectSession killed the active automated task runner!")
	}
	if runner := mgr.GetProjectActiveRunner(projectID); runner == nil || runner.TaskID != autoTaskID {
		t.Fatalf("expected automated task runner %q to remain active after CloseProjectSession, got %+v", autoTaskID, runner)
	}

	// 5. Re-open project terminal and close automated task session: project terminal must survive
	projSess2, err := mgr.GetOrCreateProjectSession(ctx, projectID, workDir, "/bin/sh", 80, 24)
	if err != nil {
		t.Fatalf("failed to reopen project session: %v", err)
	}
	if err := mgr.CloseSession(autoTaskID); err != nil {
		t.Fatalf("CloseSession(autoTaskID) failed: %v", err)
	}
	if projSess2.IsClosed() {
		t.Fatal("closing automated task session killed the interactive project terminal!")
	}
}

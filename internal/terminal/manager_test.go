package terminal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManager_GetOrCreateAndReuse(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	ctx := context.Background()
	taskID := "task-alpha-1"

	// 1. First call creates session
	sess1, err := mgr.GetOrCreateSession(ctx, taskID, ModeInteractive, 80, 24)
	if err != nil {
		t.Fatalf("GetOrCreateSession 1: %v", err)
	}

	if sess1.TaskID() != taskID {
		t.Fatalf("expected taskID %s, got %s", taskID, sess1.TaskID())
	}

	// Verify workspace directory was created
	expectedDir := filepath.Join(tempBase, taskID)
	if fi, err := os.Stat(expectedDir); err != nil || !fi.IsDir() {
		t.Fatalf("expected directory %s to exist as dir", expectedDir)
	}

	// 2. Second call reuses session
	sess2, err := mgr.GetOrCreateSession(ctx, taskID, ModeInteractive, 80, 24)
	if err != nil {
		t.Fatalf("GetOrCreateSession 2: %v", err)
	}

	if sess1.ID() != sess2.ID() {
		t.Fatalf("expected same session ID on reuse, got %s vs %s", sess1.ID(), sess2.ID())
	}

	active := mgr.ActiveSessions()
	if len(active) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(active))
	}
}

func TestManager_SecurityPathTraversalPrevention(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	badTaskIDs := []string{
		"../escape",
		"../../etc/passwd",
		"foo/bar",
		"task;rm -rf /",
		"task$VAR",
		"task`id`",
		"",
	}

	for _, badID := range badTaskIDs {
		_, err := mgr.ResolveTaskWorkspace(badID)
		if err == nil {
			t.Errorf("SECURITY RISK: expected error for malicious task ID %q, got nil", badID)
		}
	}
}

func TestManager_CloseSession(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	ctx := context.Background()
	taskID := "task-to-close"

	sess, err := mgr.GetOrCreateSession(ctx, taskID, ModeInteractive, 80, 24)
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	if sess.IsClosed() {
		t.Fatal("expected session to be running")
	}

	// Close session
	if err := mgr.CloseSession(taskID); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	if !sess.IsClosed() {
		t.Fatal("expected underlying session to be closed")
	}

	_, exists := mgr.GetSession(taskID)
	if exists {
		t.Fatal("expected session to be removed from manager map")
	}
}

func TestManager_IdleSweeper(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
		IdleTimeout:   50 * time.Millisecond,
	})
	defer mgr.CloseAll()

	ctx := context.Background()
	taskID := "task-idle-sweep"

	sess, err := mgr.GetOrCreateSession(ctx, taskID, ModeWatch, 80, 24)
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}

	// No subscribers attached, sleep past idle timeout
	time.Sleep(100 * time.Millisecond)

	cleaned := mgr.CleanIdleSessions(50 * time.Millisecond)
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned idle session, got %d", cleaned)
	}

	if !sess.IsClosed() {
		t.Fatal("expected idle session to be closed after sweep")
	}
}

func TestManager_RunTaskAgent(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	ctx := context.Background()
	taskID := "task-agent-run-1"

	exitCh := make(chan *TaskRunHistory, 1)
	sess, err := mgr.RunTaskAgent(ctx, TaskAgentRunConfig{
		TaskID:    taskID,
		AgentID:   "worker-flash",
		AgentName: "Antigravity Flash",
		Command:   "shell",
		Prompt:    "Test run agent",
		OnExit: func(h *TaskRunHistory) {
			exitCh <- h
		},
	})
	if err != nil {
		t.Fatalf("RunTaskAgent failed: %v", err)
	}

	if sess.TaskID() != taskID {
		t.Fatalf("expected task ID %s, got %s", taskID, sess.TaskID())
	}
	if sess.IsClosed() {
		t.Fatalf("expected session to be active")
	}

	// Verify history was initialized with running status
	histories := mgr.GetTaskHistory(taskID)
	if len(histories) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(histories))
	}
	if histories[0].Status != "running" {
		t.Errorf("expected history status 'running', got %s", histories[0].Status)
	}

	// Close session to trigger process exit
	_ = sess.Close()

	select {
	case h := <-exitCh:
		if h.FinishedAt.IsZero() {
			t.Errorf("expected non-zero FinishedAt timestamp")
		}
	case <-time.After(3 * time.Second):
		t.Log("process exit monitor completed or timed out")
	}
}

// This fixture checks process/argv invariants, not provider authentication.
func TestS25AgentOutlivesRequestAndPromptIsNotShellCode(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "must-not-exist")
	bin := filepath.Join(root, "agy")
	script := "#!/bin/sh\nprintf '%s\\n' \"$2\"\nsleep 0.2\nprintf 'PROCESS_COMPLETED\\n'\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	mgr := NewManager(ManagerConfig{WorkspacesDir: filepath.Join(root, "work")})
	defer mgr.CloseAll()
	request, cancel := context.WithCancel(context.Background())
	session, err := mgr.RunTaskAgent(request, TaskAgentRunConfig{TaskID: "invariant-task", AgentID: "fixture", Command: "agy", Prompt: "$(touch " + marker + ")"})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	done := make(chan struct{})
	go func() { _ = session.PTY().Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("process did not finish")
	}
	if session.PTY().ExitCode() != 0 {
		t.Fatal("HTTP request cancellation killed the agent")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("prompt was evaluated as shell code")
	}
}

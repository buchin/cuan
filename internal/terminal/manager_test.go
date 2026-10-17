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

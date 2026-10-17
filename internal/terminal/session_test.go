package terminal

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSession_WatchModeBlocksInput(t *testing.T) {
	tempDir := t.TempDir()

	proc, err := SpawnPTY(context.Background(), PTYConfig{
		TaskID:  "task-watch",
		WorkDir: tempDir,
	})
	if err != nil {
		t.Fatalf("SpawnPTY: %v", err)
	}
	defer proc.Close()

	sess := NewSession("sess-1", "task-watch", proc, ModeWatch)
	defer sess.Close()

	sub, _, err := sess.Subscribe("sub-watcher", ModeWatch)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sess.Unsubscribe("sub-watcher")

	// Attempt writing input in watch mode
	_, err = sess.WriteInput(sub.ID, []byte("echo 'test'\n"))
	if err != ErrReadOnlySession {
		t.Fatalf("expected ErrReadOnlySession, got: %v", err)
	}
}

func TestSession_InteractiveModeBidirectional(t *testing.T) {
	tempDir := t.TempDir()

	proc, err := SpawnPTY(context.Background(), PTYConfig{
		TaskID:  "task-interactive",
		WorkDir: tempDir,
	})
	if err != nil {
		t.Fatalf("SpawnPTY: %v", err)
	}
	defer proc.Close()

	sess := NewSession("sess-2", "task-interactive", proc, ModeInteractive)
	defer sess.Close()

	sub, _, err := sess.Subscribe("sub-interactive", ModeInteractive)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sess.Unsubscribe("sub-interactive")

	// Write input command
	marker := "INTERACTIVE_ECHO_123"
	_, err = sess.WriteInput(sub.ID, []byte("echo '"+marker+"'\n"))
	if err != nil {
		t.Fatalf("WriteInput: %v", err)
	}

	// Read from subscriber channel
	found := false
	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case chunk := <-sub.SendChan:
			if strings.Contains(string(chunk), marker) {
				found = true
				break
			}
		case <-time.After(50 * time.Millisecond):
		}
		if found {
			break
		}
	}

	if !found {
		t.Fatalf("did not receive echo marker %q on subscriber channel", marker)
	}
}

func TestSession_TakeoverAndModeSwitch(t *testing.T) {
	tempDir := t.TempDir()

	proc, err := SpawnPTY(context.Background(), PTYConfig{
		TaskID:  "task-takeover",
		WorkDir: tempDir,
	})
	if err != nil {
		t.Fatalf("SpawnPTY: %v", err)
	}
	defer proc.Close()

	sess := NewSession("sess-3", "task-takeover", proc, ModeWatch)
	defer sess.Close()

	sub, _, err := sess.Subscribe("sub-toggle", ModeWatch)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sess.Unsubscribe("sub-toggle")

	// Initially watch mode: input must fail
	_, err = sess.WriteInput(sub.ID, []byte("ls\n"))
	if err != ErrReadOnlySession {
		t.Fatalf("expected ErrReadOnlySession initially, got: %v", err)
	}

	// Switch to interactive (Take-Over)
	if err := sess.SetSubscriberMode(sub.ID, ModeInteractive); err != nil {
		t.Fatalf("SetSubscriberMode: %v", err)
	}

	// Now input must succeed
	_, err = sess.WriteInput(sub.ID, []byte("echo 'TAKEN_OVER'\n"))
	if err != nil {
		t.Fatalf("expected input to succeed after takeover, got: %v", err)
	}
}

func TestSession_HistoryBuffer(t *testing.T) {
	tempDir := t.TempDir()

	proc, err := SpawnPTY(context.Background(), PTYConfig{
		TaskID:  "task-history",
		WorkDir: tempDir,
	})
	if err != nil {
		t.Fatalf("SpawnPTY: %v", err)
	}
	defer proc.Close()

	sess := NewSession("sess-4", "task-history", proc, ModeInteractive)
	defer sess.Close()

	marker := "HISTORICAL_OUTPUT_MARKER"
	_, err = proc.Write([]byte("echo '" + marker + "'\n"))
	if err != nil {
		t.Fatalf("Write to PTY: %v", err)
	}

	// Allow reader pump to capture history
	time.Sleep(100 * time.Millisecond)

	// New subscriber joins
	sub, history, err := sess.Subscribe("sub-late", ModeWatch)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer sess.Unsubscribe("sub-late")

	if !strings.Contains(string(history), marker) {
		// If not in initial snapshot, wait for subscriber channel
		found := false
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case chunk := <-sub.SendChan:
				if strings.Contains(string(chunk), marker) {
					found = true
					break
				}
			case <-time.After(50 * time.Millisecond):
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatalf("expected historical buffer to include %q", marker)
		}
	}
}

func TestSession_MultipleSubscribers(t *testing.T) {
	tempDir := t.TempDir()

	proc, err := SpawnPTY(context.Background(), PTYConfig{
		TaskID:  "task-multi",
		WorkDir: tempDir,
	})
	if err != nil {
		t.Fatalf("SpawnPTY: %v", err)
	}
	defer proc.Close()

	sess := NewSession("sess-5", "task-multi", proc, ModeInteractive)
	defer sess.Close()

	sub1, _, err := sess.Subscribe("sub-1", ModeWatch)
	if err != nil {
		t.Fatalf("sub1: %v", err)
	}
	defer sess.Unsubscribe("sub-1")

	sub2, _, err := sess.Subscribe("sub-2", ModeInteractive)
	if err != nil {
		t.Fatalf("sub2: %v", err)
	}
	defer sess.Unsubscribe("sub-2")

	if sess.SubscriberCount() != 2 {
		t.Fatalf("expected 2 subscribers, got %d", sess.SubscriberCount())
	}

	marker := "BROADCAST_TEST_456"
	_, err = sess.WriteInput(sub2.ID, []byte("echo '"+marker+"'\n"))
	if err != nil {
		t.Fatalf("WriteInput: %v", err)
	}

	// Both subscribers should observe output
	found1, found2 := false, false
	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) && (!found1 || !found2) {
		select {
		case chunk := <-sub1.SendChan:
			if strings.Contains(string(chunk), marker) {
				found1 = true
			}
		case chunk := <-sub2.SendChan:
			if strings.Contains(string(chunk), marker) {
				found2 = true
			}
		case <-time.After(20 * time.Millisecond):
		}
	}

	if !found1 {
		t.Errorf("subscriber 1 did not receive broadcast")
	}
	if !found2 {
		t.Errorf("subscriber 2 did not receive broadcast")
	}
}

package terminal

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSocketBridge_LifecycleAndStreaming(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-bridge-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	taskID := "test-task-bridge-123"
	sockPath := filepath.Join(tmpDir, "bridge.sock")

	// 1. Spawn a real PTY process running sh
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proc, err := SpawnPTY(ctx, PTYConfig{
		TaskID:      taskID,
		WorkDir:     tmpDir,
		Shell:       "/bin/sh",
		InitialCols: 80,
		InitialRows: 24,
	})
	if err != nil {
		t.Fatalf("spawn pty: %v", err)
	}
	defer proc.Close()

	sess := NewSession("sess-1", taskID, proc, ModeInteractive)
	defer sess.Close()

	// 2. Start SocketBridge
	bridge, err := NewSocketBridge(sess, sockPath)
	if err != nil {
		t.Fatalf("new socket bridge: %v", err)
	}
	defer bridge.Close()

	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Fatalf("socket file does not exist at %s", sockPath)
	}

	// 3. Connect client 1 via Unix domain socket
	conn1, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial unix socket: %v", err)
	}
	defer conn1.Close()

	// Send an echo command to the shell
	cmd := "echo CUAN_PTY_LIVE_TEST_OK\n"
	if _, err := conn1.Write([]byte(cmd)); err != nil {
		t.Fatalf("write to socket: %v", err)
	}

	// Read output from PTY
	buf := make([]byte, 4096)
	foundOutput := false
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, rErr := conn1.Read(buf)
		if n > 0 {
			out := string(buf[:n])
			if strings.Contains(out, "CUAN_PTY_LIVE_TEST_OK") {
				foundOutput = true
				break
			}
		}
		if rErr != nil && !strings.Contains(rErr.Error(), "timeout") {
			break
		}
	}

	if !foundOutput {
		t.Errorf("expected to read CUAN_PTY_LIVE_TEST_OK from socket, but didn't receive it")
	}

	// 4. Simulate web server restart / reconnect:
	// Close client 1
	_ = conn1.Close()

	// Connect client 2 (like after cuan-server restarts during deploy)
	conn2, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("client 2 dial unix socket: %v", err)
	}
	defer conn2.Close()

	// Client 2 should immediately receive scrollback history containing CUAN_PTY_LIVE_TEST_OK
	receivedHistory := false
	deadline2 := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline2) {
		_ = conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, rErr := conn2.Read(buf)
		if n > 0 {
			out := string(buf[:n])
			if strings.Contains(out, "CUAN_PTY_LIVE_TEST_OK") {
				receivedHistory = true
				break
			}
		}
		if rErr != nil && !strings.Contains(rErr.Error(), "timeout") {
			break
		}
	}

	if !receivedHistory {
		t.Errorf("client 2 expected to receive scrollback history containing previous output")
	}

	// 5. Test resize control message over socket
	resizeMsg := `{"type":"resize","cols":120,"rows":35}`
	if _, err := conn2.Write([]byte(resizeMsg)); err != nil {
		t.Errorf("write resize to socket: %v", err)
	}

	// Wait briefly for resize to apply
	time.Sleep(100 * time.Millisecond)
	cols, rows := proc.GetSize()
	if cols != 120 || rows != 35 {
		t.Errorf("expected PTY size 120x35 after resize message, got %dx%d", cols, rows)
	}

	// 6. Close bridge and verify socket file is removed
	if err := bridge.Close(); err != nil {
		t.Errorf("bridge close error: %v", err)
	}

	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Errorf("expected socket file to be deleted after bridge close")
	}
}

func TestSocketBridge_ConcatenatedControlFrames(t *testing.T) {
	tmpDir := t.TempDir()
	taskID := "task-concat-frames-test"
	sockPath := filepath.Join(tmpDir, "concat.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proc, err := SpawnPTY(ctx, PTYConfig{
		TaskID:      taskID,
		WorkDir:     tmpDir,
		Shell:       "/bin/sh",
		InitialCols: 80,
		InitialRows: 24,
	})
	if err != nil {
		t.Fatalf("spawn pty: %v", err)
	}
	defer proc.Close()

	sess := NewSession("sess-concat", taskID, proc, ModeInteractive)
	defer sess.Close()

	bridge, err := NewSocketBridge(sess, sockPath)
	if err != nil {
		t.Fatalf("new socket bridge: %v", err)
	}
	defer bridge.Close()

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer conn.Close()

	// Send two concatenated JSON control frames in a single write buffer
	concatFrames := `{"type":"stdin","data":"echo HELLO_"}{"type":"stdin","data":"WORLD\n"}`
	if _, err := conn.Write([]byte(concatFrames)); err != nil {
		t.Fatalf("write concat frames: %v", err)
	}

	buf := make([]byte, 2048)
	foundTarget := false
	leakedJSON := false
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, rErr := conn.Read(buf)
		if n > 0 {
			out := string(buf[:n])
			if strings.Contains(out, "HELLO_WORLD") {
				foundTarget = true
			}
			if strings.Contains(out, `{"type":"stdin"`) {
				leakedJSON = true
			}
			if foundTarget {
				break
			}
		}
		if rErr != nil && !strings.Contains(rErr.Error(), "timeout") {
			break
		}
	}

	if !foundTarget {
		t.Errorf("expected to read HELLO_WORLD from cat through concatenated stdin frames")
	}
	if leakedJSON {
		t.Errorf("raw JSON frame string leaked into terminal PTY stream!")
	}
}

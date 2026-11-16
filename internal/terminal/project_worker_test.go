package terminal

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestProjectWorker_RunAndSocketBridge(t *testing.T) {
	projectID := "test_proj_worker_123"
	workDir := t.TempDir()

	payload := ProjectWorkerPayload{
		ProjectID: projectID,
		WorkDir:   workDir,
		Shell:     "/bin/sh",
		Cols:      80,
		Rows:      24,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunProjectWorker(ctx, payload)
	}()

	// Wait for socket to become active
	sockPath := ProjectSocketPath(projectID)
	deadline := time.Now().Add(3 * time.Second)
	active := false
	for time.Now().Before(deadline) {
		if ProjectSocketExists(projectID) {
			active = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !active {
		t.Fatalf("expected project socket %s to be active", sockPath)
	}
	defer func() {
		_ = TerminateProjectWorker(projectID)
	}()

	// 1. Connect first client over Unix domain socket
	conn1, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to dial project socket: %v", err)
	}

	// Send command
	_, err = conn1.Write([]byte("echo 'HELLO_PROJECT_SOCKET_BRIDGE'\n"))
	if err != nil {
		t.Fatalf("write to socket failed: %v", err)
	}

	// Read response
	buf := make([]byte, 4096)
	var output1 strings.Builder
	readDeadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(output1.String(), "HELLO_PROJECT_SOCKET_BRIDGE") {
		if time.Now().After(readDeadline) {
			t.Fatalf("timed out waiting for output on socket 1, got: %q", output1.String())
		}
		_ = conn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, rErr := conn1.Read(buf)
		if n > 0 {
			output1.Write(buf[:n])
		}
		if rErr != nil && !strings.Contains(output1.String(), "HELLO_PROJECT_SOCKET_BRIDGE") {
			time.Sleep(50 * time.Millisecond)
		}
	}
	_ = conn1.Close()

	// 2. Re-attach second client (simulates deploy reconnect!)
	conn2, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to reconnect to project socket: %v", err)
	}
	defer conn2.Close()

	// Reconnected client should receive scrollback buffer containing earlier output
	var output2 strings.Builder
	readDeadline2 := time.Now().Add(3 * time.Second)
	for !strings.Contains(output2.String(), "HELLO_PROJECT_SOCKET_BRIDGE") {
		if time.Now().After(readDeadline2) {
			t.Fatalf("timed out waiting for replayed history on reconnect, got: %q", output2.String())
		}
		_ = conn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, rErr := conn2.Read(buf)
		if n > 0 {
			output2.Write(buf[:n])
		}
		if rErr != nil && !strings.Contains(output2.String(), "HELLO_PROJECT_SOCKET_BRIDGE") {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// 3. Test resize control message over socket
	resizeMsg := `{"type":"resize","cols":100,"rows":30}`
	if _, err := conn2.Write([]byte(resizeMsg)); err != nil {
		t.Fatalf("write resize to socket failed: %v", err)
	}

	// 4. Terminate via cancel context
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error from RunProjectWorker: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for worker to exit after cancel")
	}

	// Socket should be gone
	if ProjectSocketExists(projectID) {
		t.Fatal("expected project socket to be cleaned up after exit")
	}
}

func TestProjectWorker_Terminate(t *testing.T) {
	projectID := "test_proj_terminate_456"
	workDir := t.TempDir()

	payload := ProjectWorkerPayload{
		ProjectID: projectID,
		WorkDir:   workDir,
		Shell:     "/bin/sh",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunProjectWorker(ctx, payload)
	}()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ProjectSocketExists(projectID) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !ProjectSocketExists(projectID) {
		t.Fatal("expected socket to exist before termination")
	}

	if err := TerminateProjectWorker(projectID); err != nil {
		t.Fatalf("TerminateProjectWorker error: %v", err)
	}

	// Wait briefly for cleanup
	time.Sleep(100 * time.Millisecond)
	if ProjectSocketExists(projectID) {
		t.Fatal("expected socket to be cleaned up after termination")
	}
}

func TestProjectWorker_CLI_Validation(t *testing.T) {
	// Missing project-id should fail
	if code := RunProjectWorkerCLI([]string{}); code != 1 {
		t.Fatalf("expected exit code 1 for missing project-id, got %d", code)
	}

	// Missing payload file should fail
	if code := RunProjectWorkerCLI([]string{"--payload-file=/nonexistent/path/payload.json"}); code != 1 {
		t.Fatalf("expected exit code 1 for nonexistent payload file, got %d", code)
	}
}

func TestSpawnDetachedProjectWorker_RealBinary(t *testing.T) {
	projectID := "test_proj_spawn_detached_789"
	workDir := t.TempDir()

	payload := ProjectWorkerPayload{
		ProjectID: projectID,
		WorkDir:   workDir,
		Shell:     "/bin/sh",
	}

	cmd, err := SpawnDetachedProjectWorker(payload, "")
	if err != nil {
		t.Skipf("skipping detached worker test: %v", err)
	}
	defer func() {
		_ = TerminateProjectWorker(projectID)
	}()

	if cmd == nil || cmd.Process == nil {
		t.Fatal("expected non-nil process after spawn")
	}

	if !ProjectSocketExists(projectID) {
		t.Fatal("expected project socket to exist after SpawnDetachedProjectWorker")
	}

	sockPath := ProjectSocketPath(projectID)
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to dial spawned project socket: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("echo 'DETACHED_WORKER_ALIVE'\n")); err != nil {
		t.Fatalf("write to socket failed: %v", err)
	}

	buf := make([]byte, 4096)
	var output strings.Builder
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(output.String(), "DETACHED_WORKER_ALIVE") {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for output, got: %q", output.String())
		}
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, rErr := conn.Read(buf)
		if n > 0 {
			output.Write(buf[:n])
		}
		if rErr != nil && !strings.Contains(output.String(), "DETACHED_WORKER_ALIVE") {
			time.Sleep(50 * time.Millisecond)
		}
	}

	if err := TerminateProjectWorker(projectID); err != nil {
		t.Fatalf("TerminateProjectWorker failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if ProjectSocketExists(projectID) {
		t.Fatal("expected socket to be cleaned up after terminate")
	}
}

package terminal

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestTerminateProcessGroup_TerminatesLeaderAndChildren(t *testing.T) {
	// Spawn a bash process that starts a background sleep child
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}

	pid := cmd.Process.Pid
	if pid <= 0 {
		t.Fatalf("invalid process pid: %d", pid)
	}

	// Verify process is running
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("process should be running: %v", err)
	}

	// Terminate the process group
	TerminateProcessGroup(pid)
	_ = cmd.Wait()

	// Verify both leader and child group are dead (ESRCH)
	deadline := time.Now().Add(500 * time.Millisecond)
	dead := false
	for time.Now().Before(deadline) {
		err := syscall.Kill(-pid, 0)
		if err == syscall.ESRCH {
			dead = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !dead {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		t.Errorf("process group %d was still alive after TerminateProcessGroup", pid)
	}
}

func TestPTYProcess_KillPropagatesSIGINTAndSIGTERM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proc, err := SpawnPTY(ctx, PTYConfig{
		TaskID:      "test-kill-propagate",
		WorkDir:     os.TempDir(),
		Shell:       "/bin/sh",
		Args:        []string{"-c", "sleep 30"},
		InitialCols: 80,
		InitialRows: 24,
	})
	if err != nil {
		t.Fatalf("SpawnPTY failed: %v", err)
	}

	pid := proc.PID()
	if pid <= 0 {
		t.Fatalf("invalid PTY pid: %d", pid)
	}

	// Call Kill()
	if err := proc.Kill(); err != nil {
		t.Fatalf("Kill() returned error: %v", err)
	}

	select {
	case <-proc.Done():
		// Clean exit
	case <-time.After(1 * time.Second):
		t.Errorf("process did not exit within 1s after Kill()")
	}

	// Ensure process is dead
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("process %d was still alive after Kill()", pid)
	}
}

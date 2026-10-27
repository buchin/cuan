package agent

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestProcessTree_KillTerminatesDescendants(t *testing.T) {
	// Spawn a parent process that spawns background sleep grandchild processes
	cmd := exec.Command("bash", "-c", "sleep 30 & sleep 30 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test bash process: %v", err)
	}

	parentPID := cmd.Process.Pid
	// Wait brief moment for bash to fork the background children
	time.Sleep(200 * time.Millisecond)

	childPIDs := GetChildPIDs(parentPID)
	t.Logf("Spawned parent PID %d with %d child/grandchild PIDs: %v", parentPID, len(childPIDs), childPIDs)

	if !IsProcessAlive(parentPID) {
		t.Fatalf("Expected parent PID %d to be alive before cancellation", parentPID)
	}

	// Terminate the entire process tree
	err := KillProcessTree(parentPID)
	if err != nil {
		t.Fatalf("KillProcessTree returned error: %v", err)
	}

	// Reap parent process
	_ = cmd.Wait()

	// Verify parent and all descendants are terminated
	time.Sleep(150 * time.Millisecond)

	if IsProcessAlive(parentPID) {
		t.Errorf("Parent PID %d is still alive after KillProcessTree", parentPID)
	}

	for _, cpid := range childPIDs {
		if IsProcessAlive(cpid) {
			t.Errorf("Child PID %d is still alive after KillProcessTree! Process tree was NOT killed completely.", cpid)
		}
	}

	t.Logf("Process tree verification SUCCESS: Parent PID %d and all %d child processes cleanly killed.", parentPID, len(childPIDs))
}

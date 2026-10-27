package agent

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// KillProcessTree terminates the process group and any lingering child processes.
func KillProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}

	// 1. Gather any known direct child PIDs before killing the parent
	childPids := GetChildPIDs(pid)

	// 2. Send SIGTERM to process group
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	// Brief grace period for clean teardown
	time.Sleep(100 * time.Millisecond)

	// 3. Send SIGKILL to the entire process group
	_ = syscall.Kill(-pid, syscall.SIGKILL)

	// 4. Force kill any individual children that might have detached
	for _, cpid := range childPids {
		_ = syscall.Kill(cpid, syscall.SIGKILL)
	}

	return nil
}

// GetChildPIDs returns child process IDs of a given PID by inspecting /proc/<pid>/task/<pid>/children.
func GetChildPIDs(pid int) []int {
	if pid <= 0 {
		return nil
	}

	path := fmt.Sprintf("/proc/%d/task/%d/children", pid, pid)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var pids []int
	fields := strings.Fields(string(data))
	for _, f := range fields {
		if cpid, err := strconv.Atoi(f); err == nil && cpid > 0 {
			pids = append(pids, cpid)
			// Recursively fetch grandchildren
			pids = append(pids, GetChildPIDs(cpid)...)
		}
	}
	return pids
}

// IsProcessAlive checks if a process with the given PID is still active in the OS.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Sending signal 0 performs error checking without actually sending a signal
	err := syscall.Kill(pid, 0)
	return err == nil
}

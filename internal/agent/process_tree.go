package agent

import (
	"github.com/buchin/agentrunner"
)

// KillProcessTree terminates the process group and any lingering child processes.
func KillProcessTree(pid int) error {
	return agentrunner.KillProcessTree(pid)
}

// GetChildPIDs returns child process IDs of a given PID by inspecting /proc/<pid>/task/<pid>/children.
func GetChildPIDs(pid int) []int {
	return agentrunner.GetChildPIDs(pid)
}

// IsProcessAlive checks if a process with the given PID is still active in the OS.
func IsProcessAlive(pid int) bool {
	return agentrunner.IsProcessAlive(pid)
}

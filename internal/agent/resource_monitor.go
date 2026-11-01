package agent

import (
	"time"

	"github.com/buchin/agentrunner"
)

// ProcessMonitor periodically samples process RSS bytes until stopped.
type ProcessMonitor = agentrunner.ProcessMonitor

// StartProcessMonitor starts sampling memory usage for the given pid.
func StartProcessMonitor(pid int, sampleInterval time.Duration) *ProcessMonitor {
	return agentrunner.StartProcessMonitor(pid, sampleInterval)
}

// readRSS reads the resident set size in bytes from /proc/<pid>/statm.
func readRSS(pid int) int64 {
	return agentrunner.ReadRSS(pid)
}

// Package taskprocess provides durable, OS-backed ownership for local task
// agents. Extracted and powered by github.com/buchin/agentrunner.
package taskprocess

import (
	"io"
	"os"

	"github.com/buchin/agentrunner"
)

const (
	Command               = agentrunner.CuanSupervisorCommand
	ConflictExit          = agentrunner.ConflictExit
	DefaultTaskTimeout    = agentrunner.DefaultBaseTimeout
	DefaultRequeueTimeout = agentrunner.DefaultRequeueTimeout
	DefaultMaxCapTimeout  = agentrunner.DefaultMaxCapTimeout
	DefaultGracePeriod    = agentrunner.DefaultGracePeriod
)

var ErrActive = agentrunner.ErrActive

// OpenLock opens or creates a lock file at path.
func OpenLock(path string) (*os.File, error) {
	return agentrunner.OpenLock(path)
}

// Acquire acquires an exclusive non-blocking lock on path.
func Acquire(path string) (*os.File, error) {
	return agentrunner.Acquire(path)
}

// Active checks whether the lock is active.
func Active(path string) (bool, error) {
	return agentrunner.Active(path)
}

// Run executes a supervised process with OS-backed locking, parent-death tracking,
// and progressive timeout guardrails via the agentrunner library.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return agentrunner.Run(args, stdin, stdout, stderr)
}

//go:build linux || darwin

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// AcquireRunnerInstanceLock acquires an exclusive non-blocking advisory lock for runnerID
// so two cuan-runner daemons cannot share the same runner_id and corrupt each other's heartbeats/leases.
func AcquireRunnerInstanceLock(dir, runnerID string) (func(), error) {
	if strings.TrimSpace(runnerID) == "" {
		return func() {}, nil
	}
	if strings.TrimSpace(dir) == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create runner lock dir: %w", err)
	}
	safeID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, runnerID)
	lockPath := filepath.Join(dir, fmt.Sprintf("cuan-runner-%s.lock", safeID))
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open runner lock %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another cuan-runner instance already holds lock for runner-id %q (%s): %w", runnerID, lockPath, err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

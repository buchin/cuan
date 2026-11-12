package runner

import (
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// An agent waiting on one long command keeps redrawing "Running command...".
// Counting that animation as progress would hide a real stall, which is exactly
// the failure production showed: attempts kept printing a spinner while making
// no progress and were killed by the wall clock.
func TestSpinnerAnimationIsNotProgress(t *testing.T) {
	var c ptyDiagnosticCollector
	for _, frame := range []string{
		"\r⣯  \x1b[90mRunning command...\x1b[0m",
		"\r⣷  \x1b[90mRunning command...\x1b[0m",
		"\r\x1b[3A⣿  \x1b[90mRunning command...\x1b[0m",
		"\r⢿  \x1b[90mThinking...\x1b[0m",
		"\r⡿  \x1b[90mReading file\x1b[0m",
	} {
		c.Feed([]byte(frame))
		if c.Progressed() {
			t.Fatalf("animation frame reported as progress: %q", frame)
		}
	}
}

// Real work output must refresh the watchdog, otherwise a productive agent
// would be stopped as if it were stuck.
func TestRealOutputCountsAsProgress(t *testing.T) {
	var c ptyDiagnosticCollector
	c.Feed([]byte("PASS  web/src/features/inbox/ApprovalCard.test.tsx\n"))
	if !c.Progressed() {
		t.Fatal("real output did not count as progress")
	}

	c.Feed([]byte("\r⣯  \x1b[90mRunning command...\x1b[0m"))
	if c.Progressed() {
		t.Fatal("progress must reset between reads")
	}

	c.Feed([]byte("Edited 2 files (ApprovalCard.tsx, .gitignore)\n"))
	if !c.Progressed() {
		t.Fatal("edit activity did not count as progress")
	}
}

// The idle budget must be long enough to outlast a single slow command but far
// shorter than the wall-clock ceiling, which is now only a safety bound.
func TestProgressIdleTimeoutIsShorterThanCeiling(t *testing.T) {
	idle := task.ProgressIdleTimeout()
	if idle <= 0 {
		t.Fatal("idle timeout must be positive")
	}
	ceiling := task.CalculateTaskTimeout("code", 1, "Build auth", "implement login")
	if idle >= ceiling {
		t.Fatalf("idle timeout %v must be shorter than the ceiling %v", idle, ceiling)
	}
	if idle > 20*time.Minute {
		t.Fatalf("idle timeout %v is too long to free a stuck lane promptly", idle)
	}
}

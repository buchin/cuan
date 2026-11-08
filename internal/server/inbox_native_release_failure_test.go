package server

import (
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestNativeReleaseFailureOutput(t *testing.T) {
	marker := "Native release process ended; reconciling its saved result and production state."
	logs := []*task.LogRecord{
		{AttemptID: "current", Stream: "system", Line: "unrelated initialization"},
		{AttemptID: "other", Stream: "system", Line: marker},
		{AttemptID: "current", Stream: "stdout", Line: "untrusted stdout"},
	}
	if got := nativeReleaseFailureOutput("current", logs); got != "" {
		t.Fatalf("unbound output included: %q", got)
	}
	logs = append(logs, &task.LogRecord{AttemptID: "current", Stream: "system", Line: marker},
		&task.LogRecord{AttemptID: "current", Stream: "system", Line: "Frontend gate failed: obsolete wait assertion"},
		&task.LogRecord{AttemptID: "current", Stream: "system", Line: "later administrative message"})
	if got := nativeReleaseFailureOutput("current", logs); got != "Frontend gate failed: obsolete wait assertion" {
		t.Fatalf("lost actual failure: %q", got)
	}
	logs = append(logs, &task.LogRecord{AttemptID: "current", Stream: "system", Line: marker},
		&task.LogRecord{AttemptID: "current", Stream: "system", Line: strings.Repeat("x", 9000)})
	if got := nativeReleaseFailureOutput("current", logs); len(got) > 6100 {
		t.Fatalf("unbounded diagnostic: %d", len(got))
	}
}

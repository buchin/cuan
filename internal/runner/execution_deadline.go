package runner

import (
	"strings"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// Persist the same upper bound used by native verification execution. Recognizing
// a marker only shortens a deadline; launch validation still grants capabilities.
func claimExecutionDeadline(t *task.Task, a *task.TaskAttempt, started time.Time, revision string) time.Time {
	deadline := started.Add(task.CalculateTaskTimeout(t.ExecutionCategory, a.AttemptNumber, t.Title, t.Instructions))
	verification := strings.Contains(revision, task.VerificationIntentMarker)
	deadline = verificationDeadline(deadline, started, verification || strings.Contains(revision, task.LiveInspectionMarker))
	if verification && strings.Contains(revision, `"finalize_attempt_id":`) && started.Add(30*time.Second).Before(deadline) {
		deadline = started.Add(30 * time.Second)
	}
	return deadline
}

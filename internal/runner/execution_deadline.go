package runner

import (
	"strings"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// Independent checks need time after the agent exits. This partitions the
// existing execution window; neither preparation nor model routing can extend it.
func inspectionReportingDeadline(overall, now time.Time, reserveLimit time.Duration, projectChecks bool) time.Time {
	if reserveLimit <= 0 || !overall.After(now) {
		return overall
	}
	reserve := overall.Sub(now) / 5
	if projectChecks {
		// Full native project checks can consume their five-minute command
		// budget. Share the remaining window equally, capped by that budget,
		// rather than giving model investigation four times as much time.
		reserve = overall.Sub(now) / 2
	}
	if reserve > reserveLimit {
		reserve = reserveLimit
	}
	return overall.Add(-reserve)
}

const inspectionReportingTimeout = task.InspectionReportingTimeout

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

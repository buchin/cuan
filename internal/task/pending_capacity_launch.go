package task

import (
	"strings"
	"time"
)

// A newer, already-saved continuation can be stranded when an older attempt
// reports quota failure after admission deferred that continuation. This does
// not create an execution, retry budget or authorization.
func pendingCapacityContinuation(t *Task, j *TaskLaunch, a *TaskAttempt, now time.Time) bool {
	return t != nil && t.Status == TaskStatusBlocked && j != nil && j.State == "queued" && j.ExpiresAt.After(now) && j.ProjectID == t.ProjectID && j.InstructionsHash == LaunchInstructionsHash(t.Instructions) && a != nil && a.TaskID == t.ID && a.ID != attemptRequestID(t.ID, j.RequestID) && a.Status == AttemptStatusFailed && a.ExitCode != nil && *a.ExitCode == 75 && a.FinishedAt != nil && !a.CancelRequested && a.CanceledAt == nil && strings.HasPrefix(a.ErrorMessage, "The AI provider reported a usage limit. The run was stopped;")
}

package runner

import (
	"github.com/buchin/cuan/internal/task"
	"testing"
	"time"
)

func TestClaimDeadlineMatchesVerificationExecutionAndReaper(t *testing.T) {
	started := time.Now().UTC()
	item := &task.Task{ExecutionCategory: task.ExecutionCategoryCode, Title: "Implement approved feature"}
	attempt := &task.TaskAttempt{AttemptNumber: 8, CreatedAt: started.Add(-time.Hour)}
	for _, tc := range []struct {
		name, revision string
		cap            time.Duration
	}{
		{"ordinary", "", 0},
		{"live", task.LiveInspectionMarker + `{}`, 10 * time.Minute},
		{"verification", task.VerificationIntentMarker + `{}`, 10 * time.Minute},
		{"finalizer", task.VerificationIntentMarker + `{"finalize_attempt_id":"source"}`, 30 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := claimExecutionDeadline(item, attempt, started, tc.revision)
			base := started.Add(task.CalculateTaskTimeout(item.ExecutionCategory, attempt.AttemptNumber, item.Title, item.Instructions))
			if got.After(base) {
				t.Fatal("verification extended execution budget")
			}
			if tc.cap == 0 && !got.Equal(base) {
				t.Fatal("ordinary execution policy changed")
			}
			if tc.cap > 0 && got.After(started.Add(tc.cap)) {
				t.Fatal("stored deadline exceeds native cap")
			}
			copy := *attempt
			copy.StartedAt = &started
			copy.ExecutionDeadline = &got
			if !task.AttemptDeadline(item, &copy).Equal(got) {
				t.Fatal("reaper does not share immutable deadline")
			}
		})
	}
}

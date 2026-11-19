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
		{"live", task.LiveInspectionMarker + `{}`, 25 * time.Minute},
		{"verification", task.VerificationIntentMarker + `{}`, 25 * time.Minute},
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

func TestInspectionReportingBudgetPreservesOriginalDeadline(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name          string
		remaining     time.Duration
		reserveLimit  time.Duration
		reserve       time.Duration
		projectChecks bool
	}{
		{"ordinary", 45 * time.Minute, 0, 0, false},
		{"live inspection", 10 * time.Minute, projectVerifierTimeout, 5 * time.Minute, true},
		{"source verification", 10 * time.Minute, 30 * time.Second, 30 * time.Second, false},
		{"preparation already consumed time", 5 * time.Minute, projectVerifierTimeout, 150 * time.Second, true},
		{"long inspection remains capped", 20 * time.Minute, projectVerifierTimeout, 5 * time.Minute, true},
		{"short remaining window", time.Second, projectVerifierTimeout, 500 * time.Millisecond, true},
		{"expired", -time.Second, projectVerifierTimeout, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			overall := now.Add(tc.remaining)
			got := inspectionReportingDeadline(overall, now, tc.reserveLimit, tc.projectChecks)
			if !got.Equal(overall.Add(-tc.reserve)) || got.After(overall) {
				t.Fatalf("unexpected reporting deadline %v", got)
			}
		})
	}
}

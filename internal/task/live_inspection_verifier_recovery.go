package task

import (
	"context"
	"encoding/json"
	"strings"
)

// One narrowly identified retry repairs Cuan's known PHPUnit/Pest invocation
// mismatch. Actual failing assertions, changed source, interruption and cancelled
// executions never gain this allowance. The failed attempt remains historical.
func ValidateInspectionVerifierRecovery(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	if t == nil || proof == nil || proof.VerifierRecoveryAttemptID == "" || proof.Purpose != CurrentOutcomeInspection || proof.PreparationLaunchID != "" || proof.PreviousAttemptID != "" || proof.ReviewRequestID != "" || len(proof.ReviewPaths) != 0 {
		return ErrInvalidTaskInput
	}
	a, err := store.GetAttempt(ctx, proof.VerifierRecoveryAttemptID)
	if err != nil {
		return err
	}
	if a == nil || a.TaskID != t.ID || a.Status != AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 0 || a.ErrorMessage != "Live result inspection project checks did not pass" || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || proof.ObservedAt.Before(*a.FinishedAt) {
		return ErrInvalidTaskInput
	}
	previous, err := LiveInspectionForAttempt(ctx, store, t, a.ID)
	if err != nil || previous == nil || previous.VerifierRecoveryAttemptID != "" {
		return ErrInvalidTaskInput
	}
	// The whole intent must match; only a new observation time and the bounded
	// recovery identity differ. No target or accepted source can be substituted.
	expected := *proof
	expected.VerifierRecoveryAttemptID = ""
	expected.ObservedAt = previous.ObservedAt
	left, _ := json.Marshal(expected)
	right, _ := json.Marshal(previous)
	if string(left) != string(right) {
		return ErrInvalidTaskInput
	}
	events, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return err
	}
	if !liveImplementationFinalReport(events, t.ID, a.ID) {
		return ErrInvalidTaskInput
	}
	d := &TaskDetailResponse{Task: *t, LatestAttempt: a, Activities: events}
	evidence := AttemptVerificationEvidence(d)
	if evidence.Author != a.RunnerID || !strings.HasPrefix(evidence.Current, "Project verification: php vendor/bin/phpunit --colors=never tests/") || !strings.Contains(evidence.Current, "(exit 1)\nPest\\Exceptions\\InvalidPestCommand") || !strings.Contains(evidence.Current, "Please run [./vendor/bin/pest] instead.") {
		return ErrInvalidTaskInput
	}
	for _, e := range events {
		if e == nil || e.ID != "repository-observation:"+a.ID || e.TaskID != t.ID || e.AttemptID == nil || *e.AttemptID != a.ID || e.Author != a.RunnerID || e.ActivityType != "repository_scope_observed" || e.NewStatus != AttemptStatusFailed {
			continue
		}
		var observed struct {
			Baseline string `json:"baseline"`
			Final    string `json:"final"`
		}
		if json.Unmarshal([]byte(e.Message), &observed) == nil && validVerificationHash(observed.Baseline) && observed.Baseline == observed.Final {
			return nil
		}
	}
	return ErrInvalidTaskInput
}

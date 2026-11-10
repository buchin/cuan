package task

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

// NativeCheckRecipeReview identifies a saved PM replan after an authenticated
// aborted registered-script check. A deployment alone is not corrective evidence:
// admission and the runner independently require this exact recipe to have changed.
func NativeCheckRecipeReview(t *Task, previous *TaskAttempt, events []*TaskActivity, requestID string) (string, []string) {
	if t == nil || previous == nil || !inspectionProjectVerificationDeadlineFailure(previous, events) {
		return "", nil
	}
	checks := AttemptVerificationEvidence(&TaskDetailResponse{Task: *t, LatestAttempt: previous, Activities: events})
	if checks.Author != previous.RunnerID || !strings.HasPrefix(checks.Current, "Project verification: test.sh --fast (exit -1)\n") {
		return "", nil
	}
	var latest *TaskActivity
	var saved outcomeRevalidationReviewRecord
	for _, event := range events {
		if event == nil || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != previous.ID || event.Author != "coordinator" || event.ActivityType != "coordinator_review_assessment" || event.CreatedAt.Before(*previous.FinishedAt) || event.CreatedAt.After(time.Now().UTC()) {
			continue
		}
		var record outcomeRevalidationReviewRecord
		if json.Unmarshal([]byte(event.Message), &record) != nil || record.InstructionsHash != LaunchInstructionsHash(t.Instructions) || record.RequestID == "" {
			continue
		}
		if latest == nil || event.CreatedAt.After(latest.CreatedAt) || event.CreatedAt.Equal(latest.CreatedAt) && event.ID > latest.ID {
			latest, saved = event, record
		}
	}
	a := saved.Assessment
	if latest == nil || requestID != "" && requestID != saved.RequestID || len(saved.EvidenceDigest) != 64 || a.Decision != "revise" || a.ManagerAction != "replan" || a.RequestedOutcome != "unfinished" || (a.RemainingWork != "verification" && a.RemainingWork != "publication") || strings.TrimSpace(a.ManagerReason) == "" || strings.TrimSpace(a.Revision) == "" {
		return "", nil
	}
	// This path is selected by the native verifier's registered-script recipe,
	// never extracted from provider prose or arbitrary owner-supplied filenames.
	return saved.RequestID, []string{"scripts/test.sh"}
}

func NativeCheckRecipeRevalidation(proof *LiveInspectionIntent) bool {
	return proof != nil && proof.Purpose == NativeReleaseInspection && proof.PreviousAttemptID != "" && proof.ReviewRequestID != "" && proof.RevalidationKind == "" && len(proof.ReviewPaths) > 0
}

func validateNativeCheckRecipeRevalidation(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	previous, err := store.GetAttempt(ctx, proof.PreviousAttemptID)
	if err != nil || previous == nil || previous.TaskID != t.ID || previous.FinishedAt == nil || proof.ObservedAt.Before(*previous.FinishedAt) || proof.TargetCommit == proof.PreviousCommit {
		return ErrInvalidTaskInput
	}
	binding, err := LiveInspectionForAttempt(ctx, store, t, previous.ID)
	if err != nil || binding == nil || binding.Purpose != NativeReleaseInspection || NativeCheckRecipeRevalidation(binding) {
		return ErrInvalidTaskInput
	}
	// Revalidate the complete historical lineage. No prior budget, failed check,
	// approval, report or acceptance receipt is rewritten or promoted to success.
	if err := ValidateLiveInspectionSource(ctx, store, t, binding); err != nil {
		return err
	}
	expected := *proof
	expected.PreviousAttemptID, expected.PreviousCommit = binding.PreviousAttemptID, binding.PreviousCommit
	expected.ReviewRequestID, expected.ReviewPaths, expected.RevalidationKind = binding.ReviewRequestID, binding.ReviewPaths, binding.RevalidationKind
	expected.TargetCommit, expected.ObservedAt = binding.TargetCommit, binding.ObservedAt
	expected.ReportingRecoveryAttemptID, expected.VerifierRecoveryAttemptID, expected.CapacityRecoveryAttemptID = binding.ReportingRecoveryAttemptID, binding.VerifierRecoveryAttemptID, binding.CapacityRecoveryAttemptID
	if !reflect.DeepEqual(expected, *binding) || proof.PreviousCommit != binding.TargetCommit || proof.ReportingRecoveryAttemptID != "" || proof.VerifierRecoveryAttemptID != "" || proof.CapacityRecoveryAttemptID != "" {
		return ErrInvalidTaskInput
	}
	events, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return err
	}
	request, paths := NativeCheckRecipeReview(t, previous, events, proof.ReviewRequestID)
	if request == "" || request != proof.ReviewRequestID || !reflect.DeepEqual(paths, proof.ReviewPaths) {
		return ErrInvalidTaskInput
	}
	return nil
}

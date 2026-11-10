package task

import (
	"context"
	"encoding/json"
	"time"
)

// The existing read-only live-inspection lane can verify a delivered native
// target without changing the original task's implementation source identity.
func validateNativeReleaseInspectionSource(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	if NativeCheckRecipeRevalidation(proof) {
		return validateNativeCheckRecipeRevalidation(ctx, store, t, proof)
	}
	if proof.PreviousAttemptID != "" && proof.RevalidationKind == EvidenceRevalidation && proof.ReviewRequestID != "" && len(proof.ReviewPaths) == 0 {
		return validateOutcomeRevalidation(ctx, store, t, proof)
	}
	if proof.PreparationLaunchID != "" || proof.PreviousAttemptID != "" || proof.PreviousCommit != "" || proof.RevalidationKind != "" || proof.ReviewRequestID != "" || len(proof.ReviewPaths) != 0 || proof.ObservedAt.IsZero() || proof.ObservedAt.After(time.Now().Add(5*time.Second)) {
		return ErrInvalidTaskInput
	}
	delivery, err := NativeReleaseDeliveryForAttempt(ctx, store, t, proof.SourceAttemptID)
	if reason, ok := err.(nativeDeliveryValidationError); ok {
		return reason
	}
	if err != nil || delivery == nil {
		return ErrInvalidTaskInput
	}
	if proof.SourceCommit != delivery.SourceCommit || proof.Repository != delivery.Repository || proof.InstructionsHash != delivery.InstructionsHash {
		return ErrInvalidTaskInput
	}
	if proof.VerifierRecoveryAttemptID == "" {
		if proof.TargetCommit != delivery.TargetCommit {
			return ErrInvalidTaskInput
		}
	} else if err := validateNativeInspectionVerifierRecovery(ctx, store, t, proof); err != nil {
		return err
	}
	intent, err := NativeReleaseIntentForAttempt(ctx, store, t, proof.SourceAttemptID)
	if err != nil || intent == nil || intent.OwnerID != proof.OwnerID || intent.RoomID != proof.RoomID || intent.SourceRequestID != proof.SourceRequestID {
		return ErrInvalidTaskInput
	}
	stage, err := store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil || stage == nil || stage.FinishedAt == nil || proof.ObservedAt.Before(*stage.FinishedAt) {
		return ErrInvalidTaskInput
	}
	if proof.SourceVerifierAttemptID != "" {
		review, err := NativeSourceReviewForAttempt(ctx, store, t, proof.SourceVerifierAttemptID)
		if err != nil || review == nil || review.SourceCommit != proof.SourceCommit || review.InstructionsHash != proof.InstructionsHash || review.SourceAttemptID != delivery.SourceAttemptID {
			return ErrInvalidTaskInput
		}
		if intent.SourceReviewAttemptID != "" && intent.SourceReviewAttemptID != proof.SourceVerifierAttemptID {
			return ErrInvalidTaskInput
		}
		verifierAttempt, err := store.GetAttempt(ctx, proof.SourceVerifierAttemptID)
		if err != nil || verifierAttempt == nil || verifierAttempt.FinishedAt == nil || proof.ObservedAt.Before(*verifierAttempt.FinishedAt) || verifierAttempt.CancelRequested || verifierAttempt.CanceledAt != nil {
			return ErrInvalidTaskInput
		}
	}
	return nil
}

// A stale target or authenticated incomplete native check permits one new
// read-only observation. It grants no acceptance credit or publication authority.
func validateNativeInspectionVerifierRecovery(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	a, err := store.GetAttempt(ctx, proof.VerifierRecoveryAttemptID)
	if err != nil || a == nil || a.TaskID != t.ID || a.Status != AttemptStatusFailed || a.ExitCode == nil || a.RunnerID == "" || a.StartedAt == nil || a.FinishedAt == nil || a.FinishedAt.Before(*a.StartedAt) || a.CancelRequested || a.CanceledAt != nil || proof.ObservedAt.Before(*a.FinishedAt) || !fullObjectID(proof.TargetCommit) {
		return ErrInvalidTaskInput
	}
	previous, err := LiveInspectionForAttempt(ctx, store, t, a.ID)
	if err != nil || previous == nil || previous.Purpose != NativeReleaseInspection || previous.VerifierRecoveryAttemptID != "" || previous.ReportingRecoveryAttemptID != "" {
		return ErrInvalidTaskInput
	}
	events, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return err
	}
	infrastructure := InspectionInfrastructureFailure(a, events)
	if !infrastructure && (*a.ExitCode != 1 || a.ErrorMessage != StaleInspectionReleaseMessage || previous.TargetCommit == proof.TargetCommit) {
		return ErrInvalidTaskInput
	}
	expected := *proof
	expected.VerifierRecoveryAttemptID = ""
	expected.TargetCommit, expected.ObservedAt = previous.TargetCommit, previous.ObservedAt
	left, _ := json.Marshal(expected)
	right, _ := json.Marshal(previous)
	if string(left) != string(right) {
		return ErrInvalidTaskInput
	}
	if infrastructure {
		return nil
	}
	for _, event := range events {
		if event == nil || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != a.ID {
			continue
		}
		if event.ActivityType == "agent_report" || event.ActivityType == "repository_scope_observed" || event.ActivityType == ActivityLiveInspection {
			return ErrInvalidTaskInput
		}
	}
	return nil
}

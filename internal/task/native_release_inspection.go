package task

import (
	"context"
	"time"
)

// The existing read-only live-inspection lane can verify a delivered native
// target without changing the original task's implementation source identity.
func validateNativeReleaseInspectionSource(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	if proof.SourceVerifierAttemptID != "" || proof.VerifierRecoveryAttemptID != "" || proof.PreparationLaunchID != "" || proof.PreviousAttemptID != "" || proof.PreviousCommit != "" || proof.RevalidationKind != "" || proof.ReviewRequestID != "" || len(proof.ReviewPaths) != 0 || proof.ObservedAt.IsZero() || proof.ObservedAt.After(time.Now().Add(5*time.Second)) {
		return ErrInvalidTaskInput
	}
	delivery, err := NativeReleaseDeliveryForAttempt(ctx, store, t, proof.SourceAttemptID)
	if err != nil || delivery == nil {
		return ErrInvalidTaskInput
	}
	if proof.SourceCommit != delivery.SourceCommit || proof.TargetCommit != delivery.TargetCommit || proof.Repository != delivery.Repository || proof.InstructionsHash != delivery.InstructionsHash {
		return ErrInvalidTaskInput
	}
	intent, err := NativeReleaseIntentForAttempt(ctx, store, t, proof.SourceAttemptID)
	if err != nil || intent == nil || intent.OwnerID != proof.OwnerID || intent.RoomID != proof.RoomID || intent.SourceRequestID != proof.SourceRequestID {
		return ErrInvalidTaskInput
	}
	stage, err := store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil || stage == nil || stage.FinishedAt == nil || proof.ObservedAt.Before(*stage.FinishedAt) {
		return ErrInvalidTaskInput
	}
	return nil
}

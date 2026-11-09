package task

import (
	"context"
	"time"
)

// One native continuation of a preparation-only failure. The old failed attempt
// stays failed, while source review, authority and publication effects are read
// and reconciled again by the existing native preparation path.
func NativePreparationRecoveryIntent(ctx context.Context, store Store, t *Task) (*NativeReleaseIntent, error) {
	if t == nil || (t.Status != TaskStatusBlocked && t.Status != TaskStatusReview) {
		return nil, ErrInvalidTaskInput
	}
	latest, err := store.GetLatestAttempt(ctx, t.ID)
	if err != nil || latest == nil {
		return nil, ErrInvalidTaskInput
	}
	previous, err := NativeReleaseIntentForAttempt(ctx, store, t, latest.ID)
	if err != nil || previous == nil || previous.PreparationRecoveryAttemptID != "" {
		return nil, ErrInvalidTaskInput
	}
	proof := *previous
	proof.PreparationRecoveryAttemptID = latest.ID
	if err := ValidateNativeReleaseSource(ctx, store, t, &proof); err != nil {
		return nil, err
	}
	return &proof, nil
}

func validateNativePreparationRecovery(ctx context.Context, store Store, t *Task, proof *NativeReleaseIntent) error {
	previous, err := store.GetAttempt(ctx, proof.PreparationRecoveryAttemptID)
	if err != nil || previous == nil || previous.TaskID != t.ID || previous.Status != AttemptStatusFailed || previous.CancelRequested || previous.CanceledAt != nil || previous.RunnerID == "" || previous.StartedAt == nil || previous.FinishedAt == nil || previous.ExitCode == nil || *previous.ExitCode != 1 || previous.ErrorMessage != NativeReleasePreparationUnavailableMessage || proof.SourceReviewAttemptID == "" {
		return ErrInvalidTaskInput
	}
	if previous.CreatedAt.IsZero() || previous.StartedAt.Before(previous.CreatedAt) || previous.FinishedAt.Before(*previous.StartedAt) || previous.FinishedAt.After(time.Now()) {
		return ErrInvalidTaskInput
	}
	original, err := NativeReleaseIntentForAttempt(ctx, store, t, previous.ID)
	if err != nil || original == nil || original.PreparationRecoveryAttemptID != "" {
		return ErrInvalidTaskInput
	}
	expected := *proof
	expected.PreparationRecoveryAttemptID = ""
	if *original != expected {
		return ErrInvalidTaskInput
	}
	return validateNativeReleaseSource(ctx, store, t, &expected, false)
}

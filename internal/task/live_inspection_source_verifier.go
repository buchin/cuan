package task

import "context"

// Bind the immediate verifier separately from the original implementation.
// A verifier receipt never establishes publication or acceptance of that work.
func validateInspectionSourceVerifier(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	if proof.Purpose != CurrentOutcomeInspection || proof.SourceVerifierAttemptID == proof.SourceAttemptID {
		return ErrInvalidTaskInput
	}
	a, err := store.GetAttempt(ctx, proof.SourceVerifierAttemptID)
	if err != nil {
		return err
	}
	if a == nil || a.TaskID != t.ID || a.Status != AttemptStatusSucceeded || a.ExitCode == nil || *a.ExitCode != 0 || a.ErrorMessage != "" || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || proof.ObservedAt.Before(*a.FinishedAt) {
		return ErrInvalidTaskInput
	}
	binding, err := ValidateVerificationSource(ctx, store, t, a.ID)
	if err != nil {
		return err
	}
	if binding == nil || binding.SourceAttemptID != proof.SourceAttemptID || binding.SourceCommit != proof.SourceCommit || binding.OwnerID != proof.OwnerID || binding.RoomID != proof.RoomID || binding.SourceRequestID != proof.SourceRequestID {
		return ErrInvalidTaskInput
	}
	events, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return err
	}
	if VerificationScopeBaseline(&TaskDetailResponse{Task: *t, Attempts: []*TaskAttempt{a}, Activities: events}, a.ID) == "" {
		return ErrInvalidTaskInput
	}
	for _, e := range events {
		if e != nil && e.TaskID == t.ID && e.AttemptID != nil && *e.AttemptID == a.ID && e.Author == a.RunnerID && e.ActivityType == "verification_source_verified" && e.Message == proof.SourceCommit {
			return nil
		}
	}
	return ErrInvalidTaskInput
}

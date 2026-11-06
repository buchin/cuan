package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const InspectionPublicationPreparationError = "verification snapshot could not be published; existing files were preserved"
const InspectionScopePreparationError = "The live result inspection scope could not be validated."

// Only known failures before attempt reservation may use this single repair.
// Eligibility alone is not authorization: the original intent must parse now,
// source/owner/scope must still validate, and no attempt may exist for its request.
func IsInspectionPreparationFailure(parent *TaskLaunch) bool {
	return parent != nil && parent.State == "stopped" && (parent.SafeError == InspectionPublicationPreparationError || parent.SafeError == InspectionScopePreparationError)
}

// A failed preparation has no execution to retry. Preserve that durable intent
// and permit one separately identified preparation, never a chain of attempts.
func ValidateInspectionPreparationRecovery(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	reader, ok := store.(LaunchStore)
	if !ok || t == nil || proof == nil || proof.PreparationLaunchID == "" || proof.PreviousAttemptID != "" || proof.ReviewRequestID != "" || len(proof.ReviewPaths) != 0 {
		return ErrInvalidTaskInput
	}
	parent, err := reader.GetLaunch(ctx, proof.PreparationLaunchID)
	if err != nil {
		return err
	}
	if !IsInspectionPreparationFailure(parent) || !parent.ExpiresAt.After(time.Now().UTC()) {
		return ErrInvalidTaskInput
	}
	old, err := LiveInspectionForLaunch(t, parent)
	if err != nil || old == nil || old.PreparationLaunchID != "" || old.PreviousAttemptID != "" || old.ReviewRequestID != "" || len(old.ReviewPaths) != 0 {
		return ErrInvalidTaskInput
	}
	expected := *proof
	expected.PreparationLaunchID = ""
	expected.TargetCommit = old.TargetCommit
	expected.ObservedAt = old.ObservedAt
	expectedRaw, _ := json.Marshal(expected)
	oldRaw, _ := json.Marshal(old)
	if string(expectedRaw) != string(oldRaw) || proof.ObservedAt.Before(parent.UpdatedAt) {
		return ErrInvalidTaskInput
	}
	if _, err := store.GetAttempt(ctx, attemptRequestID(t.ID, parent.RequestID)); !errors.Is(err, ErrAttemptNotFound) {
		return ErrInvalidTaskInput
	}
	return nil
}

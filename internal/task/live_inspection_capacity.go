package task

import (
	"context"
	"encoding/json"
)

// Capacity recovery creates one durable waiting launch for the exact approved
// inspection. Queueing is not evidence of available capacity or successful work;
// the native task pool is leased again before publishing a claimable attempt.
func ValidateInspectionCapacityRecovery(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	if t == nil || proof == nil || proof.Purpose != CurrentOutcomeInspection || proof.CapacityRecoveryAttemptID == "" || !fullObjectID(proof.TargetCommit) {
		return ErrInvalidTaskInput
	}
	parent, err := store.GetAttempt(ctx, proof.CapacityRecoveryAttemptID)
	if err != nil {
		return err
	}
	if !QuotaStoppedVerification(parent) || parent.TaskID != t.ID || proof.ObservedAt.Before(*parent.FinishedAt) {
		return ErrInvalidTaskInput
	}
	previous, err := LiveInspectionForAttempt(ctx, store, t, parent.ID)
	if err != nil || previous == nil || previous.CapacityRecoveryAttemptID != "" {
		return ErrInvalidTaskInput
	}
	expected := *proof
	expected.CapacityRecoveryAttemptID = ""
	expected.ObservedAt, expected.TargetCommit = previous.ObservedAt, previous.TargetCommit
	left, _ := json.Marshal(expected)
	right, _ := json.Marshal(previous)
	if string(left) != string(right) {
		return ErrInvalidTaskInput
	}
	return nil
}

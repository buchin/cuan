package task

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"time"
)

const ActivityVerificationRuntime = "verification_runtime"
const ActivityVerificationRuntimeRecovery = "verification_runtime_recovery"

// Browser readiness comes from an authenticated runner's actual render probe,
// never an agent report, account status or the presence of an executable.
type VerificationRuntime struct {
	Fingerprint string    `json:"fingerprint"`
	Version     string    `json:"version"`
	CheckedAt   time.Time `json:"checked_at"`
}

func (r *VerificationRuntime) Fresh(now time.Time) bool {
	if r == nil || len(r.Fingerprint) != 64 || r.Version == "" || len(r.Version) > 40 || r.CheckedAt.After(now.Add(5*time.Second)) || r.CheckedAt.Before(now.Add(-10*time.Minute)) {
		return false
	}
	_, err := hex.DecodeString(r.Fingerprint)
	return err == nil
}

// Recovery is one follow-up of the original verifier, never a chain. It carries
// the original implementation proof and a durable authenticated readiness receipt.
func validateVerificationRuntimeRecovery(ctx context.Context, store Store, t *Task, proof *VerificationIntent, activities []*TaskActivity) error {
	if proof.RecoveryCause == "capacity" {
		return ValidateVerificationCapacityRecovery(ctx, store, t, proof, activities)
	}
	parent, err := store.GetAttempt(ctx, proof.RecoveryAttemptID)
	if err != nil {
		return err
	}
	if parent == nil || parent.TaskID != t.ID || parent.Status != AttemptStatusSucceeded || parent.CancelRequested || parent.CanceledAt != nil || parent.FinishedAt == nil {
		return ErrInvalidTaskInput
	}
	original, err := VerificationIntentForAttempt(ctx, store, t, parent.ID)
	if err != nil {
		return err
	}
	if original == nil || original.RecoveryAttemptID != "" || original.SourceAttemptID != proof.SourceAttemptID || original.SourceCommit != proof.SourceCommit || original.SourceVerificationHash != proof.SourceVerificationHash || original.OwnerID != proof.OwnerID || original.RoomID != proof.RoomID || original.SourceRequestID != proof.SourceRequestID {
		return ErrInvalidTaskInput
	}
	sourceVerified, scopeVerified, ready := false, false, false
	for _, a := range activities {
		if a == nil || a.TaskID != t.ID || a.AttemptID == nil || *a.AttemptID != parent.ID || a.Author != parent.RunnerID {
			continue
		}
		switch a.ActivityType {
		case "verification_source_verified":
			sourceVerified = a.Message == proof.SourceCommit
		case "repository_scope_verified":
			scopeVerified = a.NewStatus == "read_only"
		case ActivityVerificationRuntimeRecovery:
			var runtime VerificationRuntime
			if json.Unmarshal([]byte(a.Message), &runtime) == nil && runtime.Fingerprint == proof.RuntimeFingerprint && !runtime.CheckedAt.Before(*parent.FinishedAt) {
				ready = true
			}
		}
	}
	if !sourceVerified || !scopeVerified || !ready {
		return ErrInvalidTaskInput
	}
	return nil
}

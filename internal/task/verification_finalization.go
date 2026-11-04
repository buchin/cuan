package task

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const ActivityRecoveredVerificationArtifacts = "verification_artifacts_recovered"
const ActivityRunnerFinalization = "runner_finalization"
const repositoryScopeReceiptPrefix = "Runner verified unchanged repository state: "

type FinalizationReport struct {
	Summary         string `json:"summary"`
	Verification    string `json:"verification"`
	OriginAttemptID string `json:"origin_attempt_id"`
	ManifestHash    string `json:"manifest_hash"`
}

func validVerificationHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func VerificationScopeBaseline(d *TaskDetailResponse, attemptID string) string {
	if d == nil {
		return ""
	}
	runnerID := ""
	for _, a := range d.Attempts {
		if a != nil && a.ID == attemptID {
			runnerID = a.RunnerID
			break
		}
	}
	if runnerID == "" {
		return ""
	}
	for _, a := range d.Activities {
		if a != nil && a.TaskID == d.ID && a.AttemptID != nil && *a.AttemptID == attemptID && a.Author == runnerID && a.ActivityType == "repository_scope_verified" && a.NewStatus == "read_only" && strings.HasPrefix(a.Message, repositoryScopeReceiptPrefix) {
			baseline := strings.TrimPrefix(a.Message, repositoryScopeReceiptPrefix)
			if validVerificationHash(baseline) {
				return baseline
			}
		}
	}
	return ""
}
func validateVerificationFinalization(ctx context.Context, store Store, t *Task, proof *VerificationIntent, activities []*TaskActivity) error {
	origin, err := store.GetAttempt(ctx, proof.FinalizeAttemptID)
	if err != nil {
		return err
	}
	if origin == nil || origin.TaskID != t.ID || origin.Status != AttemptStatusFailed || origin.ExitCode == nil || *origin.ExitCode != 124 || origin.CancelRequested || origin.CanceledAt != nil {
		return ErrInvalidTaskInput
	}
	parent, err := VerificationIntentForAttempt(ctx, store, t, origin.ID)
	if err != nil {
		return err
	}
	if parent == nil || parent.FinalizeAttemptID != "" || parent.RecoveryAttemptID != proof.RecoveryAttemptID || parent.SourceAttemptID != proof.SourceAttemptID || parent.SourceCommit != proof.SourceCommit || parent.SourceVerificationHash != proof.SourceVerificationHash || parent.RuntimeFingerprint != proof.RuntimeFingerprint || parent.OwnerID != proof.OwnerID || parent.RoomID != proof.RoomID || parent.SourceRequestID != proof.SourceRequestID {
		return ErrInvalidTaskInput
	}
	attempts, err := store.ListAttempts(ctx, t.ID)
	if err != nil {
		return err
	}
	d := &TaskDetailResponse{Task: *t, Activities: activities, Attempts: attempts}
	if VerificationScopeBaseline(d, proof.RecoveryAttemptID) != proof.RepositoryBaseline {
		return ErrInvalidTaskInput
	}
	for _, a := range activities {
		if a != nil && a.TaskID == t.ID && a.AttemptID != nil && *a.AttemptID == origin.ID && a.ActivityType == ActivityRecoveredVerificationArtifacts && a.Author == "coordinator" && VerificationSummaryHash(a.Message) == proof.ArtifactManifestHash {
			var ids []string
			if json.Unmarshal([]byte(a.Message), &ids) == nil && len(ids) > 0 && len(ids) <= 8 {
				return nil
			}
		}
	}
	return ErrInvalidTaskInput
}

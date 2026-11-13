package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const VerificationIntentMarker = "Cuan verification-only intent v1: "

// VerificationIntent permits one immutable-source evidence run, not another
// implementation cycle or publication permission. It lives in its exact launch.
type VerificationIntent struct {
	IsolatedWorkspace      bool   `json:"isolated_workspace,omitempty"`
	FinalizeAttemptID      string `json:"finalize_attempt_id,omitempty"`
	ArtifactManifestHash   string `json:"artifact_manifest_hash,omitempty"`
	RepositoryBaseline     string `json:"repository_baseline,omitempty"`
	RecoveryAttemptID      string `json:"recovery_attempt_id,omitempty"`
	RecoveryCause          string `json:"recovery_cause,omitempty"`
	RuntimeFingerprint     string `json:"runtime_fingerprint,omitempty"`
	SourceAttemptID        string `json:"source_attempt_id"`
	SourceCommit           string `json:"source_commit"`
	InstructionsHash       string `json:"instructions_hash"`
	OwnerID                string `json:"owner_id"`
	RoomID                 string `json:"room_id"`
	SourceRequestID        string `json:"source_request_id"`
	AssessmentRequestID    string `json:"assessment_request_id"`
	SourceVerificationHash string `json:"source_verification_hash"`
}

func VerificationRequestID(taskID, commit string, sourceAttempt ...string) string {
	if len(sourceAttempt) > 0 && sourceAttempt[0] != "" {
		return verificationRequestID("verification-only:v2:"+sourceAttempt[0]+":", taskID, commit)
	}
	return verificationRequestID("verification-only:v1:", taskID, commit)
}
func VerificationRecoveryRequestID(taskID, commit string) string {
	return verificationRequestID("verification-runtime-recovery:v1:", taskID, commit)
}
func VerificationFinalizeRequestID(taskID, commit string) string {
	return verificationRequestID("verification-finalize:v1:", taskID, commit)
}
func verificationRequestID(prefix, taskID, commit string) string {
	hash := sha256.Sum256([]byte(prefix + taskID + ":" + commit))
	hash[6] = (hash[6] & 0x0f) | 0x50
	hash[8] = (hash[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", hash[:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
}
func fullObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func VerificationIntentForLaunch(t *Task, j *TaskLaunch) (*VerificationIntent, error) {
	if j == nil {
		return nil, nil
	}
	at := strings.Index(j.Revision, VerificationIntentMarker)
	if at < 0 {
		if strings.Contains(j.Revision, "Cuan verification-only intent") {
			return nil, ErrInvalidTaskInput
		}
		return nil, nil
	}
	if strings.Count(j.Revision, VerificationIntentMarker) != 1 {
		return nil, ErrInvalidTaskInput
	}
	line := strings.SplitN(j.Revision[at+len(VerificationIntentMarker):], "\n", 2)[0]
	var proof VerificationIntent
	if json.Unmarshal([]byte(line), &proof) != nil || t == nil || !fullObjectID(proof.SourceCommit) || t.SourceRoomID != proof.RoomID || proof.OwnerID == "" || proof.RoomID == "" || proof.SourceRequestID == "" || proof.SourceAttemptID == "" || proof.AssessmentRequestID == "" || len(proof.SourceVerificationHash) != 64 || proof.InstructionsHash != LaunchInstructionsHash(t.Instructions) || j.InstructionsHash != proof.InstructionsHash || j.TaskID != t.ID || j.ProjectID != t.ProjectID || j.SourceRequestID != proof.SourceRequestID || j.OwnerID != proof.OwnerID || j.RoomID != proof.RoomID {
		return nil, ErrInvalidTaskInput
	}
	expected := VerificationRequestID(t.ID, proof.SourceCommit)
	bound := VerificationRequestID(t.ID, proof.SourceCommit, proof.SourceAttemptID)
	if proof.RecoveryAttemptID != "" || proof.RuntimeFingerprint != "" || proof.RecoveryCause != "" {
		if proof.RecoveryAttemptID == "" || proof.RecoveryAttemptID == proof.SourceAttemptID {
			return nil, ErrInvalidTaskInput
		}
		if proof.RecoveryCause == "capacity" || proof.RecoveryCause == "php_preparation" {
			if proof.RuntimeFingerprint != "" || proof.FinalizeAttemptID != "" {
				return nil, ErrInvalidTaskInput
			}
		} else {
			if proof.RecoveryCause != "" || len(proof.RuntimeFingerprint) != 64 {
				return nil, ErrInvalidTaskInput
			}
			if _, err := hex.DecodeString(proof.RuntimeFingerprint); err != nil {
				return nil, ErrInvalidTaskInput
			}
		}
		expected = VerificationRecoveryRequestID(t.ID, proof.SourceCommit)
		bound = expected
	}
	if proof.FinalizeAttemptID != "" || proof.ArtifactManifestHash != "" || proof.RepositoryBaseline != "" {
		if proof.FinalizeAttemptID == "" || proof.FinalizeAttemptID == proof.SourceAttemptID || proof.FinalizeAttemptID == proof.RecoveryAttemptID || proof.RecoveryAttemptID == "" || !validVerificationHash(proof.ArtifactManifestHash) || !validVerificationHash(proof.RepositoryBaseline) {
			return nil, ErrInvalidTaskInput
		}
		expected = VerificationFinalizeRequestID(t.ID, proof.SourceCommit)
		bound = expected
	}
	if j.RequestID != expected && j.RequestID != bound {
		return nil, ErrInvalidTaskInput
	}
	if _, err := hex.DecodeString(proof.SourceVerificationHash); err != nil {
		return nil, ErrInvalidTaskInput
	}
	return &proof, nil
}
func VerificationIntentForAttempt(ctx context.Context, store Store, t *Task, attemptID string) (*VerificationIntent, error) {
	reader, ok := store.(AttemptLaunchStore)
	if !ok {
		return nil, nil
	}
	j, err := reader.LaunchForAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	return VerificationIntentForLaunch(t, j)
}

// ProjectChecksPassed recognizes fixed-command runner output, never generic
// successful/unsupported checks. This also reads legacy records before typed
// project-check outcomes were available.
func ProjectChecksPassed(d *TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != AttemptStatusSucceeded || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil {
		return false
	}
	evidence := AttemptVerificationEvidence(d)
	if evidence.AttemptID != d.LatestAttempt.ID {
		return false
	}
	return evidence.Author != "" && evidence.Author == d.LatestAttempt.RunnerID && PassingProjectCheckSummary(evidence.Current)
}

// VerificationSourceChecksPassed admits a clean, natively tested source whose
// only failed completion gate was publication, after a saved verification review.
// This never changes the failed attempt or establishes deployment/acceptance.
func VerificationSourceChecksPassed(d *TaskDetailResponse) bool {
	return ProjectChecksPassed(d) || (d != nil && IsPublicationReconciliationOutcome(&d.Task, d.LatestAttempt, d.Activities))
}

func PassingProjectCheckSummary(summary string) bool {
	first := strings.SplitN(summary, "\n", 2)[0]
	if !strings.HasPrefix(first, "Project verification: ") || !strings.HasSuffix(first, " (exit 0)") {
		return false
	}
	if strings.HasPrefix(first, "Project verification incomplete:") {
		return false
	}
	return true
}
func VerificationSummaryHash(summary string) string {
	sum := sha256.Sum256([]byte(summary))
	return hex.EncodeToString(sum[:])
}

// ValidateVerificationSource rechecks persisted runner evidence at claim and
// completion. A launch marker alone never establishes that its source passed.
func ValidateVerificationSource(ctx context.Context, store Store, t *Task, attemptID string) (*VerificationIntent, error) {
	proof, err := VerificationIntentForAttempt(ctx, store, t, attemptID)
	if err != nil || proof == nil {
		return proof, err
	}
	if t.CommitSHA != proof.SourceCommit || proof.SourceAttemptID == attemptID {
		return nil, ErrInvalidTaskInput
	}
	source, err := store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil {
		return nil, err
	}
	if source == nil || source.TaskID != t.ID {
		return nil, ErrInvalidTaskInput
	}
	activities, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	d := &TaskDetailResponse{Task: *t, LatestAttempt: source, Activities: activities}
	if !VerificationSourceChecksPassed(d) || VerificationSummaryHash(AttemptVerificationEvidence(d).Current) != proof.SourceVerificationHash {
		return nil, ErrInvalidTaskInput
	}
	if proof.RecoveryAttemptID != "" {
		if err := validateVerificationRuntimeRecovery(ctx, store, t, proof, activities); err != nil {
			return nil, err
		}
	}
	if proof.FinalizeAttemptID != "" {
		if err := validateVerificationFinalization(ctx, store, t, proof, activities); err != nil {
			return nil, err
		}
	}
	return proof, nil
}

package task

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
)

const NativeReleaseIntentMarker = "Cuan native-release intent v1: "
const ActivityNativeReleaseHandoff = "native_release_handoff"
const NativeReleaseMergeConflictMessage = "Native release source has confirmed merge conflicts with the current release branch. Reconcile the approved source against the latest branch in a developer session, preserve unrelated work and release gates, rerun required checks and independent source review, then publish only the new reviewed source. The release script was not started."

// NativeReleaseHandoff is produced by completion validation, never by agent
// reports. It records unfinished publication, not a successful deployment.
type NativeReleaseHandoff struct {
	Version          int    `json:"version"`
	TaskID           string `json:"task_id"`
	AttemptID        string `json:"attempt_id"`
	SourceCommit     string `json:"source_commit"`
	InstructionsHash string `json:"instructions_hash"`
	VerificationHash string `json:"verification_hash"`
	Repository       string `json:"repository"`
	TargetBranch     string `json:"target_branch"`
	ManifestHash     string `json:"manifest_hash"`
}

// NativeReleaseIntent binds one durable release stage to a verified source
// handoff. Parsing this value grants neither original authorization nor access.
type NativeReleaseIntent struct {
	SourceReviewAttemptID  string `json:"source_review_attempt_id,omitempty"`
	SourceAttemptID        string `json:"source_attempt_id"`
	SourceCommit           string `json:"source_commit"`
	SourceVerificationHash string `json:"source_verification_hash"`
	InstructionsHash       string `json:"instructions_hash"`
	OwnerID                string `json:"owner_id"`
	RoomID                 string `json:"room_id"`
	SourceRequestID        string `json:"source_request_id"`
	Repository             string `json:"repository"`
	TargetBranch           string `json:"target_branch"`
	ManifestHash           string `json:"manifest_hash"`
}

func NativeReleaseRequestID(taskID, source string) string {
	return verificationRequestID("native-release:v2:", taskID, source)
}

func NativeReleaseIntentForLaunch(t *Task, j *TaskLaunch) (*NativeReleaseIntent, error) {
	if j == nil {
		return nil, nil
	}
	at := strings.Index(j.Revision, NativeReleaseIntentMarker)
	if at < 0 {
		if strings.Contains(j.Revision, "Cuan native-release intent") {
			return nil, ErrInvalidTaskInput
		}
		return nil, nil
	}
	if strings.Count(j.Revision, NativeReleaseIntentMarker) != 1 {
		return nil, ErrInvalidTaskInput
	}
	if strings.Contains(j.Revision, VerificationIntentMarker) || strings.Contains(j.Revision, LiveInspectionMarker) {
		return nil, ErrInvalidTaskInput
	}
	line := strings.SplitN(j.Revision[at+len(NativeReleaseIntentMarker):], "\n", 2)[0]
	var p NativeReleaseIntent
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&p) != nil || decoder.Decode(&struct{}{}) != io.EOF || !nativeReleaseShape(t, &p) || j.InstructionsHash != p.InstructionsHash || j.TaskID != t.ID || j.ProjectID != t.ProjectID || j.OwnerID != p.OwnerID || j.RoomID != p.RoomID || j.SourceRequestID != p.SourceRequestID {
		return nil, ErrInvalidTaskInput
	}
	legacyRequest := verificationRequestID("native-release:v1:", t.ID, p.SourceCommit)
	if j.RequestID != NativeReleaseRequestID(t.ID, p.SourceCommit) && !(p.SourceReviewAttemptID == "" && j.RequestID == legacyRequest) {
		return nil, ErrInvalidTaskInput
	}
	return &p, nil
}

func nativeReleaseShape(t *Task, p *NativeReleaseIntent) bool {
	if t == nil || p == nil || !fullObjectID(p.SourceCommit) || p.SourceAttemptID == "" || p.OwnerID == "" || p.RoomID == "" || p.SourceRequestID == "" || p.TargetBranch == "" || strings.ContainsAny(p.TargetBranch, "\r\n\x00") || strings.HasPrefix(p.TargetBranch, "-") || strings.Count(p.Repository, "/") != 1 || strings.HasPrefix(p.Repository, "/") || strings.HasSuffix(p.Repository, "/") || strings.ContainsAny(p.Repository, " \t\r\n?#") || t.SourceRoomID != p.RoomID || p.InstructionsHash != LaunchInstructionsHash(t.Instructions) {
		return false
	}
	for _, hash := range []string{p.SourceVerificationHash, p.ManifestHash} {
		if len(hash) != 64 {
			return false
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return false
		}
	}
	return true
}

func NativeReleaseIntentForAttempt(ctx context.Context, store Store, t *Task, attemptID string) (*NativeReleaseIntent, error) {
	reader, ok := store.(AttemptLaunchStore)
	if !ok {
		return nil, nil
	}
	j, err := reader.LaunchForAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	return NativeReleaseIntentForLaunch(t, j)
}

// ValidateNativeReleaseSource requires current native evidence. Historical
// failures and model prose cannot be reinterpreted as a fresh publication grant.
func ValidateNativeReleaseSource(ctx context.Context, store Store, t *Task, p *NativeReleaseIntent) error {
	return validateNativeReleaseSource(ctx, store, t, p, true)
}

// ValidateNativeDeveloperHandoff admits only read-only source verification. It
// cannot authorize publication. A terminal observation can retain the exact
// developer source for read-only verification without replaying development.
func ValidateNativeDeveloperHandoff(ctx context.Context, store Store, t *Task, p *NativeReleaseIntent) error {
	if t == nil || p == nil || (t.Status != TaskStatusReview && t.Status != TaskStatusBlocked) || p.SourceReviewAttemptID != "" {
		return ErrInvalidTaskInput
	}
	latest, err := store.GetLatestAttempt(ctx, t.ID)
	if err != nil {
		return err
	}
	if latest == nil {
		return ErrRequestConflict
	}
	if latest.ID != p.SourceAttemptID {
		// An older dispatcher may have inspected the current release instead of
		// this handoff after a verifier-ID collision. Its terminal observation is
		// not implementation work or source approval. Recover only its exact
		// retained developer source into a fresh read-only source verification.
		if latest.CancelRequested || latest.CanceledAt != nil || latest.FinishedAt == nil || (latest.Status != AttemptStatusSucceeded && latest.Status != AttemptStatusFailed) {
			return ErrRequestConflict
		}
		inspection, err := LiveInspectionForAttempt(ctx, store, t, latest.ID)
		if err != nil || inspection == nil || inspection.Purpose != CurrentOutcomeInspection || inspection.SourceAttemptID != p.SourceAttemptID || inspection.SourceCommit != p.SourceCommit || inspection.OwnerID != p.OwnerID || inspection.RoomID != p.RoomID || inspection.SourceRequestID != p.SourceRequestID || inspection.Repository != p.Repository || inspection.SourceVerifierAttemptID != "" || inspection.PreparedPublicationAttemptID != "" || ValidateLiveInspectionSource(ctx, store, t, inspection) != nil {
			return ErrRequestConflict
		}
	} else if t.Status != TaskStatusReview {
		return ErrInvalidTaskInput
	}
	return validateNativeReleaseSource(ctx, store, t, p, false)
}

// Historical delivery validation checks the same source evidence without
// pretending its completed native stage is still queued/running. This grants
// no new execution admission; current-stage validation retains the latest gate.
func validateNativeReleaseSource(ctx context.Context, store Store, t *Task, p *NativeReleaseIntent, requireLatest bool) error {
	if !nativeReleaseShape(t, p) || t.CommitSHA != p.SourceCommit || (requireLatest && t.Status == TaskStatusDone) || t.Status == "cancelled" || t.Status == "canceled" || t.ExecutionCategory != ExecutionCategoryCode {
		return ErrInvalidTaskInput
	}
	// Legacy records remain readable as history, never as fresh release grants.
	if requireLatest && p.SourceReviewAttemptID == "" {
		return ErrInvalidTaskInput
	}
	if p.SourceReviewAttemptID != "" {
		review, err := NativeSourceReviewForAttempt(ctx, store, t, p.SourceReviewAttemptID)
		if err != nil || review == nil || review.SourceAttemptID != p.SourceAttemptID || review.SourceCommit != p.SourceCommit || review.InstructionsHash != p.InstructionsHash {
			return ErrInvalidTaskInput
		}
		origin, err := VerificationIntentForAttempt(ctx, store, t, p.SourceReviewAttemptID)
		if err != nil || origin == nil || origin.OwnerID != p.OwnerID || origin.RoomID != p.RoomID || origin.SourceRequestID != p.SourceRequestID {
			return ErrInvalidTaskInput
		}
	}
	source, err := store.GetAttempt(ctx, p.SourceAttemptID)
	if err != nil {
		return err
	}
	latest, err := store.GetLatestAttempt(ctx, t.ID)
	if err != nil {
		return err
	}
	if latest == nil {
		return ErrInvalidTaskInput
	}
	if requireLatest && latest.ID != p.SourceReviewAttemptID {
		if latest.CancelRequested || latest.CanceledAt != nil || (latest.Status != AttemptStatusQueued && latest.Status != AttemptStatusRunning) {
			return ErrRequestConflict
		}
		current, err := NativeReleaseIntentForAttempt(ctx, store, t, latest.ID)
		if err != nil || current == nil || *current != *p {
			return ErrRequestConflict
		}
	}
	if source == nil || source.TaskID != t.ID || source.Status != AttemptStatusSucceeded || source.ExitCode == nil || *source.ExitCode != 0 || source.FinishedAt == nil || source.StartedAt == nil || source.RunnerID == "" || source.CancelRequested || source.CanceledAt != nil || source.CreatedAt.IsZero() || source.StartedAt.Before(source.CreatedAt) || source.FinishedAt.Before(*source.StartedAt) || source.FinishedAt.After(time.Now()) {
		return ErrInvalidTaskInput
	}
	limits, err := AttemptPublicationPolicy(ctx, store, t, source.ID)
	if err != nil {
		return err
	}
	if limits.DenyPush || limits.DenyDeploy || limits.ReadOnlyRepository {
		return ErrInvalidTaskInput
	}
	if executionpolicy.StandaloneArtifactWork(t.Instructions) {
		return ErrInvalidTaskInput
	}
	activities, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return err
	}
	d := &TaskDetailResponse{Task: *t, LatestAttempt: source, Activities: activities}
	if !ProjectChecksPassed(d) || VerificationSummaryHash(AttemptVerificationEvidence(d).Current) != p.SourceVerificationHash || !liveImplementationFinalReport(activities, t.ID, source.ID) {
		return ErrInvalidTaskInput
	}
	changes := AttemptChangeEvidence(d)
	if changes.Provenance != "current_runner_branch_diff" || changes.RunnerChangeRecord == nil || changes.ResultCommit != p.SourceCommit || strings.TrimSpace(changes.Summary) == "" {
		return ErrInvalidTaskInput
	}
	for _, event := range activities {
		if event == nil || event.ID != "native-release-handoff:"+source.ID || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != source.ID || event.ActivityType != ActivityNativeReleaseHandoff || event.Author != source.RunnerID || event.CreatedAt.Before(*source.StartedAt) || event.CreatedAt.After(*source.FinishedAt) {
			continue
		}
		var receipt NativeReleaseHandoff
		if json.Unmarshal([]byte(event.Message), &receipt) == nil && receipt.Version == 1 && receipt.TaskID == t.ID && receipt.AttemptID == source.ID && receipt.SourceCommit == p.SourceCommit && receipt.InstructionsHash == p.InstructionsHash && receipt.VerificationHash == p.SourceVerificationHash && receipt.Repository == p.Repository && receipt.TargetBranch == p.TargetBranch && receipt.ManifestHash == p.ManifestHash {
			return nil
		}
	}
	return ErrInvalidTaskInput
}

package task

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
)

const LiveInspectionMarker = "Cuan live-result inspection v1: "
const ActivityLiveInspection = "live_result_verified"
const CurrentOutcomeInspection = "current_outcome"
const NativeReleaseInspection = "native_release"
const EvidenceRevalidation = "evidence"

type LiveInspectionIntent struct {
	SourceVerifierAttemptID   string    `json:"source_verifier_attempt_id,omitempty"`
	RevalidationKind          string    `json:"revalidation_kind,omitempty"`
	VerifierRecoveryAttemptID string    `json:"verifier_recovery_attempt_id,omitempty"`
	PreparationLaunchID       string    `json:"preparation_launch_id,omitempty"`
	ReviewRequestID           string    `json:"review_request_id,omitempty"`
	ReviewPaths               []string  `json:"review_paths,omitempty"`
	Purpose                   string    `json:"purpose,omitempty"`
	PreviousAttemptID         string    `json:"previous_attempt_id,omitempty"`
	PreviousCommit            string    `json:"previous_commit,omitempty"`
	SourceAttemptID           string    `json:"source_attempt_id"`
	SourceCommit              string    `json:"source_commit"`
	TargetCommit              string    `json:"target_commit"`
	Repository                string    `json:"repository"`
	OwnerID                   string    `json:"owner_id"`
	RoomID                    string    `json:"room_id"`
	SourceRequestID           string    `json:"source_request_id"`
	InstructionsHash          string    `json:"instructions_hash"`
	ObservedAt                time.Time `json:"observed_at"`
}

type LiveInspectionReceipt struct {
	Purpose           string `json:"purpose,omitempty"`
	SourceAttemptID   string `json:"source_attempt_id"`
	SourceCommit      string `json:"source_commit"`
	TargetCommit      string `json:"target_commit"`
	Repository        string `json:"repository"`
	ProjectChecksHash string `json:"project_checks_hash"`
}

func LiveInspectionRequestID(taskID, sourceCommit, instructionsHash string) string {
	return verificationRequestID("live-result-inspection:v1:"+instructionsHash+":", taskID, sourceCommit)
}

func (p *LiveInspectionIntent) RequestID(taskID string) string {
	if p.Purpose == NativeReleaseInspection {
		return verificationRequestID("native-release-inspection:v1:"+p.InstructionsHash+":", taskID, p.SourceAttemptID)
	}
	if p.VerifierRecoveryAttemptID != "" {
		return verificationRequestID("current-outcome-verifier-repair:v1:"+p.InstructionsHash+":", taskID, p.SourceAttemptID)
	}
	if p.PreparationLaunchID != "" {
		return verificationRequestID("live-inspection-preparation:v1:"+p.InstructionsHash+":", taskID, p.PreparationLaunchID)
	}
	if p.Purpose == CurrentOutcomeInspection {
		if p.PreviousAttemptID != "" {
			return verificationRequestID("current-outcome-revalidation:v1:"+p.InstructionsHash+":", taskID, p.SourceAttemptID)
		}
		return verificationRequestID("current-outcome:v1:"+p.InstructionsHash+":", taskID, p.SourceAttemptID)
	}
	if p.PreviousAttemptID != "" {
		return verificationRequestID("live-result-inspection-recovery:v1:"+p.InstructionsHash+":", taskID, p.SourceCommit)
	}
	return LiveInspectionRequestID(taskID, p.SourceCommit, p.InstructionsHash)
}

// A current-outcome inspection establishes current behavior, not authorship or
// ancestry of historical work, including a branch with unverified publication.
func (p *LiveInspectionIntent) SnapshotSourceCommit() string {
	if p.Purpose == CurrentOutcomeInspection {
		if p.PreviousAttemptID != "" {
			return p.PreviousCommit
		}
		return p.TargetCommit
	}
	return p.SourceCommit
}

func IsExecutionTimeout(a *TaskAttempt) bool {
	if a == nil || a.Status != AttemptStatusFailed || a.ExitCode == nil {
		return false
	}
	return (*a.ExitCode == 124 && strings.HasPrefix(a.ErrorMessage, "Execution timed out after adaptive timeout (")) || (*a.ExitCode == 1 && strings.HasPrefix(a.ErrorMessage, "Task runner exceeded hard execution timeout ("))
}

// Legacy local runners sometimes retained exit 124 and a terminal interval but
// lost the reason. This permits inspection of today's outcome, not a timeout
// diagnosis, successful historical work, or another implementation execution.
func IsOpaqueLocalFailure(a *TaskAttempt) bool {
	return a != nil && a.RunnerID == "local_terminal" && a.Status == AttemptStatusFailed &&
		a.ExitCode != nil && *a.ExitCode == 124 && a.ErrorMessage == "" &&
		a.StartedAt != nil && a.FinishedAt != nil && a.FinishedAt.After(*a.StartedAt) &&
		!a.CancelRequested && a.CanceledAt == nil
}

// Some historical local attempts have a terminal Failed state but no exit or
// finish receipt. Their cause and stop time remain unknown. This shape can only
// authorize the existing bounded, read-only current-outcome inspection, never a
// replay of the old process or a repair of its missing termination metadata.
func IsUnrecordedLocalFailure(a *TaskAttempt) bool {
	return a != nil && a.RunnerID == "local_terminal" && a.Status == AttemptStatusFailed &&
		a.ExitCode == nil && a.FinishedAt == nil && a.ErrorMessage == "" &&
		!a.CreatedAt.IsZero() && a.StartedAt != nil && !a.StartedAt.IsZero() &&
		!a.CancelRequested && a.CanceledAt == nil
}

// A project-check failure additionally requires the exact attempt's final
// implementation report. Opaque local failures preserve their unknown cause.
func IsCurrentOutcomeFailure(a *TaskAttempt) bool {
	return IsExecutionTimeout(a) || IsOpaqueLocalFailure(a) || IsUnrecordedLocalFailure(a) || (a != nil && a.Status == AttemptStatusFailed && a.ExitCode != nil && *a.ExitCode == 0 && a.ErrorMessage == "Project verification failed")
}

// A corrected legacy acceptance can leave a successful, unpublished branch even
// though another running revision may already satisfy the original request.
// This permits one independent outcome inspection, never credit for publishing
// that branch, source ancestry, or an implementation replay.
func IsUnpublishedLegacyOutcome(t *Task, a *TaskAttempt, activities []*TaskActivity) bool {
	return unpublishedOutcome(t, a, activities, false)
}

// A completed implementation rejected solely for verification can inspect the
// current outcome once, without gaining another implementation or deployment.
func IsUnpublishedReviewedOutcome(t *Task, a *TaskAttempt, activities []*TaskActivity) bool {
	return unpublishedOutcome(t, a, activities, true)
}

func unpublishedOutcome(t *Task, a *TaskAttempt, activities []*TaskActivity, reviewed bool) bool {
	if t == nil || a == nil || a.TaskID != t.ID || !fullObjectID(t.CommitSHA) || (t.ExecutionCategory != "" && t.ExecutionCategory != ExecutionCategoryCode) || a.Status != AttemptStatusSucceeded || a.ExitCode == nil || *a.ExitCode != 0 || a.ErrorMessage != "" || a.CancelRequested || a.CanceledAt != nil || a.CreatedAt.IsZero() || a.StartedAt == nil || a.StartedAt.IsZero() || a.FinishedAt == nil || a.FinishedAt.Before(*a.StartedAt) || a.StartedAt.Before(a.CreatedAt) {
		return false
	}
	limits := executionpolicy.FromInstructions(t.Instructions)
	if !strings.HasPrefix(t.Instructions, "Cuan managed delegation v1.\n") || (!reviewed && executionpolicy.NormalizeManagedInstructions(t.Instructions) == t.Instructions) || limits.DenyDeploy || limits.ReadOnlyRepository || !liveImplementationFinalReport(activities, t.ID, a.ID) || VerifiedDeployment(activities, a, t.CommitSHA, time.Now().UTC()) {
		return false
	}
	for _, event := range activities {
		if reviewed && event != nil && event.TaskID == t.ID && event.AttemptID != nil && *event.AttemptID == a.ID && event.ActivityType == "coordinator_review_assessment" && event.Author == "coordinator" && !event.CreatedAt.Before(*a.FinishedAt) && !event.CreatedAt.After(time.Now().UTC()) {
			var record struct {
				RequestID        string `json:"request_id"`
				InstructionsHash string `json:"instructions_hash"`
				EvidenceDigest   string `json:"evidence_digest"`
				Assessment       struct {
					Decision      string `json:"decision"`
					RemainingWork string `json:"remaining_work"`
				} `json:"assessment"`
			}
			if json.Unmarshal([]byte(event.Message), &record) == nil && record.RequestID != "" && record.InstructionsHash == LaunchInstructionsHash(t.Instructions) && len(record.EvidenceDigest) == 64 && record.Assessment.Decision == "revise" && record.Assessment.RemainingWork == "verification" {
				return true
			}
		}
		if !reviewed && event != nil && event.TaskID == t.ID && event.AttemptID != nil && *event.AttemptID == a.ID && event.ActivityType == ActivityPublicationAcceptanceAudit && event.Author == "coordinator" && event.NewStatus == "review_required" && !event.CreatedAt.Before(*a.FinishedAt) && !event.CreatedAt.After(time.Now().UTC()) {
			return true
		}
	}
	return false
}

func (p *LiveInspectionIntent) PredecessorAttemptID() string {
	if p.VerifierRecoveryAttemptID != "" {
		return p.VerifierRecoveryAttemptID
	}
	if p.PreviousAttemptID != "" {
		return p.PreviousAttemptID
	}
	if p.SourceVerifierAttemptID != "" {
		return p.SourceVerifierAttemptID
	}
	return p.SourceAttemptID
}

var liveRepositoryIdentity = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*/[a-z0-9][a-z0-9_.-]*$`)

// Codes contain no caller data and preserve errors.Is(ErrInvalidTaskInput).
type liveInspectionValidationError string

func (e liveInspectionValidationError) Error() string { return "invalid live inspection: " + string(e) }
func (e liveInspectionValidationError) Unwrap() error { return ErrInvalidTaskInput }

func LiveInspectionForLaunch(t *Task, launch *TaskLaunch) (*LiveInspectionIntent, error) {
	invalid := func(code string) (*LiveInspectionIntent, error) {
		return nil, liveInspectionValidationError("live_inspection_" + code)
	}
	if launch == nil {
		return nil, nil
	}
	at := strings.Index(launch.Revision, LiveInspectionMarker)
	if at < 0 {
		if strings.Contains(launch.Revision, "Cuan live-result inspection") {
			return invalid("missing_marker")
		}
		return nil, nil
	}
	var proof LiveInspectionIntent
	line := strings.SplitN(launch.Revision[at+len(LiveInspectionMarker):], "\n", 2)[0]
	if json.Unmarshal([]byte(line), &proof) != nil {
		return invalid("malformed_intent")
	}
	if proof.RevalidationKind != "" && (proof.RevalidationKind != EvidenceRevalidation || proof.Purpose != CurrentOutcomeInspection || proof.PreviousAttemptID == "" || proof.ReviewRequestID == "" || len(proof.ReviewPaths) != 0) {
		return invalid("revalidation_kind")
	}
	if proof.VerifierRecoveryAttemptID != "" && (proof.Purpose != CurrentOutcomeInspection || proof.PreparationLaunchID != "" || proof.PreviousAttemptID != "" || proof.ReviewRequestID != "" || len(proof.ReviewPaths) != 0 || proof.VerifierRecoveryAttemptID == proof.SourceAttemptID) {
		return invalid("recovery_shape")
	}
	if proof.Purpose != CurrentOutcomeInspection && (proof.ReviewRequestID != "" || len(proof.ReviewPaths) > 0) {
		return invalid("purpose_shape")
	}
	if proof.PreparationLaunchID != "" && (proof.PreviousAttemptID != "" || proof.ReviewRequestID != "" || len(proof.ReviewPaths) != 0) {
		return invalid("preparation_shape")
	}
	if proof.Purpose == NativeReleaseInspection && (proof.SourceVerifierAttemptID != "" || proof.VerifierRecoveryAttemptID != "" || proof.PreparationLaunchID != "" || proof.PreviousAttemptID != "" || proof.PreviousCommit != "" || proof.RevalidationKind != "") {
		return invalid("native_release_shape")
	}
	validSource := proof.Purpose == "" && fullObjectID(proof.SourceCommit) && proof.SourceCommit != proof.TargetCommit
	if proof.Purpose == NativeReleaseInspection {
		validSource = fullObjectID(proof.SourceCommit)
	}
	if proof.Purpose == CurrentOutcomeInspection {
		validSource = (proof.SourceCommit == "" || fullObjectID(proof.SourceCommit)) && ((proof.PreviousAttemptID == "" && proof.PreviousCommit == "" && proof.ReviewRequestID == "" && len(proof.ReviewPaths) == 0) || (proof.PreviousAttemptID != "" && proof.ReviewRequestID != "" && ((len(proof.ReviewPaths) > 0 && len(proof.ReviewPaths) <= 16) || proof.RevalidationKind == EvidenceRevalidation)))
	}
	if t == nil {
		return invalid("missing_task")
	}
	if strings.Count(launch.Revision, LiveInspectionMarker) != 1 || strings.Contains(launch.Revision, VerificationIntentMarker) {
		return invalid("conflicting_intent")
	}
	if !validSource || !fullObjectID(proof.TargetCommit) || !liveRepositoryIdentity.MatchString(proof.Repository) || proof.SourceAttemptID == "" {
		return invalid("source_identity")
	}
	if proof.OwnerID == "" || proof.RoomID == "" || proof.SourceRequestID == "" {
		return invalid("missing_authorization_identity")
	}
	if proof.ObservedAt.IsZero() || proof.ObservedAt.After(launch.CreatedAt.Add(5*time.Second)) {
		return invalid("observation_time")
	}
	if proof.InstructionsHash != LaunchInstructionsHash(t.Instructions) || launch.InstructionsHash != proof.InstructionsHash {
		return invalid("instructions_changed")
	}
	if launch.TaskID != t.ID || launch.ProjectID != t.ProjectID {
		return invalid("task_project_binding")
	}
	if launch.OwnerID != proof.OwnerID || launch.RoomID != proof.RoomID || t.SourceRoomID != proof.RoomID || launch.SourceRequestID != proof.SourceRequestID {
		return invalid("authorization_binding")
	}
	if launch.RequestID != proof.RequestID(t.ID) {
		return invalid("request_binding")
	}
	if (proof.PreviousAttemptID == "" && proof.PreviousCommit != "") || (proof.PreviousAttemptID != "" && (!fullObjectID(proof.PreviousCommit) || (proof.PreviousCommit == proof.TargetCommit && proof.RevalidationKind != EvidenceRevalidation))) {
		return invalid("previous_revision")
	}
	return &proof, nil
}

func LiveInspectionForAttempt(ctx context.Context, store Store, t *Task, attemptID string) (*LiveInspectionIntent, error) {
	reader, ok := store.(AttemptLaunchStore)
	if !ok {
		return nil, nil
	}
	launch, err := reader.LaunchForAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	return LiveInspectionForLaunch(t, launch)
}

// A fresh inspection does not turn the source's failed checks into a pass.
// Current ancestry, tests and unchanged-scope evidence must be supplied later.
func ValidateLiveInspection(ctx context.Context, store Store, t *Task, attemptID string) (*LiveInspectionIntent, error) {
	proof, err := LiveInspectionForAttempt(ctx, store, t, attemptID)
	if err != nil || proof == nil {
		return proof, err
	}
	if t.CommitSHA != proof.SourceCommit || attemptID == proof.SourceAttemptID {
		return nil, ErrInvalidTaskInput
	}
	if err := ValidateLiveInspectionSource(ctx, store, t, proof); err != nil {
		return nil, err
	}
	return proof, nil
}

func ValidateLiveInspectionSource(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	if t == nil || proof == nil || (proof.Purpose != "" && proof.Purpose != CurrentOutcomeInspection && proof.Purpose != NativeReleaseInspection) || t.CommitSHA != proof.SourceCommit || proof.InstructionsHash != LaunchInstructionsHash(t.Instructions) {
		return ErrInvalidTaskInput
	}
	if proof.Purpose == NativeReleaseInspection {
		return validateNativeReleaseInspectionSource(ctx, store, t, proof)
	}
	if proof.SourceVerifierAttemptID != "" {
		if err := validateInspectionSourceVerifier(ctx, store, t, proof); err != nil {
			return err
		}
	}
	if proof.VerifierRecoveryAttemptID != "" {
		if err := ValidateInspectionVerifierRecovery(ctx, store, t, proof); err != nil {
			return err
		}
	}
	if proof.PreparationLaunchID != "" {
		if err := ValidateInspectionPreparationRecovery(ctx, store, t, proof); err != nil {
			return err
		}
	}
	source, err := store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil {
		return err
	}
	if proof.Purpose == CurrentOutcomeInspection {
		// All admission/revalidation routes share this source check. A local
		// implementation cannot be verified by inspecting a deployed release.
		limits, err := AttemptPublicationPolicy(ctx, store, t, proof.SourceAttemptID)
		if err != nil {
			return err
		}
		if limits.DenyPush && limits.DenyDeploy {
			return ErrInvalidTaskInput
		}
		if source == nil || source.TaskID != t.ID || source.CancelRequested || source.CanceledAt != nil {
			return ErrInvalidTaskInput
		}
		if proof.SourceCommit != "" {
			activities, err := store.ListActivities(ctx, t.ID)
			if err != nil {
				return err
			}
			if (proof.PreviousAttemptID == "" && (proof.ReviewRequestID != "" || len(proof.ReviewPaths) > 0)) || (!IsUnpublishedLegacyOutcome(t, source, activities) && !IsUnpublishedReviewedOutcome(t, source, activities) && !IsPublicationReconciliationOutcome(t, source, activities)) {
				return ErrInvalidTaskInput
			}
		} else if !IsCurrentOutcomeFailure(source) {
			return ErrInvalidTaskInput
		}
		if source.FinishedAt == nil {
			if !IsUnrecordedLocalFailure(source) || proof.ObservedAt.Before(source.CreatedAt) || proof.ObservedAt.Before(*source.StartedAt) {
				return ErrInvalidTaskInput
			}
		} else if proof.ObservedAt.Before(*source.FinishedAt) {
			return ErrInvalidTaskInput
		}
		prior, err := LiveInspectionForAttempt(ctx, store, t, source.ID)
		if err != nil || prior != nil {
			return ErrInvalidTaskInput
		}
		if !IsExecutionTimeout(source) && !IsOpaqueLocalFailure(source) && !IsUnrecordedLocalFailure(source) {
			activities, err := store.ListActivities(ctx, t.ID)
			if err != nil {
				return err
			}
			if !liveImplementationFinalReport(activities, t.ID, source.ID) {
				return ErrInvalidTaskInput
			}
		}
		if proof.PreviousAttemptID != "" {
			return validateOutcomeRevalidation(ctx, store, t, proof)
		}
		if proof.ReviewRequestID != "" || len(proof.ReviewPaths) > 0 {
			return ErrInvalidTaskInput
		}
		return nil
	}
	if source == nil || source.TaskID != t.ID || source.Status != AttemptStatusFailed || source.ExitCode == nil || *source.ExitCode != 0 || source.ErrorMessage != "Project verification failed" || source.CancelRequested || source.CanceledAt != nil || source.FinishedAt == nil || proof.ObservedAt.Before(*source.FinishedAt) {
		return ErrInvalidTaskInput
	}
	prior, err := LiveInspectionForAttempt(ctx, store, t, source.ID)
	if err != nil || prior != nil {
		return ErrInvalidTaskInput
	}
	activities, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return err
	}
	if !liveImplementationFinalReport(activities, t.ID, source.ID) {
		return ErrInvalidTaskInput
	}
	if proof.PreviousAttemptID != "" {
		previous, err := store.GetAttempt(ctx, proof.PreviousAttemptID)
		if err != nil || previous == nil || previous.TaskID != t.ID || previous.Status != AttemptStatusFailed || previous.ExitCode == nil || *previous.ExitCode != 0 || previous.ErrorMessage != "Live result inspection project checks did not pass" || previous.CancelRequested || previous.CanceledAt != nil || previous.FinishedAt == nil || proof.ObservedAt.Before(*previous.FinishedAt) {
			return ErrInvalidTaskInput
		}
		binding, err := LiveInspectionForAttempt(ctx, store, t, previous.ID)
		if err != nil || binding == nil || binding.PreviousAttemptID != "" || binding.SourceVerifierAttemptID != proof.SourceVerifierAttemptID || binding.SourceAttemptID != proof.SourceAttemptID || binding.SourceCommit != proof.SourceCommit || binding.TargetCommit != proof.PreviousCommit || binding.Repository != proof.Repository || binding.OwnerID != proof.OwnerID || binding.RoomID != proof.RoomID || binding.SourceRequestID != proof.SourceRequestID || binding.InstructionsHash != proof.InstructionsHash {
			return ErrInvalidTaskInput
		}
		var report, checks bool
		for _, event := range activities {
			if event == nil || event.AttemptID == nil || *event.AttemptID != previous.ID || event.TaskID != t.ID {
				continue
			}
			if event.ActivityType == "agent_report" && (event.NewStatus == TaskStatusDone || event.NewStatus == TaskStatusReview) {
				report = true
			}
			if event.ActivityType == ActivityTypeRunnerVerification && event.ID == "verification:"+previous.ID && event.Author == previous.RunnerID {
				var saved struct {
					Summary string `json:"summary"`
				}
				if json.Unmarshal([]byte(event.Message), &saved) == nil && strings.HasPrefix(saved.Summary, "Project verification:") && !PassingProjectCheckSummary(saved.Summary) {
					checks = true
				}
			}
		}
		if !report || !checks {
			return ErrInvalidTaskInput
		}
	}
	return nil
}

func liveImplementationFinalReport(activities []*TaskActivity, taskID, attemptID string) bool {
	var latest, kind *TaskActivity
	for _, event := range activities {
		if event != nil && event.TaskID == taskID && event.AttemptID != nil && *event.AttemptID == attemptID && event.ActivityType == "agent_result_kind" && (kind == nil || event.CreatedAt.After(kind.CreatedAt) || (event.CreatedAt.Equal(kind.CreatedAt) && event.ID > kind.ID)) {
			kind = event
		}
		if event != nil && event.TaskID == taskID && event.AttemptID != nil && *event.AttemptID == attemptID && event.ActivityType == "agent_report" && (latest == nil || event.CreatedAt.After(latest.CreatedAt) || (event.CreatedAt.Equal(latest.CreatedAt) && event.ID > latest.ID)) {
			latest = event
		}
	}
	return latest != nil && (latest.NewStatus == TaskStatusReview || latest.NewStatus == TaskStatusDone) && kind != nil && kind.NewStatus == "implementation"
}

// ImplementationFinalReport checks the latest saved report and result kind for
// this exact attempt. It is evidence of a handoff, not independent acceptance.
func ImplementationFinalReport(activities []*TaskActivity, taskID, attemptID string) bool {
	return liveImplementationFinalReport(activities, taskID, attemptID)
}

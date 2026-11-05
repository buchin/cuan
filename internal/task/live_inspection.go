package task

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

const LiveInspectionMarker = "Cuan live-result inspection v1: "
const ActivityLiveInspection = "live_result_verified"
const CurrentOutcomeInspection = "current_outcome"

type LiveInspectionIntent struct {
	Purpose           string    `json:"purpose,omitempty"`
	PreviousAttemptID string    `json:"previous_attempt_id,omitempty"`
	PreviousCommit    string    `json:"previous_commit,omitempty"`
	SourceAttemptID   string    `json:"source_attempt_id"`
	SourceCommit      string    `json:"source_commit"`
	TargetCommit      string    `json:"target_commit"`
	Repository        string    `json:"repository"`
	OwnerID           string    `json:"owner_id"`
	RoomID            string    `json:"room_id"`
	SourceRequestID   string    `json:"source_request_id"`
	InstructionsHash  string    `json:"instructions_hash"`
	ObservedAt        time.Time `json:"observed_at"`
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
	if p.Purpose == CurrentOutcomeInspection {
		return verificationRequestID("current-outcome:v1:"+p.InstructionsHash+":", taskID, p.SourceAttemptID)
	}
	if p.PreviousAttemptID != "" {
		return verificationRequestID("live-result-inspection-recovery:v1:"+p.InstructionsHash+":", taskID, p.SourceCommit)
	}
	return LiveInspectionRequestID(taskID, p.SourceCommit, p.InstructionsHash)
}

// A current-outcome inspection establishes current behavior, not authorship or
// ancestry of an implementation that never produced a source commit.
func (p *LiveInspectionIntent) SnapshotSourceCommit() string {
	if p.Purpose == CurrentOutcomeInspection {
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

// IsCurrentOutcomeFailure only recognizes native failure shapes. A project-check
// failure additionally requires the exact attempt's final implementation report.
func IsCurrentOutcomeFailure(a *TaskAttempt) bool {
	return IsExecutionTimeout(a) || (a != nil && a.Status == AttemptStatusFailed && a.ExitCode != nil && *a.ExitCode == 0 && a.ErrorMessage == "Project verification failed")
}

func (p *LiveInspectionIntent) PredecessorAttemptID() string {
	if p.PreviousAttemptID != "" {
		return p.PreviousAttemptID
	}
	return p.SourceAttemptID
}

var liveRepositoryIdentity = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*/[a-z0-9][a-z0-9_.-]*$`)

func LiveInspectionForLaunch(t *Task, launch *TaskLaunch) (*LiveInspectionIntent, error) {
	if launch == nil {
		return nil, nil
	}
	at := strings.Index(launch.Revision, LiveInspectionMarker)
	if at < 0 {
		if strings.Contains(launch.Revision, "Cuan live-result inspection") {
			return nil, ErrInvalidTaskInput
		}
		return nil, nil
	}
	var proof LiveInspectionIntent
	line := strings.SplitN(launch.Revision[at+len(LiveInspectionMarker):], "\n", 2)[0]
	if json.Unmarshal([]byte(line), &proof) != nil {
		return nil, ErrInvalidTaskInput
	}
	validSource := proof.Purpose == "" && fullObjectID(proof.SourceCommit) && proof.SourceCommit != proof.TargetCommit
	if proof.Purpose == CurrentOutcomeInspection {
		validSource = proof.SourceCommit == "" && proof.PreviousAttemptID == "" && proof.PreviousCommit == ""
	}
	if t == nil || strings.Count(launch.Revision, LiveInspectionMarker) != 1 || strings.Contains(launch.Revision, VerificationIntentMarker) || !validSource || !fullObjectID(proof.TargetCommit) || !liveRepositoryIdentity.MatchString(proof.Repository) || proof.SourceAttemptID == "" || proof.OwnerID == "" || proof.RoomID == "" || proof.SourceRequestID == "" || proof.ObservedAt.IsZero() || proof.ObservedAt.After(launch.CreatedAt.Add(5*time.Second)) || proof.InstructionsHash != LaunchInstructionsHash(t.Instructions) || launch.InstructionsHash != proof.InstructionsHash || launch.TaskID != t.ID || launch.ProjectID != t.ProjectID || launch.OwnerID != proof.OwnerID || launch.RoomID != proof.RoomID || t.SourceRoomID != proof.RoomID || launch.SourceRequestID != proof.SourceRequestID || launch.RequestID != proof.RequestID(t.ID) {
		return nil, ErrInvalidTaskInput
	}
	if (proof.PreviousAttemptID == "" && proof.PreviousCommit != "") || (proof.PreviousAttemptID != "" && (!fullObjectID(proof.PreviousCommit) || proof.PreviousCommit == proof.TargetCommit)) {
		return nil, ErrInvalidTaskInput
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
	if t == nil || proof == nil || (proof.Purpose != "" && proof.Purpose != CurrentOutcomeInspection) || t.CommitSHA != proof.SourceCommit || proof.InstructionsHash != LaunchInstructionsHash(t.Instructions) {
		return ErrInvalidTaskInput
	}
	source, err := store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil {
		return err
	}
	if proof.Purpose == CurrentOutcomeInspection {
		if proof.SourceCommit != "" || proof.PreviousAttemptID != "" || proof.PreviousCommit != "" || t.CommitSHA != "" || !IsCurrentOutcomeFailure(source) || source.TaskID != t.ID || source.CancelRequested || source.CanceledAt != nil || source.FinishedAt == nil || proof.ObservedAt.Before(*source.FinishedAt) {
			return ErrInvalidTaskInput
		}
		prior, err := LiveInspectionForAttempt(ctx, store, t, source.ID)
		if err != nil || prior != nil {
			return ErrInvalidTaskInput
		}
		if !IsExecutionTimeout(source) {
			activities, err := store.ListActivities(ctx, t.ID)
			if err != nil {
				return err
			}
			if !liveImplementationFinalReport(activities, t.ID, source.ID) {
				return ErrInvalidTaskInput
			}
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
		if err != nil || binding == nil || binding.PreviousAttemptID != "" || binding.SourceAttemptID != proof.SourceAttemptID || binding.SourceCommit != proof.SourceCommit || binding.TargetCommit != proof.PreviousCommit || binding.Repository != proof.Repository || binding.OwnerID != proof.OwnerID || binding.RoomID != proof.RoomID || binding.SourceRequestID != proof.SourceRequestID || binding.InstructionsHash != proof.InstructionsHash {
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

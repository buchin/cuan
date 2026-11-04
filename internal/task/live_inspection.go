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

type LiveInspectionIntent struct {
	SourceAttemptID  string    `json:"source_attempt_id"`
	SourceCommit     string    `json:"source_commit"`
	TargetCommit     string    `json:"target_commit"`
	Repository       string    `json:"repository"`
	OwnerID          string    `json:"owner_id"`
	RoomID           string    `json:"room_id"`
	SourceRequestID  string    `json:"source_request_id"`
	InstructionsHash string    `json:"instructions_hash"`
	ObservedAt       time.Time `json:"observed_at"`
}

type LiveInspectionReceipt struct {
	SourceAttemptID   string `json:"source_attempt_id"`
	SourceCommit      string `json:"source_commit"`
	TargetCommit      string `json:"target_commit"`
	Repository        string `json:"repository"`
	ProjectChecksHash string `json:"project_checks_hash"`
}

func LiveInspectionRequestID(taskID, sourceCommit, instructionsHash string) string {
	return verificationRequestID("live-result-inspection:v1:"+instructionsHash+":", taskID, sourceCommit)
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
	if t == nil || strings.Count(launch.Revision, LiveInspectionMarker) != 1 || strings.Contains(launch.Revision, VerificationIntentMarker) || json.Unmarshal([]byte(line), &proof) != nil || !fullObjectID(proof.SourceCommit) || !fullObjectID(proof.TargetCommit) || proof.SourceCommit == proof.TargetCommit || !liveRepositoryIdentity.MatchString(proof.Repository) || proof.SourceAttemptID == "" || proof.OwnerID == "" || proof.RoomID == "" || proof.SourceRequestID == "" || proof.ObservedAt.IsZero() || proof.ObservedAt.After(launch.CreatedAt.Add(5*time.Second)) || proof.InstructionsHash != LaunchInstructionsHash(t.Instructions) || launch.InstructionsHash != proof.InstructionsHash || launch.TaskID != t.ID || launch.ProjectID != t.ProjectID || launch.OwnerID != proof.OwnerID || launch.RoomID != proof.RoomID || t.SourceRoomID != proof.RoomID || launch.SourceRequestID != proof.SourceRequestID || launch.RequestID != LiveInspectionRequestID(t.ID, proof.SourceCommit, proof.InstructionsHash) {
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
	if t == nil || proof == nil || t.CommitSHA != proof.SourceCommit || proof.InstructionsHash != LaunchInstructionsHash(t.Instructions) {
		return ErrInvalidTaskInput
	}
	source, err := store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil {
		return err
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
	var latest *TaskActivity
	for _, event := range activities {
		if event != nil && event.TaskID == t.ID && event.AttemptID != nil && *event.AttemptID == source.ID && event.ActivityType == "agent_report" && (latest == nil || event.CreatedAt.After(latest.CreatedAt) || (event.CreatedAt.Equal(latest.CreatedAt) && event.ID > latest.ID)) {
			latest = event
		}
	}
	if latest == nil || (latest.NewStatus != TaskStatusReview && latest.NewStatus != TaskStatusDone) {
		return ErrInvalidTaskInput
	}
	return nil
}

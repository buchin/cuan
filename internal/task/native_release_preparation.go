package task

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
)

const ActivityNativeReleasePreparation = "native_release_preparation"

// NativeReleasePreparation preserves implementation and release identities as
// distinct facts. Only native coordination records this after preparing and
// verifying the exact target checkout. It is not live deployment evidence.
type NativeReleasePreparation struct {
	Version          int    `json:"version"`
	TaskID           string `json:"task_id"`
	AttemptID        string `json:"attempt_id"`
	SourceCommit     string `json:"source_commit"`
	TargetCommit     string `json:"target_commit"`
	InstructionsHash string `json:"instructions_hash"`
	Repository       string `json:"repository"`
	TargetBranch     string `json:"target_branch"`
	ManifestHash     string `json:"manifest_hash"`
}

func NativeReleasePreparationForAttempt(ctx context.Context, store Store, t *Task, attemptID string) (*NativeReleasePreparation, error) {
	intent, err := NativeReleaseIntentForAttempt(ctx, store, t, attemptID)
	if err != nil || intent == nil {
		return nil, err
	}
	if err := ValidateNativeReleaseSource(ctx, store, t, intent); err != nil {
		return nil, err
	}
	att, err := store.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if att == nil || att.TaskID != t.ID || att.Status != AttemptStatusRunning || att.RunnerID == "" || att.StartedAt == nil || att.CreatedAt.IsZero() || att.StartedAt.Before(att.CreatedAt) || att.StartedAt.After(now) || att.CancelRequested || att.CanceledAt != nil || (att.LeaseExpiresAt != nil && !att.LeaseExpiresAt.After(now)) {
		return nil, ErrRequestConflict
	}
	activities, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	var result *NativeReleasePreparation
	for _, event := range activities {
		if event == nil || event.ID != "native-release-preparation:"+attemptID {
			continue
		}
		if result != nil || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != attemptID || event.Author != "cuan-native-release" || event.ActivityType != ActivityNativeReleasePreparation || event.CreatedAt.Before(*att.StartedAt) || event.CreatedAt.After(now) {
			return nil, ErrInvalidTaskInput
		}
		var receipt NativeReleasePreparation
		decoder := json.NewDecoder(strings.NewReader(event.Message))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&receipt) != nil || decoder.Decode(&struct{}{}) != io.EOF || receipt.Version != 1 || receipt.TaskID != t.ID || receipt.AttemptID != attemptID || receipt.SourceCommit != intent.SourceCommit || !fullObjectID(receipt.TargetCommit) || receipt.InstructionsHash != intent.InstructionsHash || receipt.Repository != intent.Repository || receipt.TargetBranch != intent.TargetBranch || receipt.ManifestHash != intent.ManifestHash {
			return nil, ErrInvalidTaskInput
		}
		result = &receipt
	}
	if result == nil {
		return nil, ErrInvalidTaskInput
	}
	return result, nil
}

// NativeReleaseDelivery records a successful native process whose exact target
// was independently observed healthy. Independent task verification is pending.
type NativeReleaseDelivery struct {
	Version           int       `json:"version"`
	TaskID            string    `json:"task_id"`
	AttemptID         string    `json:"attempt_id"`
	SourceAttemptID   string    `json:"source_attempt_id"`
	SourceCommit      string    `json:"source_commit"`
	TargetCommit      string    `json:"target_commit"`
	InstructionsHash  string    `json:"instructions_hash"`
	Repository        string    `json:"repository"`
	TargetBranch      string    `json:"target_branch"`
	ManifestHash      string    `json:"manifest_hash"`
	ProcessStartedAt  time.Time `json:"process_started_at"`
	ProcessFinishedAt time.Time `json:"process_finished_at"`
}

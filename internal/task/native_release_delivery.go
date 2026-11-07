package task

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// NativeReleaseDeliveryForAttempt validates completed native publication lineage.
// It does not admit a verifier or assert that a later running release is unchanged.
func NativeReleaseDeliveryForAttempt(ctx context.Context, store Store, t *Task, attemptID string) (*NativeReleaseDelivery, error) {
	intent, err := NativeReleaseIntentForAttempt(ctx, store, t, attemptID)
	if err != nil || intent == nil {
		return nil, err
	}
	if err := validateNativeReleaseSource(ctx, store, t, intent, false); err != nil {
		return nil, err
	}
	att, err := store.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if att == nil || att.TaskID != t.ID || att.Status != AttemptStatusSucceeded || att.ExitCode == nil || *att.ExitCode != 0 || att.RunnerID == "" || att.CreatedAt.IsZero() || att.StartedAt == nil || att.FinishedAt == nil || att.StartedAt.Before(att.CreatedAt) || att.FinishedAt.Before(*att.StartedAt) || att.FinishedAt.After(now) || att.CancelRequested || att.CanceledAt != nil {
		return nil, ErrInvalidTaskInput
	}
	activities, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	var result *NativeReleaseDelivery
	var prepared *NativeReleasePreparation
	var preparedAt time.Time
	for _, event := range activities {
		if event == nil {
			continue
		}
		if event.ID != "native-release-delivery:"+attemptID && event.ID != "native-release-preparation:"+attemptID {
			continue
		}
		if event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != attemptID || event.CreatedAt.Before(*att.StartedAt) || event.CreatedAt.After(*att.FinishedAt) {
			return nil, ErrInvalidTaskInput
		}
		decoder := json.NewDecoder(strings.NewReader(event.Message))
		decoder.DisallowUnknownFields()
		if event.ID == "native-release-preparation:"+attemptID {
			var receipt NativeReleasePreparation
			if prepared != nil || event.Author != "cuan-native-release" || event.ActivityType != ActivityNativeReleasePreparation || decoder.Decode(&receipt) != nil || decoder.Decode(&struct{}{}) != io.EOF {
				return nil, ErrInvalidTaskInput
			}
			prepared = &receipt
			preparedAt = event.CreatedAt
		} else {
			var receipt NativeReleaseDelivery
			if result != nil || event.Author != att.RunnerID || event.ActivityType != "native_release_delivery" || decoder.Decode(&receipt) != nil || decoder.Decode(&struct{}{}) != io.EOF {
				return nil, ErrInvalidTaskInput
			}
			result = &receipt
		}
	}
	if result == nil || prepared == nil {
		return nil, ErrInvalidTaskInput
	}
	d, p := result, prepared
	if d.Version != 1 || d.TaskID != t.ID || d.AttemptID != attemptID || d.SourceAttemptID != intent.SourceAttemptID || d.SourceCommit != intent.SourceCommit || !fullObjectID(d.TargetCommit) || d.InstructionsHash != intent.InstructionsHash || d.Repository != intent.Repository || d.TargetBranch != intent.TargetBranch || d.ManifestHash != intent.ManifestHash || d.ProcessStartedAt.Before(*att.StartedAt) || d.ProcessStartedAt.Before(preparedAt) || d.ProcessFinishedAt.Before(d.ProcessStartedAt) || d.ProcessFinishedAt.After(*att.FinishedAt) {
		return nil, ErrInvalidTaskInput
	}
	if p.Version != 1 || p.TaskID != d.TaskID || p.AttemptID != d.AttemptID || p.SourceCommit != d.SourceCommit || p.TargetCommit != d.TargetCommit || p.InstructionsHash != d.InstructionsHash || p.Repository != d.Repository || p.TargetBranch != d.TargetBranch || p.ManifestHash != d.ManifestHash {
		return nil, ErrInvalidTaskInput
	}
	if !VerifiedDeployment(activities, att, d.TargetCommit, now) {
		return nil, ErrInvalidTaskInput
	}
	if VerifiedPostProcessDeployment(activities, att, d.TargetCommit, d.ProcessFinishedAt, now) {
		return d, nil
	}
	return nil, ErrInvalidTaskInput
}

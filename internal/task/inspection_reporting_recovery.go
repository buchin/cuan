package task

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
)

// A runner-recorded reporting timeout is incomplete work, never a passed check.
// Recovery requires unchanged source and no final report or native check result.
func InspectionReportingFailure(a *TaskAttempt, events []*TaskActivity) bool {
	if a == nil || a.Status != AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 124 || a.ErrorMessage != InspectionReportingTimeout || a.RunnerID == "" || a.StartedAt == nil || a.FinishedAt == nil || !a.FinishedAt.After(*a.StartedAt) || a.CancelRequested || a.CanceledAt != nil {
		return false
	}
	unchanged, emptyChecks, transition := false, false, false
	for _, e := range events {
		if e == nil || e.TaskID != a.TaskID || e.AttemptID == nil || *e.AttemptID != a.ID {
			continue
		}
		if e.ActivityType == ActivityLiveInspection || (e.ActivityType == "agent_report" && (e.NewStatus == TaskStatusReview || e.NewStatus == TaskStatusDone || e.NewStatus == TaskStatusBlocked)) {
			return false
		}
		switch e.ActivityType {
		case "repository_scope_observed":
			var observation struct {
				Baseline string `json:"baseline"`
				Final    string `json:"final"`
			}
			if e.ID != "repository-observation:"+a.ID || e.Author != a.RunnerID || e.NewStatus != AttemptStatusFailed || json.Unmarshal([]byte(e.Message), &observation) != nil || !validVerificationHash(observation.Baseline) || observation.Baseline != observation.Final {
				return false
			}
			unchanged = true
		case ActivityTypeRunnerVerification:
			var record RunnerVerificationRecord
			if e.ID != "verification:"+a.ID || e.Author != a.RunnerID || json.Unmarshal([]byte(e.Message), &record) != nil || record.Summary != "" || record.Changes != nil {
				return false
			}
			emptyChecks = true
		case ActivityTypeAttempt:
			if e.Author == a.RunnerID && e.OldStatus == TaskStatusDoing && e.NewStatus == TaskStatusBlocked && !e.CreatedAt.Before(*a.StartedAt) && !e.CreatedAt.After(*a.FinishedAt) && strings.Contains(e.Message, "selesai dengan status 'failed' (exit code: 124,") {
				transition = true
			}
		}
	}
	return unchanged && emptyChecks && transition
}

// One separate reporting recovery retains the entire native delivery lineage.
// It does not renew stale-target recovery or permit another recovery chain.
func validateNativeInspectionReportingRecovery(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	if proof.Purpose != NativeReleaseInspection || proof.ReportingRecoveryAttemptID == proof.SourceAttemptID {
		return ErrInvalidTaskInput
	}
	a, err := store.GetAttempt(ctx, proof.ReportingRecoveryAttemptID)
	if err != nil || a == nil || a.TaskID != t.ID || a.FinishedAt == nil || proof.ObservedAt.Before(*a.FinishedAt) {
		return ErrInvalidTaskInput
	}
	previous, err := LiveInspectionForAttempt(ctx, store, t, a.ID)
	if err != nil || previous == nil || previous.ReportingRecoveryAttemptID != "" || previous.Purpose != NativeReleaseInspection {
		return ErrInvalidTaskInput
	}
	expected := *proof
	expected.ReportingRecoveryAttemptID = ""
	expected.TargetCommit, expected.ObservedAt = previous.TargetCommit, previous.ObservedAt
	if !reflect.DeepEqual(expected, *previous) || !fullObjectID(proof.TargetCommit) {
		return ErrInvalidTaskInput
	}
	if err := ValidateLiveInspectionSource(ctx, store, t, previous); err != nil {
		return err
	}
	events, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return err
	}
	if !InspectionReportingFailure(a, events) {
		return ErrInvalidTaskInput
	}
	return nil
}

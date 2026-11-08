package task

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
)

const ActivityNativeSourceReview = "native_source_review_approved"

// NativeSourceReview approves only a reviewed source stage, never publication
// authority or task completion. It must be revalidated at release admission.
type NativeSourceReview struct {
	Version          int    `json:"version"`
	TaskID           string `json:"task_id"`
	ReviewAttemptID  string `json:"review_attempt_id"`
	SourceAttemptID  string `json:"source_attempt_id"`
	SourceCommit     string `json:"source_commit"`
	InstructionsHash string `json:"instructions_hash"`
	ReportHash       string `json:"report_hash"`
}

// SourceReviewReportHash binds the latest final report for this exact session.
// Progress, blocked reports and task-level historical summaries cannot qualify.
func SourceReviewReportHash(events []*TaskActivity, taskID, attemptID string) string {
	var selected *TaskActivity
	for _, event := range events {
		if event != nil && event.TaskID == taskID && event.AttemptID != nil && *event.AttemptID == attemptID && event.ActivityType == "agent_report" && (selected == nil || event.CreatedAt.After(selected.CreatedAt) || (event.CreatedAt.Equal(selected.CreatedAt) && event.ID > selected.ID)) {
			selected = event
		}
	}
	if selected == nil || (selected.NewStatus != TaskStatusReview && selected.NewStatus != TaskStatusDone) || strings.TrimSpace(selected.Message) == "" {
		return ""
	}
	return VerificationSummaryHash(selected.Message)
}

func NativeSourceReviewForAttempt(ctx context.Context, store Store, t *Task, reviewID string) (*NativeSourceReview, error) {
	if t == nil || reviewID == "" || t.Status == "cancelled" || t.Status == "canceled" {
		return nil, ErrInvalidTaskInput
	}
	review, err := store.GetAttempt(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	if review == nil || review.TaskID != t.ID || review.Status != AttemptStatusSucceeded || review.ExitCode == nil || *review.ExitCode != 0 || review.ErrorMessage != "" || review.RunnerID == "" || review.CancelRequested || review.CanceledAt != nil || review.StartedAt == nil || review.FinishedAt == nil || review.StartedAt.Before(review.CreatedAt) || review.FinishedAt.Before(*review.StartedAt) || review.FinishedAt.After(time.Now()) {
		return nil, ErrInvalidTaskInput
	}
	proof, err := ValidateVerificationSource(ctx, store, t, reviewID)
	if err != nil || proof == nil {
		return nil, ErrInvalidTaskInput
	}
	source, err := store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil || source == nil || source.FinishedAt == nil || review.StartedAt.Before(*source.FinishedAt) {
		return nil, ErrInvalidTaskInput
	}
	events, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	verified, unchanged := false, false
	var approval *NativeSourceReview
	for _, event := range events {
		if event == nil || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != reviewID {
			continue
		}
		if event.Author == review.RunnerID && !event.CreatedAt.Before(*review.StartedAt) && !event.CreatedAt.After(*review.FinishedAt) {
			verified = verified || (event.ActivityType == "verification_source_verified" && event.Message == proof.SourceCommit)
			unchanged = unchanged || (event.ActivityType == "repository_scope_verified" && event.NewStatus == "read_only")
		}
		if event.ID == "native-source-review:"+reviewID && event.ActivityType == ActivityNativeSourceReview && event.Author == "coordinator" && !event.CreatedAt.Before(*review.FinishedAt) && !event.CreatedAt.After(time.Now()) {
			var record NativeSourceReview
			decoder := json.NewDecoder(strings.NewReader(event.Message))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&record) != nil || decoder.Decode(new(any)) != io.EOF {
				return nil, ErrInvalidTaskInput
			}
			approval = &record
		}
	}
	if approval == nil {
		return nil, nil
	}
	expected := NativeSourceReview{Version: 1, TaskID: t.ID, ReviewAttemptID: reviewID, SourceAttemptID: proof.SourceAttemptID, SourceCommit: proof.SourceCommit, InstructionsHash: proof.InstructionsHash, ReportHash: SourceReviewReportHash(events, t.ID, reviewID)}
	if !verified || !unchanged || expected.ReportHash == "" || *approval != expected {
		return nil, ErrInvalidTaskInput
	}
	return approval, nil
}

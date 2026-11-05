package task

import (
	"context"
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var reviewSourcePath = regexp.MustCompile(`\b(?:internal|pkg|web/src|scripts|docs)/[A-Za-z0-9_./-]+\.(?:go|tsx?|jsx?|mjs|cjs|py|sh|md)\b`)

type outcomeRevalidationReviewRecord struct {
	RequestID        string `json:"request_id"`
	InstructionsHash string `json:"instructions_hash"`
	Assessment       struct {
		Decision      string `json:"decision"`
		RemainingWork string `json:"remaining_work"`
		Revision      string `json:"revision"`
	} `json:"assessment"`
}

// OutcomeRevalidationReview returns the exact rejected inspection's latest
// assessment and named source paths. Generic failure prose is not fresh evidence.
func OutcomeRevalidationReview(t *Task, previous *TaskAttempt, events []*TaskActivity, requestID string) (string, []string) {
	if t == nil || previous == nil || previous.Status != AttemptStatusSucceeded || previous.FinishedAt == nil || previous.CancelRequested || previous.CanceledAt != nil {
		return "", nil
	}
	var latest *TaskActivity
	var selected outcomeRevalidationReviewRecord
	for _, event := range events {
		if event == nil || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != previous.ID || event.Author != "coordinator" || event.ActivityType != "coordinator_review_assessment" || event.CreatedAt.Before(*previous.FinishedAt) {
			continue
		}
		var record outcomeRevalidationReviewRecord
		if json.Unmarshal([]byte(event.Message), &record) != nil || record.InstructionsHash != LaunchInstructionsHash(t.Instructions) || record.RequestID == "" {
			continue
		}
		if latest == nil || event.CreatedAt.After(latest.CreatedAt) || (event.CreatedAt.Equal(latest.CreatedAt) && event.ID > latest.ID) {
			latest = event
			selected = record
		}
	}
	if latest == nil || (requestID != "" && requestID != selected.RequestID) || selected.Assessment.Decision != "revise" || selected.Assessment.RemainingWork != "implementation" || len(selected.Assessment.Revision) > 16000 {
		return "", nil
	}
	unique := map[string]bool{}
	for _, span := range reviewSourcePath.FindAllStringIndex(selected.Assessment.Revision, -1) {
		if span[0] > 0 && (selected.Assessment.Revision[span[0]-1] == '/' || selected.Assessment.Revision[span[0]-1] == '\\') {
			continue
		}
		name := selected.Assessment.Revision[span[0]:span[1]]
		if path.Clean(name) == name && !strings.Contains(name, "../") {
			unique[name] = true
		}
	}
	if len(unique) == 0 || len(unique) > 16 {
		return "", nil
	}
	paths := make([]string, 0, len(unique))
	for name := range unique {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return selected.RequestID, paths
}

func validateOutcomeRevalidation(ctx context.Context, store Store, t *Task, proof *LiveInspectionIntent) error {
	previous, err := store.GetAttempt(ctx, proof.PreviousAttemptID)
	if err != nil {
		return err
	}
	if previous == nil || previous.TaskID != t.ID || previous.FinishedAt == nil || proof.ObservedAt.Before(*previous.FinishedAt) {
		return ErrInvalidTaskInput
	}
	binding, err := LiveInspectionForAttempt(ctx, store, t, previous.ID)
	if err != nil {
		return err
	}
	if binding == nil || binding.Purpose != CurrentOutcomeInspection || binding.PreviousAttemptID != "" || binding.SourceAttemptID != proof.SourceAttemptID || binding.SourceCommit != proof.SourceCommit || binding.TargetCommit != proof.PreviousCommit || binding.Repository != proof.Repository || binding.OwnerID != proof.OwnerID || binding.RoomID != proof.RoomID || binding.SourceRequestID != proof.SourceRequestID || binding.InstructionsHash != proof.InstructionsHash {
		return ErrInvalidTaskInput
	}
	if err := ValidateLiveInspectionSource(ctx, store, t, binding); err != nil {
		return err
	}
	events, err := store.ListActivities(ctx, t.ID)
	if err != nil {
		return err
	}
	request, paths := OutcomeRevalidationReview(t, previous, events, proof.ReviewRequestID)
	if request == "" || request != proof.ReviewRequestID || !slices.Equal(paths, proof.ReviewPaths) {
		return ErrInvalidTaskInput
	}
	detail := &TaskDetailResponse{Task: *t, LatestAttempt: previous, Attempts: []*TaskAttempt{previous}, Activities: events}
	if !ProjectChecksPassed(detail) || VerificationScopeBaseline(detail, previous.ID) == "" {
		return ErrInvalidTaskInput
	}
	for _, event := range events {
		if event == nil || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != previous.ID || event.ID != "live-result:"+previous.ID || event.Author != previous.RunnerID || event.ActivityType != ActivityLiveInspection {
			continue
		}
		var receipt LiveInspectionReceipt
		if json.Unmarshal([]byte(event.Message), &receipt) == nil && receipt.Purpose == CurrentOutcomeInspection && receipt.SourceAttemptID == binding.SourceAttemptID && receipt.SourceCommit == "" && receipt.TargetCommit == binding.TargetCommit && receipt.Repository == binding.Repository && receipt.ProjectChecksHash == VerificationSummaryHash(AttemptVerificationEvidence(detail).Current) {
			return nil
		}
	}
	return ErrInvalidTaskInput
}

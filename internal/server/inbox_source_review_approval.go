package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/task"
)

const inboxSourceReviewPolicy = "Source review policy: v1"

// Reassess older reports that have independent source evidence but were saved
// before an explicit source verdict was required. This does not approve source,
// reset a correction allowance, rerun the verifier or authorize publication.
func (s *Server) sourceReviewClassificationMissing(ctx context.Context, d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded {
		return false
	}
	var latest *task.TaskActivity
	for _, event := range d.Activities {
		if event != nil && event.TaskID == d.ID && event.AttemptID != nil && *event.AttemptID == d.LatestAttempt.ID && event.ActivityType == reviewAssessmentActivity && event.Author == "coordinator" && (latest == nil || event.CreatedAt.After(latest.CreatedAt) || (event.CreatedAt.Equal(latest.CreatedAt) && event.ID > latest.ID)) {
			latest = event
		}
	}
	if latest == nil {
		return false
	}
	var record reviewAssessmentRecord
	// A new review evaluates current evidence. The old digest need not still
	// match: it supplies only evidence that this stage omitted the new field,
	// never a source verdict or acceptance authority.
	if json.Unmarshal([]byte(latest.Message), &record) != nil || record.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) || record.Assessment.SourceReview != "" {
		return false
	}
	source, proof := s.verifiedSupplementalSource(ctx, d)
	return source != nil && proof != nil
}

// A model verdict is insufficient: retain approval only after the existing
// independent-source and read-only scope checks, plus the saved assessment.
func (s *Server) saveNativeSourceReview(ctx context.Context, d *task.TaskDetailResponse, requestID string, assessment inboxReviewEvaluation) error {
	if assessment.SourceReview != "pass" {
		return nil
	}
	if d == nil || d.LatestAttempt == nil {
		return task.ErrInvalidTaskInput
	}
	saved, ok := reviewAssessment(d, requestID)
	if !ok || saved != assessment {
		return fmt.Errorf("%w: source review assessment is not current", task.ErrInvalidTaskInput)
	}
	source, proof := s.verifiedSupplementalSource(ctx, d)
	if source == nil || proof == nil {
		return fmt.Errorf("%w: independent source evidence is missing", task.ErrInvalidTaskInput)
	}
	// Backend visual requirements still apply to source-stage approval.
	if gate := s.requireVisualReview(ctx, d, inboxReviewEvaluation{Decision: "pass", VisualInspection: assessment.VisualInspection}); gate.Decision != "pass" {
		return fmt.Errorf("%w: source visual review is incomplete", task.ErrInvalidTaskInput)
	}
	for _, act := range d.Activities {
		if act != nil && strings.HasPrefix(act.ID, "native-source-review:") && act.AttemptID != nil && *act.AttemptID != "" {
			if verified, err := task.NativeSourceReviewForAttempt(ctx, s.taskSvc.Store(), &d.Task, *act.AttemptID); err == nil && verified != nil && verified.SourceCommit == proof.SourceCommit {
				return nil
			}
		}
	}
	for _, a := range d.Attempts {
		if a != nil && a.ID != "" {
			if verified, err := task.NativeSourceReviewForAttempt(ctx, s.taskSvc.Store(), &d.Task, a.ID); err == nil && verified != nil && verified.SourceCommit == proof.SourceCommit {
				return nil
			}
		}
	}
	receipt := task.NativeSourceReview{Version: 1, TaskID: d.ID, ReviewAttemptID: d.LatestAttempt.ID, SourceAttemptID: proof.SourceAttemptID, SourceCommit: proof.SourceCommit, InstructionsHash: proof.InstructionsHash, ReportHash: task.SourceReviewReportHash(d.Activities, d.ID, d.LatestAttempt.ID), AssessmentHash: task.SourceReviewAssessmentHash(d.Activities, d.ID, d.LatestAttempt.ID, proof.InstructionsHash)}
	if receipt.ReportHash == "" || receipt.AssessmentHash == "" {
		return task.ErrInvalidTaskInput
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	event := &task.TaskActivity{ID: "native-source-review:" + d.LatestAttempt.ID, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityNativeSourceReview, Author: "coordinator", Message: string(raw), CreatedAt: time.Now().UTC()}
	if err := s.taskSvc.Store().CreateActivity(ctx, event); err != nil {
		previous, readErr := task.NativeSourceReviewForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
		if readErr != nil || previous == nil || *previous != receipt {
			return err
		}
	}
	verified, err := task.NativeSourceReviewForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil {
		return err
	}
	if verified == nil {
		return task.ErrInvalidTaskInput
	}
	return nil
}

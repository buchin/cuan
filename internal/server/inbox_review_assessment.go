package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const inboxReviewAssessmentPolicy = "Review assessment policy: v1"
const reviewAssessmentActivity = "coordinator_review_assessment"

type reviewAssessmentRecord struct {
	RequestID        string                `json:"request_id"`
	InstructionsHash string                `json:"instructions_hash"`
	EvidenceDigest   string                `json:"evidence_digest"`
	Assessment       inboxReviewEvaluation `json:"assessment"`
}

// A model assessment is evidence of a review, not task acceptance. State changes,
// independent execution checks and approved publication gates remain authoritative.
func reviewAssessment(d *task.TaskDetailResponse, requestID string) (inboxReviewEvaluation, bool) {
	if d == nil || d.LatestAttempt == nil {
		return inboxReviewEvaluation{}, false
	}
	var selected *task.TaskActivity
	var assessment inboxReviewEvaluation
	for _, a := range d.Activities {
		if a == nil || a.TaskID != d.ID || a.AttemptID == nil || *a.AttemptID != d.LatestAttempt.ID || a.ActivityType != reviewAssessmentActivity || a.Author != "coordinator" {
			continue
		}
		var record reviewAssessmentRecord
		if json.Unmarshal([]byte(a.Message), &record) != nil || (requestID != "" && record.RequestID != requestID) || record.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) || record.EvidenceDigest != inboxReviewEvidenceDigest(d) {
			continue
		}
		raw, _ := json.Marshal(record.Assessment)
		if parsed, err := parseInboxReviewEvaluation(string(raw)); err == nil {
			if selected == nil || a.CreatedAt.After(selected.CreatedAt) || (a.CreatedAt.Equal(selected.CreatedAt) && a.ID > selected.ID) {
				selected, assessment = a, parsed
			}
		}
	}
	return assessment, selected != nil
}
func reviewAssessmentMissing(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded {
		return false
	}
	provenance := inboxReportEvidence(d).Provenance
	if provenance != "current_attempt_report" && provenance != "current_runner_finalization" {
		return false
	}
	_, found := reviewAssessment(d, "")
	return !found
}

func (s *Server) saveReviewAssessment(ctx context.Context, d *task.TaskDetailResponse, q db.InboxRequest, assessment inboxReviewEvaluation) (inboxReviewEvaluation, error) {
	if previous, found := reviewAssessment(d, q.RequestID); found {
		return previous, nil
	}
	record := reviewAssessmentRecord{RequestID: q.RequestID, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), EvidenceDigest: inboxReviewEvidenceDigest(d), Assessment: assessment}
	raw, err := json.Marshal(record)
	if err != nil {
		return inboxReviewEvaluation{}, err
	}
	event := &task.TaskActivity{ID: delegationMessageID("review-assessment:" + q.RequestID + ":" + record.EvidenceDigest), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: reviewAssessmentActivity, Author: "coordinator", Message: string(raw), CreatedAt: time.Now().UTC()}
	if err = s.taskSvc.Store().CreateActivity(ctx, event); err != nil {
		// A concurrent/retried application may already have stored the same decision.
		if current, readErr := s.taskSvc.GetTaskDetail(ctx, d.ID); readErr == nil {
			if previous, found := reviewAssessment(current, q.RequestID); found {
				return previous, nil
			}
		}
		return inboxReviewEvaluation{}, errors.New("task review assessment could not be saved")
	}
	d.Activities = append(d.Activities, event)
	return assessment, nil
}

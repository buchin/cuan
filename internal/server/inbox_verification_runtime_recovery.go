package server

import (
	"context"
	"encoding/json"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// A rejected legacy verifier can recover once after a newly verified browser
// capability becomes available. It cannot change source, publication or scope,
// replay an interrupted process, or chain another verification recovery.
func (s *Server) verificationRuntimeRecoveryProof(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, prior *task.VerificationIntent, assessment inboxReviewEvaluation) *task.VerificationIntent {
	if prior == nil || prior.RecoveryAttemptID != "" || prior.FinalizeAttemptID != "" || s.runnerSvc == nil || prior.OwnerID != q.OwnerID || prior.RoomID != q.RoomID || assessment.VisualInspection == "" || len(s.inboxReviewImages(ctx, d)) == 0 || d.LatestAttempt.FinishedAt == nil || s.verificationReviewContext(ctx, d) == nil {
		return nil
	}
	runtime := s.runnerSvc.BrowserRuntime(ctx, d.LatestAttempt.RunnerID)
	if runtime == nil || runtime.CheckedAt.Before(*d.LatestAttempt.FinishedAt) {
		return nil
	}
	recorded := false
	for _, a := range d.Activities {
		if a == nil || a.TaskID != d.ID || a.AttemptID == nil || *a.AttemptID != d.LatestAttempt.ID || a.Author != d.LatestAttempt.RunnerID {
			continue
		}
		var previous task.VerificationRuntime
		switch a.ActivityType {
		case task.ActivityVerificationRuntime:
			if json.Unmarshal([]byte(a.Message), &previous) != nil || previous.Fingerprint == runtime.Fingerprint {
				return nil
			}
		case task.ActivityVerificationRuntimeRecovery:
			if json.Unmarshal([]byte(a.Message), &previous) != nil || previous.Fingerprint != runtime.Fingerprint {
				return nil
			}
			recorded = true
		}
	}
	if !recorded {
		raw, _ := json.Marshal(runtime)
		event := &task.TaskActivity{ID: "runtime-recovery:" + d.LatestAttempt.ID, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityVerificationRuntimeRecovery, Author: d.LatestAttempt.RunnerID, Message: string(raw), CreatedAt: time.Now().UTC()}
		if err := s.taskSvc.Store().CreateActivity(ctx, event); err != nil {
			current, readErr := s.taskSvc.GetTaskDetail(ctx, d.ID)
			if readErr != nil {
				return nil
			}
			found := false
			for _, a := range current.Activities {
				if a.ID == event.ID && a.Author == event.Author && a.Message == event.Message {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		} else {
			d.Activities = append(d.Activities, event)
		}
	}
	proof := *prior
	proof.RecoveryAttemptID = d.LatestAttempt.ID
	proof.RuntimeFingerprint = runtime.Fingerprint
	proof.AssessmentRequestID = q.RequestID
	return &proof
}

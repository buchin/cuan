package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"time"
)

func (s *Server) recoverInspectionVerifier(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusFailed {
		return false
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || previous.Purpose != task.CurrentOutcomeInspection || previous.VerifierRecoveryAttemptID != "" {
		return false
	}
	proof := *previous
	proof.VerifierRecoveryAttemptID = d.LatestAttempt.ID
	proof.ObservedAt = time.Now().UTC()
	if task.ValidateInspectionVerifierRecovery(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Commit != proof.TargetCommit || observed.Repository != proof.Repository {
		return false
	}
	proof.ObservedAt = observed.ObservedAt
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	return s.startLiveResultInspection(ctx, d, origin, &proof, true)
}

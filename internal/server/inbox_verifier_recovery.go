package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) recoverInspectionVerifier(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if d == nil || d.LatestAttempt == nil || (d.LatestAttempt.Status != task.AttemptStatusFailed && d.LatestAttempt.Status != task.AttemptStatusInterrupted) {
		return false
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || previous.Purpose != task.CurrentOutcomeInspection || previous.VerifierRecoveryAttemptID != "" || previous.OwnerID != origin.OwnerID || previous.RoomID != origin.RoomID || previous.SourceRequestID != origin.RequestID {
		return false
	}
	proof := *previous
	proof.VerifierRecoveryAttemptID = d.LatestAttempt.ID
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Repository != proof.Repository {
		return false
	}
	if observed.Commit != proof.TargetCommit {
		if !task.InspectionInfrastructureFailure(d.LatestAttempt, d.Activities) {
			return false
		}
		if proof.PreparedPublicationAttemptID != "" && !s.preparedSourceAlreadyPublished(ctx, d) {
			return false
		}
		proof.TargetCommit = observed.Commit
	}
	proof.ObservedAt = observed.ObservedAt
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil || !s.liveInspectionReadScopeAllowed(ctx, d, proof.SourceAttemptID) || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	return s.startLiveResultInspection(ctx, d, origin, &proof, true)
}

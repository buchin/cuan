package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) recoverCapacityInspection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if d == nil || d.Status != task.TaskStatusBlocked || !task.QuotaStoppedVerification(d.LatestAttempt) || d.SourceRoomID != origin.RoomID || d.SourceMessageID != origin.MessageID || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || previous.Purpose != task.CurrentOutcomeInspection || previous.CapacityRecoveryAttemptID != "" || previous.OwnerID != origin.OwnerID || previous.RoomID != origin.RoomID || previous.SourceRequestID != origin.RequestID || !s.liveInspectionReadScopeAllowed(ctx, d, previous.SourceAttemptID) {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Repository != previous.Repository {
		return false
	}
	if observed.Commit != previous.TargetCommit && previous.PreparedPublicationAttemptID != "" && !s.preparedSourceAlreadyPublished(ctx, d) {
		return false
	}
	proof := *previous
	proof.CapacityRecoveryAttemptID = d.LatestAttempt.ID
	proof.TargetCommit, proof.ObservedAt = observed.Commit, observed.ObservedAt
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
		return false
	}
	// Save waiting work even while every allowed model is cooled down. Existing
	// launch scheduling handles retries without creating more failed attempts or
	// probing paid inference. It uses the same native task pool at admission.
	return s.startLiveResultInspection(ctx, d, origin, &proof, true)
}

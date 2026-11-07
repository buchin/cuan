package server

import (
	"context"
	"log"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) recoverCapacityInspection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	proof, code := s.planCapacityInspection(ctx, d, origin)
	if proof == nil {
		if d != nil && task.QuotaStoppedVerification(d.LatestAttempt) && ctx.Value(inboxDeliveryDiagnosticKey{}) == true {
			log.Printf("inbox inspection checkpoint task=%s code=%s", d.ID, code)
		}
		return false
	}
	return s.startLiveResultInspection(ctx, d, origin, proof, true)
}

// Shared read-only planning keeps execution and Owner diagnostics on the same gates.
func (s *Server) planCapacityInspection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) (*task.LiveInspectionIntent, string) {
	if d == nil || d.Status != task.TaskStatusBlocked || !task.QuotaStoppedVerification(d.LatestAttempt) || d.SourceRoomID != origin.RoomID || d.SourceMessageID != origin.MessageID || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return nil, "capacity_authorization_unavailable"
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || previous.Purpose != task.CurrentOutcomeInspection || previous.CapacityRecoveryAttemptID != "" || previous.OwnerID != origin.OwnerID || previous.RoomID != origin.RoomID || previous.SourceRequestID != origin.RequestID || !s.liveInspectionReadScopeAllowed(ctx, d, previous.SourceAttemptID) {
		return nil, "capacity_parent_intent_invalid"
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Repository != previous.Repository {
		return nil, "capacity_running_release_unavailable"
	}
	if observed.Commit != previous.TargetCommit && previous.PreparedPublicationAttemptID != "" && !s.preparedSourceAlreadyPublished(ctx, d) {
		return nil, "capacity_prepared_publication_unverified"
	}
	proof := *previous
	proof.CapacityRecoveryAttemptID = d.LatestAttempt.ID
	proof.TargetCommit, proof.ObservedAt = observed.Commit, observed.ObservedAt
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
		return nil, "capacity_source_binding_invalid"
	}
	// Save waiting work even while every allowed model is cooled down. Existing
	// launch scheduling handles retries without creating more failed attempts or
	// probing paid inference. It uses the same native task pool at admission.
	return &proof, "capacity_ready"
}

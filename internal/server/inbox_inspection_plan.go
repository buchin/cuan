package server

import (
	"context"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) liveInspectionEligibility(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) string {
	if d == nil || d.LatestAttempt == nil {
		return "no_attempt"
	}
	if d.Status != task.TaskStatusBlocked && d.Status != task.TaskStatusReview {
		return "task_not_waiting_for_recovery"
	}
	if origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) {
		return "managed_request_binding_invalid"
	}
	if !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		return "original_review_authority_unavailable"
	}
	return ""
}

func (s *Server) liveInspectionSource(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) (*task.TaskAttempt, string) {
	source := d.LatestAttempt
	if original, proof := s.verifiedSupplementalSource(ctx, d); original != nil {
		if proof.OwnerID != origin.OwnerID || proof.RoomID != origin.RoomID || proof.SourceRequestID != origin.RequestID {
			return nil, "supplemental_authorization_mismatch"
		}
		source = original.LatestAttempt
	}
	return source, ""
}

func liveInspectionReviewGate(d *task.TaskDetailResponse, source *task.TaskAttempt) string {
	if d.Status != task.TaskStatusReview {
		return ""
	}
	assessment, found := reviewAssessment(d, "")
	if !found || assessment.Decision != "revise" || assessment.RemainingWork != "verification" {
		return "current_review_not_verification"
	}
	if !task.IsUnpublishedReviewedOutcome(&d.Task, source, d.Activities) && !task.IsPublicationReconciliationOutcome(&d.Task, source, d.Activities) && len(task.FailedCheckRevalidationPaths(d)) == 0 {
		return "review_source_not_eligible"
	}
	return ""
}

// This describes the ordinary existing-release plan, not the separate identity,
// verifier or unstarted-preparation repairs. It performs no recovery mutations.
func (s *Server) liveInspectionPlanDiagnostic(ctx context.Context, d *task.TaskDetailResponse) *task.InspectionPlanDiagnostic {
	diagnostic := &task.InspectionPlanDiagnostic{Code: "original_request_not_found"}
	if d == nil || d.SourceRoomID == "" || d.SourceMessageID == "" {
		diagnostic.Code = "not_managed_inbox_work"
		return diagnostic
	}
	repo, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		diagnostic.Code = "conversation_storage_unavailable"
		return diagnostic
	}
	requests, err := repo.ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		diagnostic.Code = "conversation_storage_unavailable"
		return diagnostic
	}
	for _, origin := range requests {
		if origin.MessageID != d.SourceMessageID {
			continue
		}
		if code := s.liveInspectionEligibility(ctx, d, origin); code != "" {
			diagnostic.Code = code
			return diagnostic
		}
		source, code := s.liveInspectionSource(ctx, d, origin)
		if code != "" {
			diagnostic.Code = code
			return diagnostic
		}
		diagnostic.SourceAttemptID = source.ID
		diagnostic.SupplementalSourceVerified = source.ID != d.LatestAttempt.ID
		proof, code := s.planLiveResultInspection(ctx, d, origin, source)
		diagnostic.Code = code
		if proof != nil {
			diagnostic.TargetCommit = proof.TargetCommit
		}
		return diagnostic
	}
	return diagnostic
}

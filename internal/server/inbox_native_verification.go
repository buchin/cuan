package server

import (
	"context"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Delivered native work enters the existing independent read-only inspection
// lane. Successful release process/health never replace original DoD review.
func (s *Server) recoverNativeReleaseVerification(ctx context.Context, d *task.TaskDetailResponse, requests []db.InboxRequest) bool {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded || d.Status != task.TaskStatusReview || s.projectSvc == nil {
		return false
	}
	delivery, err := task.NativeReleaseDeliveryForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || delivery == nil {
		return false
	}
	intent, err := task.NativeReleaseIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || intent == nil {
		return false
	}
	p, err := s.projectSvc.TaskContext(ctx, d.ProjectID, d.Instructions)
	if err != nil || p == nil || p.IsArchived {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Commit != delivery.TargetCommit || !strings.EqualFold(observed.Repository, delivery.Repository) {
		return false
	}
	proof := &task.LiveInspectionIntent{Purpose: task.NativeReleaseInspection, SourceAttemptID: d.LatestAttempt.ID, SourceCommit: delivery.SourceCommit, TargetCommit: delivery.TargetCommit, Repository: delivery.Repository, OwnerID: intent.OwnerID, RoomID: intent.RoomID, SourceRequestID: intent.SourceRequestID, InstructionsHash: delivery.InstructionsHash, ObservedAt: observed.ObservedAt}
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, proof) != nil {
		return false
	}
	for _, origin := range requests {
		if origin.RequestID == intent.SourceRequestID && origin.OwnerID == intent.OwnerID && origin.RoomID == intent.RoomID {
			return s.startLiveResultInspection(ctx, d, origin, proof, false)
		}
	}
	return false
}

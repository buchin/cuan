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
	if d != nil && d.LatestAttempt != nil && d.LatestAttempt.Status == task.AttemptStatusSucceeded && (d.Status == task.TaskStatusBlocked || d.Status == task.TaskStatusReview) {
		if previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err == nil && previous != nil && previous.Purpose == task.NativeReleaseInspection {
			for _, origin := range requests {
				if origin.RequestID == previous.SourceRequestID && origin.OwnerID == previous.OwnerID && origin.RoomID == previous.RoomID && s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) && s.recoverChangedOutcomeInspection(ctx, d, origin) {
					return true
				}
			}
		}
	}

	if d != nil && d.LatestAttempt != nil && d.LatestAttempt.Status == task.AttemptStatusFailed {
		if s.recoverNativeInspectionReporting(ctx, d, requests) {
			return true
		}
		return s.recoverStaleNativeReleaseInspection(ctx, d, requests)
	}
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

func (s *Server) recoverNativeInspectionReporting(ctx context.Context, d *task.TaskDetailResponse, requests []db.InboxRequest) bool {
	if s.projectSvc == nil || (d.Status != task.TaskStatusReview && d.Status != task.TaskStatusBlocked) || !task.InspectionReportingFailure(d.LatestAttempt, d.Activities) {
		return false
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || previous.Purpose != task.NativeReleaseInspection || previous.ReportingRecoveryAttemptID != "" {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Repository != previous.Repository {
		return false
	}
	if observed.Commit != previous.TargetCommit {
		contained, err := s.projectSvc.PublishedCommitContained(ctx, d.ProjectID, previous.TargetCommit, observed.Commit)
		if err != nil || !contained {
			return false
		}
	}
	proof := *previous
	proof.ReportingRecoveryAttemptID = d.LatestAttempt.ID
	proof.TargetCommit, proof.ObservedAt = observed.Commit, observed.ObservedAt
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
		return false
	}
	for _, origin := range requests {
		if origin.RequestID == proof.SourceRequestID && origin.OwnerID == proof.OwnerID && origin.RoomID == proof.RoomID && s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) && s.taskContinuationAfterCancel(ctx, origin, d) && s.liveInspectionReadScopeAllowed(ctx, d, proof.SourceAttemptID) {
			return s.startLiveResultInspection(ctx, d, origin, &proof, false)
		}
	}
	return false
}

func (s *Server) recoverStaleNativeReleaseInspection(ctx context.Context, d *task.TaskDetailResponse, requests []db.InboxRequest) bool {
	if s.projectSvc == nil || (d.Status != task.TaskStatusReview && d.Status != task.TaskStatusBlocked) || d.LatestAttempt.ErrorMessage != task.StaleInspectionReleaseMessage {
		return false
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || previous.Purpose != task.NativeReleaseInspection || previous.VerifierRecoveryAttemptID != "" {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Commit == previous.TargetCommit || observed.Repository != previous.Repository {
		return false
	}
	// A newer release must contain the delivered target as well as its original
	// implementation (enforced again by native snapshot preparation).
	contained, err := s.projectSvc.PublishedCommitContained(ctx, d.ProjectID, previous.TargetCommit, observed.Commit)
	if err != nil || !contained {
		return false
	}
	proof := *previous
	proof.VerifierRecoveryAttemptID = d.LatestAttempt.ID
	proof.TargetCommit, proof.ObservedAt = observed.Commit, observed.ObservedAt
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
		return false
	}
	for _, origin := range requests {
		if origin.RequestID == proof.SourceRequestID && origin.OwnerID == proof.OwnerID && origin.RoomID == proof.RoomID && s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) && s.taskContinuationAfterCancel(ctx, origin, d) && s.liveInspectionReadScopeAllowed(ctx, d, proof.SourceAttemptID) {
			return s.startLiveResultInspection(ctx, d, origin, &proof, false)
		}
	}
	return false
}

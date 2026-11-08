package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// A mistaken legacy production inspection must not strand unfinished local
// implementation. This permits only a new developer attempt under the saved PM
// decision; it never grants writes in the inspection snapshot or accepts work.
func (s *Server) localSourceCorrectionAllowed(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, revision string) bool {
	if d == nil || d.LatestAttempt == nil || d.ExecutionCategory != task.ExecutionCategoryCode || d.CommitSHA != "" || (d.Status != task.TaskStatusBlocked && d.Status != task.TaskStatusReview) || d.LatestAttempt.Status != task.AttemptStatusFailed || d.LatestAttempt.FinishedAt == nil || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || q.RoomID != d.SourceRoomID || s.inboxReviewedTask(ctx, q) == nil {
		return false
	}
	saved, ok := reviewAssessment(d, q.RequestID)
	if !ok || saved.Decision != "revise" || saved.RemainingWork != "implementation" || saved.RequestedOutcome == "needs_input" || saved.ManagerReason == "" || (saved.ManagerAction != "continue" && saved.ManagerAction != "replan") || !matchesManagerRevision(saved, revision) {
		return false
	}
	intent, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || intent == nil || intent.Purpose != task.CurrentOutcomeInspection || intent.PreviousAttemptID != "" || intent.SourceCommit != "" || intent.OwnerID != q.OwnerID || intent.RoomID != d.SourceRoomID || intent.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) {
		return false
	}
	source, err := s.taskSvc.Store().GetAttempt(ctx, intent.SourceAttemptID)
	if err != nil || source == nil || source.TaskID != d.ID || !task.IsExecutionTimeout(source) || source.FinishedAt == nil || source.FinishedAt.After(d.LatestAttempt.CreatedAt) || source.CancelRequested || source.CanceledAt != nil {
		return false
	}
	limits, err := s.taskSvc.AttemptPublicationPolicy(ctx, d.ID, source.ID)
	if err != nil || limits.ReadOnlyRepository || !limits.DenyDeploy {
		return false
	}
	approved, _ := s.inboxApprovedTaskScope(ctx, d)
	return approved != nil && !approved.ReadOnlyRepository && approved.DenyDeploy
}

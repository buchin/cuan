package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// An outcome inspection must not strand unfinished original implementation.
// This permits only a new developer attempt under the saved PM
// decision; it never grants writes in the inspection snapshot or accepts work.
func (s *Server) localSourceCorrectionAllowed(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, revision string) bool {
	if d == nil || d.LatestAttempt == nil || d.ExecutionCategory != task.ExecutionCategoryCode || (d.Status != task.TaskStatusBlocked && d.Status != task.TaskStatusReview) || d.LatestAttempt.FinishedAt == nil || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || q.RoomID != d.SourceRoomID || s.inboxReviewedTask(ctx, q) == nil {
		return false
	}
	saved, ok := reviewAssessment(d, q.RequestID)
	if !ok || saved.Decision != "revise" || saved.RemainingWork != "implementation" || saved.RequestedOutcome == "needs_input" || saved.ManagerReason == "" || (saved.ManagerAction != "continue" && saved.ManagerAction != "replan") || !matchesManagerRevision(saved, revision) {
		return false
	}
	if d.CommitSHA != "" {
		return s.inspectedSourceCorrectionAllowed(ctx, q, d)
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
	if err != nil || limits.ReadOnlyRepository {
		return false
	}
	approved, _ := s.inboxApprovedTaskScope(ctx, d)
	if approved == nil || approved.ReadOnlyRepository || approved.DenyDeploy != limits.DenyDeploy || approved.DenyPush != limits.DenyPush {
		return false
	}
	if d.LatestAttempt.Status == task.AttemptStatusFailed {
		// Preserve the bounded legacy local-only recovery.
		return approved.DenyDeploy
	}
	// A successful independent inspection can truthfully report that the
	// timed-out developer never implemented the original task. Return that
	// gap to a separate developer; never grant writes to this snapshot.
	_, unchanged, scopeErr := s.inboxReviewRepositoryScope(ctx, d)
	return d.LatestAttempt.Status == task.AttemptStatusSucceeded &&
		inboxReviewExecutionSucceeded(d) && task.ProjectChecksPassed(d) &&
		inboxReportEvidence(d).Provenance == "current_attempt_report" &&
		scopeErr == nil && unchanged
}

// A successful current-outcome inspection may identify unfinished source work.
// Keep its snapshot read-only and return the saved PM correction to a separate
// developer under the original scope, using the existing admission path.
func (s *Server) inspectedSourceCorrectionAllowed(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse) bool {
	intent, err := task.ValidateLiveInspection(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || intent == nil || intent.Purpose != task.CurrentOutcomeInspection || intent.OwnerID != q.OwnerID || intent.RoomID != d.SourceRoomID || intent.SourceCommit != d.CommitSHA || intent.SourceCommit == "" || d.LatestAttempt.Status != task.AttemptStatusSucceeded || inboxReportEvidence(d).Provenance != "current_attempt_report" || s.liveInspectionEvidence(ctx, d, false) == nil {
		return false
	}
	current := s.runningProjectRelease(ctx, d)
	if current == nil || current.Repository != intent.Repository {
		return false
	}
	source, err := s.taskSvc.Store().GetAttempt(ctx, intent.SourceAttemptID)
	if err != nil || source == nil || source.TaskID != d.ID || source.FinishedAt == nil || source.FinishedAt.After(d.LatestAttempt.CreatedAt) || source.CancelRequested || source.CanceledAt != nil || (source.Status != task.AttemptStatusSucceeded && source.Status != task.AttemptStatusFailed) {
		return false
	}
	limits, err := s.taskSvc.AttemptPublicationPolicy(ctx, d.ID, source.ID)
	approved, _ := s.inboxApprovedTaskScope(ctx, d)
	return err == nil && approved != nil && !approved.ReadOnlyRepository && !limits.ReadOnlyRepository && approved.DenyDeploy == limits.DenyDeploy && approved.DenyPush == limits.DenyPush
}

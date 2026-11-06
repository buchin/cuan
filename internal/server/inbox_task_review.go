package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type inboxTaskReviewKey struct{}

// Result review uses the canonical task snapshot rather than replaying an entire
// conversation, which may contain obsolete agent actions or false completion.
func (s *Server) inboxTaskReviewPrompt(ctx context.Context, q db.InboxRequest) (string, bool) {
	if s.projectStudioRepo == nil {
		return "", false
	}
	m, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
	if err != nil || m == nil || m.RoomID != q.RoomID || m.AuthorType != "system" || m.AuthorID != "cuan_work_review" {
		return "", false
	}
	d := s.inboxReviewedTask(ctx, q)
	if d == nil || d.Status == "cancelled" || d.Status == "canceled" {
		return "", true
	}
	if a := d.LatestAttempt; a != nil && (a.CancelRequested || a.CanceledAt != nil || a.Status == task.AttemptStatusCanceled) {
		return "", true
	}
	if isCustomerReplyDelivery(q, d) {
		if !s.acceptedCustomerReplyEligible(ctx, d) || !s.customerReplyAuthorized(ctx, q, d) {
			return "", true
		}
		return customerReplyDeliveryInstruction + "\n\nAccepted result data:\n" + s.inboxReviewData(ctx, d), true
	}
	return s.inboxReviewData(ctx, d), true
}

// Explain a stale automatic update using current saved state, without running AI
// or granting permission to resume cancelled work.
func (s *Server) inboxStaleReviewOutcome(ctx context.Context, q db.InboxRequest) string {
	const fallback = "This update is no longer current. Your saved work is unchanged; no work was restarted or marked complete by this update."
	if s.projectStudioRepo == nil || s.taskSvc == nil {
		return fallback
	}
	m, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
	if err != nil || m == nil || m.RoomID != q.RoomID || m.AuthorType != "system" || m.AuthorID != "cuan_work_review" {
		return fallback
	}
	for _, match := range inboxReviewTaskLink.FindAllStringSubmatch(m.Content, -1) {
		d, err := s.taskSvc.GetTaskDetail(ctx, match[1])
		if err != nil || d == nil || d.LatestAttempt == nil {
			continue
		}
		if (d.AssignedAgent != "" && d.AssignedAgent != q.AgentID) || (d.LatestAttempt.AgentName != "" && d.LatestAttempt.AgentName != q.AgentID) {
			continue
		}
		a := d.LatestAttempt
		if d.Status == "cancelled" || d.Status == "canceled" || a.CancelRequested || a.Status == task.AttemptStatusCanceled || a.CanceledAt != nil {
			return "The work has stopped. Your conversation and saved work are preserved. I will wait for your instruction before continuing it."
		}
	}
	return fallback
}

// Re-evaluation is a fresh, tool-free task snapshot, never a replay of an old
// provider process. Cancellation and current execution remain authoritative.
func reviewSnapshotRecoveryAllowed(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.Status == task.TaskStatusDone || d.Status == task.TaskStatusQueued || d.Status == "cancelled" || d.Status == "canceled" {
		return false
	}
	a := d.LatestAttempt
	if a.CancelRequested || a.CanceledAt != nil || a.Status == task.AttemptStatusCanceled || a.Status == task.AttemptStatusRunning || a.Status == task.AttemptStatusQueued {
		return false
	}
	return d.Launch == nil || (d.Launch.State != "queued" && d.Launch.State != "claimed" && d.Launch.State != "cancelled")
}

// Fresh evaluation never manufactures missing or cancelled owner authorization.
func (s *Server) reviewSnapshotRecoveryAuthorized(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if !reviewSnapshotRecoveryAllowed(d) || s.projectStudioRepo == nil || origin.OwnerID == "" || origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || origin.State == "cancelled" || origin.State == "canceled" || origin.State == "cancelling" {
		return false
	}
	message, err := s.projectStudioRepo.GetMessageByID(ctx, origin.MessageID)
	if err != nil || message == nil || message.RoomID != origin.RoomID {
		return false
	}
	if message.AuthorType == "user" {
		return true
	}
	if message.AuthorType != "system" || message.AuthorID != "cuan_auto_delegation" {
		return false
	}
	// Generated phase inputs carry no independent user authority. Recover their
	// review only through the original approval and unchanged accepted lineage.
	_, err = s.phaseApproval(ctx, d, origin.OwnerID, origin.RoomID)
	return err == nil
}

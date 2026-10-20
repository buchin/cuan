package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// A coordinator result is not consent to undo an owner's cancellation. Only a
// later authenticated user turn can request continuation of the existing scope.
func (s *Server) taskContinuationAfterCancel(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil {
		return true
	}
	a := d.LatestAttempt
	if !a.CancelRequested && a.Status != task.AttemptStatusCanceled && a.CanceledAt == nil {
		return true
	}
	stopped := a.CanceledAt
	if stopped == nil {
		stopped = a.FinishedAt
	}
	if stopped == nil || !q.CreatedAt.After(*stopped) {
		return false
	}
	store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return false
	}
	requests, err := store.ListInboxRequests(ctx, q.RoomID)
	if err != nil {
		return false
	}
	accepted := false
	for _, r := range requests {
		if r.RequestID == q.RequestID && r.MessageID == q.MessageID && r.OwnerID == q.OwnerID && r.AgentID == q.AgentID && r.CreatedAt.After(*stopped) && r.State != "cancelled" && r.State != "canceled" && r.State != "cancelling" {
			accepted = true
		}
	}
	if !accepted {
		return false
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
	if err != nil {
		return false
	}
	for _, m := range messages {
		if m.ID == q.MessageID && m.AuthorType == "user" && m.CreatedAt.After(*stopped) {
			return true
		}
	}
	return false
}

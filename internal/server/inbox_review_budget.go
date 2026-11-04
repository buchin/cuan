package server

import (
	"context"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// An accepted owner continuation starts a new bounded correction cycle. Old
// attempts remain evidence, but must not spend a newly authorized cycle's budget.
// A system review or an unrelated owner message cannot reset that cycle.
func (s *Server) reviewBudgetDetail(ctx context.Context, d *task.TaskDetailResponse) *task.TaskDetailResponse {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.CreatedAt.IsZero() || s.taskSvc == nil {
		return d
	}
	reader, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return d
	}
	rooms := []string{d.SourceRoomID}
	if latest, err := s.taskSvc.LatestLaunch(ctx, d.ID); err == nil && latest != nil && latest.RoomID != d.SourceRoomID {
		rooms = append(rooms, latest.RoomID)
	}
	var anchor time.Time
	for _, room := range rooms {
		requests, err := reader.ListInboxRequests(ctx, room)
		if err != nil {
			return d
		}
		for _, q := range requests {
			if q.MessageID == d.SourceMessageID || q.CreatedAt.After(d.LatestAttempt.CreatedAt) || q.State == "cancelled" || q.State == "canceled" || q.State == "cancelling" {
				continue
			}
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
			if err != nil || launch == nil || launch.OwnerID != q.OwnerID || launch.RoomID != q.RoomID || launch.ProjectID != d.ProjectID || launch.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) || launch.Revision == "" || launch.State == "cancelled" || launch.CreatedAt.After(d.LatestAttempt.CreatedAt) || !launch.CreatedAt.After(anchor) {
				continue
			}
			message, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
			if err != nil {
				return d
			}
			if message != nil && message.AuthorType == "user" && message.RoomID == q.RoomID && !message.CreatedAt.After(launch.CreatedAt) {
				anchor = launch.CreatedAt
			}
		}
	}
	if anchor.IsZero() {
		return d
	}
	filtered := *d
	filtered.Attempts = nil
	for _, a := range d.Attempts {
		if a != nil && !a.CreatedAt.Before(anchor) {
			filtered.Attempts = append(filtered.Attempts, a)
		}
	}
	return &filtered
}

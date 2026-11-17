package server

import (
	"context"
	"errors"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Durable execution intent is not sufficient when its underlying owner approval
// was canceled or disappeared. System review turns never create fresh consent.
func (s *Server) authorizeSavedTaskLaunch(ctx context.Context, j task.TaskLaunch, d *task.TaskDetailResponse) (db.InboxRequest, bool, error) {
	var trigger db.InboxRequest
	reader, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return trigger, false, errors.New("Conversation approval is unavailable")
	}
	requests, err := reader.ListInboxRequests(ctx, j.RoomID)
	if err != nil {
		return trigger, false, err
	}
	if d.SourceRoomID != "" && d.SourceRoomID != j.RoomID {
		rows, e := reader.ListInboxRequests(ctx, d.SourceRoomID)
		if e != nil {
			return trigger, false, e
		}
		requests = append(requests, rows...)
	}
	original := false
	for _, q := range requests {
		if q.OwnerID != j.OwnerID {
			continue
		}
		if q.RequestID == j.RequestID {
			trigger = q
		}
		if q.RequestID != j.SourceRequestID || q.MessageID != d.SourceMessageID {
			continue
		}
		if q.State == "cancelled" || q.State == "canceled" || q.State == "cancelling" {
			continue
		}
		message, e := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
		if e != nil {
			return trigger, false, e
		}
		if message != nil && message.AuthorType == "user" && !message.CreatedAt.After(j.CreatedAt) {
			original = true
			if trigger.RequestID == "" {
				trigger = q
			}
		}
	}
	if trigger.RequestID != "" && (trigger.State == "cancelled" || trigger.State == "canceled" || trigger.State == "cancelling") {
		return trigger, false, nil
	}
	if original {
		return trigger, true, nil
	}
	// A fresh accepted owner turn may continue the same approved task, including
	// after a cancellation; taskContinuationAfterCancel checks its timestamp.
	if trigger.RequestID == j.RequestID && trigger.OwnerID == j.OwnerID {
		message, e := s.projectStudioRepo.GetMessageByID(ctx, trigger.MessageID)
		if e != nil {
			return trigger, false, e
		}
		if message != nil && message.AuthorType == "user" && !message.CreatedAt.After(j.CreatedAt) {
			return trigger, true, nil
		}
	}
	authorized, phaseErr := s.authorizeApprovedPhase(ctx, j, d)
	return trigger, authorized, phaseErr
}

func (s *Server) recoverUnstartedInboxTask(ctx context.Context, d *task.TaskDetailResponse, j task.TaskLaunch) bool {
	if d == nil {
		return false
	}
	kind := task.UnstartedLaunchRecoveryKind(j, &d.Task)
	if j.State != "stopped" || kind == "" || !j.ExpiresAt.After(time.Now()) || (d.Status != task.TaskStatusQueued && d.Status != task.TaskStatusBlocked) {
		return false
	}
	if kind == "legacy-agent-routing-v1" {
		if s.agentRepo == nil {
			return false
		}
		resolved, err := s.runtimeAgent(ctx, j.AgentID)
		if err != nil || resolved == nil || resolved.ID != db.AgentIDCuan || !resolved.Enabled {
			return false
		}
	}
	trigger, authorized, err := s.authorizeSavedTaskLaunch(ctx, j, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, trigger, d) {
		return false
	}
	recovered, err := s.taskSvc.RecoverUnstartedLaunch(ctx, j.ID, j.UpdatedAt, time.Now().UTC())
	if err != nil || !recovered {
		return false
	}
	s.publishInbox(j.RoomID)
	if d.SourceRoomID != "" && d.SourceRoomID != j.RoomID {
		s.publishInbox(d.SourceRoomID)
	}
	return true
}

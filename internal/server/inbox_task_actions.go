package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type inboxTaskCancelAction struct {
	TaskID          string `json:"task_id"`
	Reason          string `json:"reason,omitempty"`
	AuthorizationID string `json:"authorization_message_id,omitempty"`
}

type inboxTaskReopenAction struct {
	TaskID          string `json:"task_id"`
	Reason          string `json:"reason,omitempty"`
	AuthorizationID string `json:"authorization_message_id,omitempty"`
}

func parseInboxTaskCancel(content string) (string, *inboxTaskCancelAction, error) {
	const marker = "```cuan-task-cancel\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid task cancellation action.")
	if strings.Count(normalized, marker) != 1 || !strings.HasSuffix(normalized, "```") {
		return "", nil, bad
	}
	raw := normalized[start+len(marker) : len(normalized)-3]
	var a inboxTaskCancelAction
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32000 || decoder.Decode(&a) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	if strings.TrimSpace(a.TaskID) == "" {
		return "", nil, bad
	}
	return strings.TrimSpace(normalized[:start]), &a, nil
}

func parseInboxTaskReopen(content string) (string, *inboxTaskReopenAction, error) {
	const marker = "```cuan-task-reopen\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid task reopen action.")
	if strings.Count(normalized, marker) != 1 || !strings.HasSuffix(normalized, "```") {
		return "", nil, bad
	}
	raw := normalized[start+len(marker) : len(normalized)-3]
	var a inboxTaskReopenAction
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32000 || decoder.Decode(&a) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	if strings.TrimSpace(a.TaskID) == "" {
		return "", nil, bad
	}
	return strings.TrimSpace(normalized[:start]), &a, nil
}

func (s *Server) applyInboxTaskCancel(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, action, err := parseInboxTaskCancel(content)
	if err != nil {
		return err.Error()
	}
	if action == nil {
		return content
	}
	if s.taskSvc == nil {
		return cleanText + "\n\n⚠️ Task cancellation is unavailable: task service is unconfigured."
	}
	author := q.OwnerID
	if author == "" {
		author = "scrum_master"
	}

	// 1. Cancel queued launches
	_, _ = s.taskSvc.CancelLaunches(ctx, action.TaskID, time.Now().UTC())

	// 2. Cancel active running/queued attempt
	if s.terminalManager != nil {
		_ = s.terminalManager.CloseSession(action.TaskID)
	}
	_ = s.taskSvc.FinishTaskLaunches(ctx, action.TaskID, "cancelled")
	s.kickTaskLaunches()
	_, cancelErr := s.taskSvc.CancelTask(ctx, action.TaskID, author)
	if cancelErr != nil {
		// Nothing is running, so there is no attempt to cancel. The task itself is
		// still an owner decision: return it to the queue rather than blocking it.
		backlog := task.TaskStatusBacklog
		note := action.Reason
		if note == "" {
			note = "Cancelled upon user request in chat"
		}
		_, updateErr := s.taskSvc.UpdateTask(ctx, action.TaskID, author, task.UpdateTaskRequest{
			Status:       &backlog,
			CompleteNote: &note,
		})
		if updateErr != nil && !errors.Is(cancelErr, task.ErrInvalidTaskInput) && !errors.Is(cancelErr, task.ErrAttemptNotFound) {
			return cleanText + fmt.Sprintf("\n\n⚠️ Could not cancel task %s: %v", action.TaskID, cancelErr)
		}
	}

	if detail, err := s.taskSvc.GetTaskDetail(ctx, action.TaskID); err == nil && detail != nil {
		if detail.SourceRoomID != "" {
			s.publishInbox(detail.SourceRoomID)
		}
		if q.RoomID != "" && q.RoomID != detail.SourceRoomID {
			s.publishInbox(q.RoomID)
		}
		return cleanText + fmt.Sprintf("\n\n*Task [%s] (%s) has been cancelled.*", detail.ID, detail.Title)
	}
	return cleanText + fmt.Sprintf("\n\n*Task [%s] has been cancelled.*", action.TaskID)
}

func (s *Server) applyInboxTaskReopen(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, action, err := parseInboxTaskReopen(content)
	if err != nil {
		return err.Error()
	}
	if action == nil {
		return content
	}
	if s.taskSvc == nil {
		return cleanText + "\n\n⚠️ Task reopen is unavailable: task service is unconfigured."
	}
	author := q.OwnerID
	if author == "" {
		author = "scrum_master"
	}

	backlogStatus := task.TaskStatusBacklog
	note := action.Reason
	if note == "" {
		note = "Reopened upon user request in chat"
	}
	_, updateErr := s.taskSvc.UpdateTask(ctx, action.TaskID, author, task.UpdateTaskRequest{
		Status:       &backlogStatus,
		CompleteNote: &note,
	})
	if updateErr != nil {
		return cleanText + fmt.Sprintf("\n\n⚠️ Could not reopen task %s: %v", action.TaskID, updateErr)
	}

	if detail, err := s.taskSvc.GetTaskDetail(ctx, action.TaskID); err == nil && detail != nil {
		if detail.SourceRoomID != "" {
			s.publishInbox(detail.SourceRoomID)
		}
		if q.RoomID != "" && q.RoomID != detail.SourceRoomID {
			s.publishInbox(q.RoomID)
		}
		return cleanText + fmt.Sprintf("\n\n*Task [%s] (%s) has been reopened and moved to Backlog.*", detail.ID, detail.Title)
	}
	return cleanText + fmt.Sprintf("\n\n*Task [%s] has been reopened and moved to Backlog.*", action.TaskID)
}

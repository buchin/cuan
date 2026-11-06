package server

import (
	"context"
	"regexp"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type inboxWorkUpdate struct {
	TaskID    string `json:"task_id"`
	AttemptID string `json:"attempt_id,omitempty"`
	Kind      string `json:"kind"` // progress/files live in Detail; result may remain speech.
}

var workLink = regexp.MustCompile(`/tasks/([A-Za-z0-9_-]+)(?:[)\]\s]|$)`)

// Native identities, not the wording of an arbitrary agent message, distinguish
// background work from conversation. Projection never deletes stored messages.
func (s *Server) inboxWorkUpdates(ctx context.Context, room string, messages []db.ProjectMessage) map[string]*inboxWorkUpdate {
	result := map[string]*inboxWorkUpdate{}
	if s.taskSvc == nil {
		return result
	}
	reader, ok := s.taskSvc.Store().(task.InboxWorkRefStore)
	if !ok {
		return result
	}
	refs, err := reader.InboxWorkRefs(ctx, room)
	if err != nil {
		return result
	}
	latest := map[string]task.InboxWorkRef{}
	files := map[string]task.InboxWorkRef{}
	for _, ref := range refs {
		if current, ok := latest[ref.TaskID]; !ok || ref.AttemptNumber > current.AttemptNumber {
			latest[ref.TaskID] = ref
		}
		for _, prefix := range []string{"task-delivery:", "task-visual-delivery:"} {
			files[delegationMessageID(prefix+ref.AttemptID)] = ref
		}
	}
	byID := map[string]db.ProjectMessage{}
	for _, m := range messages {
		byID[m.ID] = m
		if ref, ok := files[m.ID]; ok && m.AuthorType == "agent" && m.ReplyToID != nil && *m.ReplyToID == ref.SourceMessageID {
			result[m.ID] = &inboxWorkUpdate{TaskID: ref.TaskID, AttemptID: ref.AttemptID, Kind: "files"}
		} else if m.AuthorType == "agent" {
			matches := workLink.FindStringSubmatch(m.Content)
			if len(matches) == 2 && strings.TrimSpace(m.Content) == "[View work](/tasks/"+matches[1]+")" {
				if ref, ok := latest[matches[1]]; ok {
					result[m.ID] = &inboxWorkUpdate{TaskID: ref.TaskID, Kind: "progress"}
				}
			}
		}
	}
	if repo, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		requests, err := repo.ListInboxRequests(ctx, room)
		if err != nil {
			return result
		}
		latestResult := map[string]db.InboxRequest{}
		for _, q := range requests {
			if q.RoomID != room {
				continue
			}
			input, ok := byID[q.MessageID]
			if !ok || input.AuthorType != "system" || input.AuthorID != "cuan_work_review" {
				continue
			}
			match := workLink.FindStringSubmatch(input.Content)
			if len(match) != 2 {
				continue
			}
			ref, ok := latest[match[1]]
			if !ok {
				continue
			}
			reply, exists := byID[q.ReplyID]
			if !exists || reply.AuthorType != "agent" {
				continue
			}
			result[q.ReplyID] = &inboxWorkUpdate{TaskID: ref.TaskID, Kind: "progress"}
			if ref.Status == task.TaskStatusDone {
				if old, ok := latestResult[ref.TaskID]; !ok || q.CreatedAt.After(old.CreatedAt) || q.CreatedAt.Equal(old.CreatedAt) && q.RequestID > old.RequestID {
					latestResult[ref.TaskID] = q
				}
			}
		}
		for _, q := range latestResult {
			result[q.ReplyID].Kind = "result"
		}
	}
	return result
}

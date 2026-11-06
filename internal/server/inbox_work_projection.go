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
	// Match application-owned message IDs to durable launches, never keyword-filter
	// ordinary speech. An unresolved failure remains visible; obsolete recovery
	// failures move to Detail only when a current continuation is recorded.
	launchMessages := map[string]string{}
	if launches, ok := s.taskSvc.Store().(task.InboxLaunchRefStore); ok {
		if refs, err := launches.InboxLaunchRefs(ctx, room); err == nil {
			current := map[string]task.InboxLaunchRef{}
			continuing := map[string]bool{}
			for _, l := range refs {
				old, exists := current[l.TaskID]
				if !exists || l.CreatedAt.After(old.CreatedAt) || l.CreatedAt.Equal(old.CreatedAt) && l.ID > old.ID {
					current[l.TaskID] = l
				}
			}
			for id, l := range current {
				continuing[id] = l.State == "queued" || l.State == "claimed" || l.State == "started" && latest[id].Status == task.TaskStatusDoing
			}
			for _, l := range refs {
				launchMessages[delegationMessageID(l.RequestID+":notice")] = l.TaskID
				obsolete := continuing[l.TaskID] || latest[l.TaskID].Status == task.TaskStatusDone
				internalRecovery := l.SafeError == staleInspectionLaunchReason || l.SafeError == supersededRecoveryLaunchReason || l.SafeError == task.InspectionPublicationPreparationError
				if l.State == "started" || l.State == "stopped" && internalRecovery && obsolete {
					launchMessages[delegationMessageID("launch-outcome:"+l.ID+":"+l.State)] = l.TaskID
				}
			}
		}
	}
	byID := map[string]db.ProjectMessage{}
	for _, m := range messages {
		byID[m.ID] = m
		if taskID, ok := launchMessages[m.ID]; ok && m.RoomID == room && (m.AuthorType == "system" || m.AuthorType == "agent") {
			result[m.ID] = &inboxWorkUpdate{TaskID: taskID, Kind: "progress"}
			continue
		}
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

// Native diagnostics are an API-only read projection. They do not become saved
// task activities or inputs to acceptance/authorization evaluation.
func (s *Server) projectInboxLaunchDiagnostics(ctx context.Context, detail *task.TaskDetailResponse) {
	if detail == nil || detail.SourceRoomID == "" {
		return
	}
	reader, ok := s.taskSvc.Store().(task.InboxLaunchRefStore)
	if !ok {
		return
	}
	refs, err := reader.InboxLaunchRefs(ctx, detail.SourceRoomID)
	if err != nil {
		return
	}
	for _, ref := range refs {
		if ref.TaskID != detail.ID {
			continue
		}
		body := "State: " + ref.State
		if ref.SafeError != "" {
			body += "\n" + ref.SafeError
		}
		detail.Activities = append(detail.Activities, &task.TaskActivity{ID: "launch:" + ref.ID, TaskID: detail.ID, ActivityType: "launch_diagnostic", Author: "system", Message: body, CreatedAt: ref.UpdatedAt})
	}
}

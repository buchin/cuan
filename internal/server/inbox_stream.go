package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) handleCancelInboxRequest(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), "")
	store, ok := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if !ok {
		respondJSONError(w, 503, "Storage is not ready")
		return
	}
	id := chi.URLParam(r, "requestID")
	all, err := store.ListInboxRequests(r.Context(), resolved.RoomID)
	if err != nil {
		respondJSONError(w, 503, "Could not read status")
		return
	}
	found := false
	for _, q := range all {
		if q.RequestID == id && (q.OwnerID == "" || q.OwnerID == owner) {
			found = true
		}
	}
	if !found {
		respondJSONError(w, 404, "Request not found")
		return
	}
	_, err = store.CancelInboxRequest(r.Context(), resolved.RoomID, id)
	if err != nil {
		respondJSONError(w, 503, "Cancellation could not be saved")
		return
	}
	if cancel, ok := s.inboxCancels.Load(resolved.RoomID + ":" + id); ok {
		cancel.(context.CancelFunc)()
	}
	s.publishInbox(resolved.RoomID)
	s.kickInboxRoom(resolved.RoomID)
	respondJSON(w, 200, map[string]string{"status": "reconciled"})
}

// Stream snapshots rather than relying on the lossy in-memory event bus as a log.
// Subscribe before the first read; periodic snapshots close lost-event races.
func (s *Server) handleInboxStream(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), "")
	store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		respondJSONError(w, 503, "Storage is not ready")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondJSONError(w, 500, "Streaming is unavailable")
		return
	}
	if s.roomEventBroker == nil {
		respondJSONError(w, 503, "Event broker is not ready")
		return
	}
	ch := s.roomEventBroker.Subscribe(resolved.RoomID)
	defer s.roomEventBroker.Unsubscribe(resolved.RoomID, ch)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	var lastSnapshot []byte
	roomTitle := resolved.RoomTitle
	snapshot := func() bool {
		requests, err := store.ListInboxRequests(r.Context(), resolved.RoomID)
		if err != nil {
			return false
		}
		messages, _, err := s.getFormattedMessagesResultPaged(r.Context(), resolved.RoomID, "", 50, "", !resolved.IsAgent)
		if err != nil {
			return false
		}
		linked, err := s.inboxLinkedTasks(r.Context(), resolved.RoomID, requests)
		if err != nil {
			return false
		}
		if roomTitle == "" {
			if room, rErr := s.projectStudioRepo.GetRoomByID(r.Context(), resolved.RoomID); rErr == nil && room != nil {
				roomTitle = room.Title
			}
		}
		payload := map[string]any{"requests": inboxRecoveryRequests(requests), "messages": messages, "tasks": linked, "room_id": resolved.RoomID, "room_title": roomTitle}
		if notices := s.activeSystemNotices(resolved.RoomID, time.Now().UTC()); len(notices) > 0 {
			payload["system_notices"] = notices
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if bytes.Equal(lastSnapshot, data) {
			_, err = fmt.Fprint(w, ": heartbeat\n\n")
		} else {
			_, err = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", data)
			lastSnapshot = data
		}
		flusher.Flush()
		return err == nil
	}
	s.kickInboxRoom(resolved.RoomID)
	if !snapshot() {
		return
	}
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	end := time.NewTimer(45 * time.Second)
	defer end.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-end.C:
			return
		case <-tick.C:
			if !snapshot() {
				return
			}
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.Type == "chat.delta" || ev.Type == "system.notice" {
				dataBytes, err := json.Marshal(ev.Data)
				if err == nil {
					if _, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(dataBytes)); err != nil {
						return
					}
					flusher.Flush()
				}
			} else {
				if !snapshot() {
					return
				}
			}
		}
	}
}

func (s *Server) inboxLinkedTasks(ctx context.Context, room string, requestSets ...[]db.InboxRequest) ([]*task.TaskItemResponse, error) {
	if s.taskSvc == nil {
		return []*task.TaskItemResponse{}, nil
	}
	items, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: room})
	if items == nil {
		items = []*task.TaskItemResponse{}
	}
	if err != nil || len(items) == 0 {
		return items, err
	}
	var requests []db.InboxRequest
	if len(requestSets) > 0 {
		requests = requestSets[0]
	} else if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		requests, err = store.ListInboxRequests(ctx, room)
		if err != nil {
			return nil, err
		}
	}
	if err := s.projectTaskReviewFindings(ctx, items); err != nil {
		return nil, err
	}
	if err := s.projectInboxTaskReviews(ctx, items, requests, time.Now().UTC()); err != nil {
		return nil, err
	}
	return items, nil
}

// Read-only projection: a live review is not human approval or task completion.
func (s *Server) projectInboxTaskReviews(ctx context.Context, items []*task.TaskItemResponse, requests []db.InboxRequest, now time.Time) error {
	for _, item := range items {
		if item != nil {
			item.ReviewState = ""
			a := item.LatestAttempt
			if a != nil && (a.Status == task.AttemptStatusRunning || a.Status == task.AttemptStatusQueued) && item.ExecutionReviewState == a.Status && item.Status != task.TaskStatusDone && item.Status != "cancelled" && item.Status != "canceled" && !a.CancelRequested && a.CanceledAt == nil {
				item.ReviewState = item.ExecutionReviewState
			}
		}
	}
	for _, q := range requests {
		if (q.State != "queued" && q.State != "running") || !q.Deadline.After(now) || (q.State == "running" && q.StartedAt == nil) {
			continue
		}
		message, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
		if err != nil {
			return err
		}
		if message == nil || message.RoomID != q.RoomID || message.AuthorType != "system" || message.AuthorID != "cuan_work_review" {
			continue
		}
		for _, item := range items {
			if item == nil || item.LatestAttempt == nil || item.Status == task.TaskStatusDone || item.Status == "cancelled" || item.Status == "canceled" || item.LatestAttempt.Status == task.AttemptStatusCanceled || item.LatestAttempt.CancelRequested || item.LatestAttempt.Status == task.AttemptStatusRunning || item.LatestAttempt.Status == task.AttemptStatusQueued {
				continue
			}
			matches := strings.Contains(message.Content, "/tasks/"+item.ID+")") && (q.RequestID == delegationMessageID("task-review:"+item.LatestAttempt.ID) || strings.Contains(message.Content, "reported attempt: "+item.LatestAttempt.ID+"."))
			if matches && (item.ReviewState != "running" || q.State == "running") {
				item.ReviewState = q.State
			}
		}
	}
	return nil
}

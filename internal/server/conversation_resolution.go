package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

// Resolution is shared room state. It never marks work Done or deletes evidence.
func (s *Server) handleConversationResolution(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	if s.projectStudioRepo == nil || s.taskSvc == nil {
		respondJSONError(w, 503, "Conversations are unavailable")
		return
	}
	var req struct {
		Resolved   *bool      `json:"resolved"`
		ObservedAt *time.Time `json:"observed_at"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || req.Resolved == nil {
		respondJSONError(w, 400, "Choose whether to resolve this conversation")
		return
	}
	id := chi.URLParam(r, "id")
	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), id)
	if err != nil {
		respondJSONError(w, 404, "Conversation not found")
		return
	}
	var at *time.Time
	if *req.Resolved {
		now := time.Now().UTC()
		if req.ObservedAt == nil || req.ObservedAt.IsZero() || req.ObservedAt.After(now) {
			respondJSONError(w, 400, "Review the latest conversation before resolving it")
			return
		}
		reason, err := s.conversationResolutionBlocker(r.Context(), id)
		if err != nil {
			respondJSONError(w, 503, "Could not check conversation work")
			return
		}
		if reason != "" {
			respondJSONError(w, 409, reason)
			return
		}

		at = req.ObservedAt
	}
	if err = s.projectStudioRepo.SetRoomResolved(r.Context(), room.ID, at); err != nil {
		if errors.Is(err, db.ErrConversationChanged) {
			respondJSONError(w, 409, "New messages arrived. Review them before resolving this conversation.")
			return
		}
		respondJSONError(w, 503, "Could not save conversation status")
		return
	}
	s.publishInbox(room.ID)
	if at != nil {
		s.requestStorageMaintenance()
	}
	respondJSON(w, 200, map[string]any{"id": room.ID, "resolved": *req.Resolved, "resolved_at": at})
}

// Read all attempts and admitted launches, not just the newest projection. A
// failed observation is never evidence that it is safe to discard workspaces.
func (s *Server) conversationResolutionBlocker(ctx context.Context, id string) (string, error) {
	items, err := s.taskSvc.Store().ListTasks(ctx, task.TaskFilter{SourceRoomID: id})
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.ID] = true
		if item.Status != "done" && item.Status != "cancelled" && item.Status != "canceled" {
			return "Finish or cancel the remaining tasks before resolving this conversation", nil
		}
		attempts, err := s.taskSvc.Store().ListAttempts(ctx, item.ID)
		if err != nil {
			return "", err
		}
		for _, attempt := range attempts {
			if attempt.Status == "queued" || attempt.Status == "running" {
				return "Wait for the current work before resolving this conversation", nil
			}
		}
	}
	legacy, err := s.projectStudioRepo.ListTasksByRoomID(ctx, id)
	if err != nil {
		return "", err
	}
	for _, item := range legacy {
		if !seen[item.ID] && item.Status != "done" && item.Status != "cancelled" && item.Status != "canceled" {
			return "Finish or cancel the remaining tasks before resolving this conversation", nil
		}
	}
	if store, ok := s.taskSvc.Store().(task.InboxLaunchRefStore); ok {
		launches, err := store.InboxLaunchRefs(ctx, id)
		if err != nil {
			return "", err
		}
		for _, launch := range launches {
			if launch.State == "queued" || launch.State == "running" {
				return "Wait for the current work before resolving this conversation", nil
			}
		}
	} else {
		return "", errors.New("launch observation unavailable")
	}
	if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		requests, err := store.ListInboxRequests(ctx, id)
		if err != nil {
			return "", err
		}
		for _, request := range requests {
			if request.State == "queued" || request.State == "running" || request.State == "cancelling" {
				return "Wait for the current reply before resolving this conversation", nil
			}
		}
	} else {
		return "", errors.New("reply observation unavailable")
	}
	return "", nil
}

package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
)

type inboxRecoveryDTO struct {
	db.InboxRequest
	RecoveryAction string `json:"recovery_action,omitempty"`
}

func inboxRecoveryRequests(requests []db.InboxRequest) []inboxRecoveryDTO {
	out := make([]inboxRecoveryDTO, 0, len(requests))
	// Show the prepared recovery during the short backoff, before the dispatcher
	// atomically queues its child. Only freshly opted-in, proven-safe failures
	// can expose this state; historical/unknown outcomes cannot.
	pending := db.InboxAutomaticRecoveryCandidate(requests, time.Now().Add(2*time.Second))
	for _, q := range requests {
		action := ""
		if q.OwnerID != "" && q.AgentID != "" {
			action = db.InboxRecoveryAction(q)
			if pending != nil && pending.RequestID == q.RequestID {
				action = "recovering"
			}
		}
		out = append(out, inboxRecoveryDTO{InboxRequest: q, RecoveryAction: action})
	}
	return out
}

func (s *Server) handleRetryInboxRequest(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), "")
	store, ok := s.projectStudioRepo.(db.InboxRetryRepository)
	reader, canRead := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok || !canRead {
		respondJSONError(w, 503, "Chat recovery is unavailable")
		return
	}
	source := chi.URLParam(r, "requestID")
	requests, err := reader.ListInboxRequests(r.Context(), resolved.RoomID)
	if err != nil {
		respondJSONError(w, 503, "Could not check the previous attempt")
		return
	}
	found := false
	for _, q := range requests {
		if q.RequestID == source && q.OwnerID == owner {
			found = true
		}
	}
	if !found {
		respondJSONError(w, 404, "Request not found")
		return
	}
	next, fresh, err := store.RetryInboxRequest(r.Context(), resolved.RoomID, source, owner)
	if errors.Is(err, db.ErrInboxRetryUnavailable) {
		respondJSONError(w, 409, "This attempt cannot be safely repeated. Check its status and details.")
		return
	}
	if errors.Is(err, db.ErrInboxQueueFull) {
		respondJSONError(w, 429, "This conversation has too many pending messages. Wait for them to finish.")
		return
	}
	if err != nil {
		respondJSONError(w, 503, "Could not save the retry. Check status before trying again.")
		return
	}
	s.publishInbox(resolved.RoomID)
	s.kickInboxRoom(resolved.RoomID)
	status := http.StatusOK
	if fresh {
		status = http.StatusAccepted
	}
	respondJSON(w, status, map[string]any{"request": inboxRecoveryDTO{InboxRequest: next, RecoveryAction: db.InboxRecoveryAction(next)}})
}

package server

import (
	"crypto/subtle"
	"encoding/json"
	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s *Server) handleOwnerChatReceipts(w http.ResponseWriter, r *http.Request) {
	s.handleChatReceipts(w, r, true)
}
func (s *Server) handleChatReceipts(w http.ResponseWriter, r *http.Request, owner bool) {
	var in struct {
		SessionUUID  string   `json:"session_uuid"`
		GuestUUID    string   `json:"guest_uuid"`
		DeliveredIDs []string `json:"delivered_ids"`
		ReadIDs      []string `json:"read_ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&in); err != nil || len(in.DeliveredIDs) > 500 || len(in.ReadIDs) > 500 {
		respondJSONError(w, 400, "Invalid receipts")
		return
	}
	room := in.SessionUUID
	if owner {
		if s.accountOwner(w, r) == "" {
			return
		}
		room = chi.URLParam(r, "id")
	} else {
		sess, err := s.crmSession(r.Context(), room)
		if err != nil || sess == nil || in.GuestUUID == "" || subtle.ConstantTimeCompare([]byte(sess.GuestUUID), []byte(in.GuestUUID)) != 1 {
			respondJSONError(w, 403, "Session not available")
			return
		}
	}
	if !s.isCustomerRoom(r.Context(), room) {
		respondJSONError(w, 404, "Conversation not found")
		return
	}
	store, ok := s.projectStudioRepo.(db.ChatReceiptRepository)
	if !ok {
		respondJSONError(w, 503, "Receipts unavailable")
		return
	}
	messages, err := s.getFormattedMessagesResult(r.Context(), room, "")
	if err != nil {
		respondJSONError(w, 503, "Receipts unavailable")
		return
	}
	eligible := map[string]bool{}
	for _, m := range messages {
		if !m.IsInternal && ((owner && m.SenderType == "visitor") || (!owner && m.SenderType == "operator")) {
			eligible[m.UUID] = true
		}
	}
	filter := func(ids []string) []string {
		out := []string{}
		seen := map[string]bool{}
		for _, id := range ids {
			if eligible[id] && !seen[id] {
				out = append(out, id)
				seen[id] = true
			}
		}
		return out
	}
	if err = store.AcknowledgeMessages(r.Context(), room, filter(in.DeliveredIDs), false); err == nil {
		err = store.AcknowledgeMessages(r.Context(), room, filter(in.ReadIDs), true)
	}
	if err != nil {
		respondJSONError(w, 503, "Could not save receipts")
		return
	}
	s.publishInbox(room)
	respondJSON(w, 200, map[string]any{"ok": true})
}

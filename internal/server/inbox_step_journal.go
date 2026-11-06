package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleInboxRequestSteps(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), "")
	repo, ok := s.projectStudioRepo.(db.InboxStepJournalRepository)
	if !ok {
		respondJSONError(w, 503, "Storage is not ready")
		return
	}
	after, limit := 0, 100
	var err error
	if v := r.URL.Query().Get("after_seq"); v != "" {
		after, err = strconv.Atoi(v)
		if err != nil || after < 0 {
			respondJSONError(w, 400, "Invalid after_seq")
			return
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > 200 {
			respondJSONError(w, 400, "Limit must be between 1 and 200")
			return
		}
	}
	page, err := repo.ListInboxSteps(r.Context(), resolved.RoomID, chi.URLParam(r, "requestID"), owner, after, limit)
	if errors.Is(err, sql.ErrNoRows) {
		respondJSONError(w, 404, "Request not found")
		return
	}
	if err != nil {
		respondJSONError(w, 503, "Could not read response details")
		return
	}
	for i := range page.Steps {
		page.Steps[i].Text = visibleInboxToolText(page.Steps[i].Kind, page.Steps[i].Text)
	}
	respondJSON(w, 200, page)
}

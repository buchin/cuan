package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	sdk "github.com/buchin/cuan/pkg/extensions"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/db"
)

// Manual contacts are records, not connected live-chat recipients. The key is
// private to the owner; visitor initialization may not claim the manual namespace.
func (s *Server) handleCRMCreate(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	var req struct {
		RequestID string `json:"request_id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || !db.IsValidUUID(req.RequestID) {
		respondJSONError(w, 400, "Invalid contact request")
		return
	}
	draft := struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}{req.Name, req.Email}
	if err := s.prepareExtension(r.Context(), "contact.before_save", sdk.Scope{ResourceID: req.RequestID}, draft, &draft); err != nil {
		respondJSONError(w, 422, "An extension could not validate the contact")
		return
	}
	req.Name, req.Email = draft.Name, draft.Email
	req.RequestID = strings.ToLower(req.RequestID)
	req.Name = strings.TrimSpace(req.Name)
	email, err := normalizeCRMEmail(req.Email)
	if err != nil || req.Name == "" || utf8.RuneCountInString(req.Name) > 120 || strings.ContainsAny(req.Name, "\r\n\x00") {
		respondJSONError(w, 400, "Enter a name of up to 120 characters and a valid email if provided")
		return
	}
	now := time.Now().UTC()
	c := CRMContact{ID: req.RequestID, GuestUUID: "manual_" + req.RequestID, SessionUUID: db.GenerateUUID(), Name: req.Name, Email: email, Product: "Manual", Tags: []string{}, CreatedAt: now, UpdatedAt: now, IsManual: true}
	created := true
	if s.db != nil {
		c, err = scanCRM(s.db.QueryRowContext(r.Context(), `INSERT INTO crm_contacts(id,product,guest_uuid,session_uuid,name,email) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(product,guest_uuid) DO NOTHING RETURNING `+crmColumns, c.ID, c.Product, c.GuestUUID, c.SessionUUID, c.Name, c.Email))
		if errors.Is(err, sql.ErrNoRows) {
			created = false
			c, err = scanCRM(s.db.QueryRowContext(r.Context(), `SELECT `+crmColumns+` FROM crm_contacts WHERE product='Manual' AND guest_uuid=$1`, "manual_"+req.RequestID))
		}
	} else {
		s.crm.Lock()
		key := "Manual\x00" + c.GuestUUID
		if existing, ok := s.crm.contacts[key]; ok {
			c = existing
			created = false
		} else {
			s.crm.contacts[key] = c
		}
		s.crm.Unlock()
	}
	if err != nil {
		respondJSONError(w, 503, "Could not save the contact. Try again from the same form")
		return
	}
	if c.Name != req.Name || c.Email != email {
		respondJSONError(w, 409, "This request has already been used. Reload Contacts to view the saved contact")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		s.publishExtension(r.Context(), "contact.created", c.ID, "", c.CreatedAt)
		s.publishExtension(r.Context(), "contact.saved", c.ID, "", c.UpdatedAt)
	}
	respondJSON(w, status, map[string]any{"contact": c})
}

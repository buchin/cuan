package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
)

type ContactMemory struct {
	ID              string     `json:"id"`
	ContactID       string     `json:"contact_id"`
	Kind            string     `json:"kind"`
	Body            string     `json:"body"`
	RecordedBy      string     `json:"recorded_by"`
	SourceRoomID    string     `json:"source_room_id"`
	SourceMessageID string     `json:"source_message_id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"-"`
}

const memoryColumns = `id,contact_id,kind,body,recorded_by,source_room_id,source_message_id,created_at,updated_at,deleted_at`

func scanMemory(row interface{ Scan(...any) error }) (ContactMemory, error) {
	var m ContactMemory
	err := row.Scan(&m.ID, &m.ContactID, &m.Kind, &m.Body, &m.RecordedBy, &m.SourceRoomID, &m.SourceMessageID, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt)
	return m, err
}
func validateMemory(kind, body string) error {
	if kind != "note" && kind != "fact" && kind != "birthday" {
		return errors.New("Choose a note, fact, or birthday")
	}
	if n := utf8.RuneCountInString(body); n < 1 || n > 6000 {
		return errors.New("Use 1–6000 characters")
	}
	if kind == "birthday" {
		format := "2006-01-02"
		value := body
		if len(body) == 5 {
			format = "2006-01-02"
			value = "2000-" + body
		}
		if _, err := time.Parse(format, value); err != nil {
			return errors.New("Use MM-DD or YYYY-MM-DD for a birthday")
		}
	}
	return nil
}
func (s *Server) crmFind(ctx context.Context, id string) (CRMContact, error) {
	if s.db != nil {
		return scanCRM(s.db.QueryRowContext(ctx, `SELECT `+crmColumns+` FROM crm_contacts WHERE id::text=$1`, id))
	}
	s.crm.Lock()
	defer s.crm.Unlock()
	for _, c := range s.crm.contacts {
		if c.ID == id {
			return c, nil
		}
	}
	return CRMContact{}, sql.ErrNoRows
}
func (s *Server) memoryList(ctx context.Context, contact, query string, offset int) ([]ContactMemory, error) {
	out := []ContactMemory{}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT `+memoryColumns+` FROM contact_memory WHERE contact_id::text=$1 AND deleted_at IS NULL AND strpos(lower(body),lower($2))>0 ORDER BY created_at DESC,id DESC LIMIT 51 OFFSET $3`, contact, query, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			m, e := scanMemory(rows)
			if e != nil {
				return nil, e
			}
			out = append(out, m)
		}
		return out, rows.Err()
	}
	s.crm.Lock()
	defer s.crm.Unlock()
	for _, m := range s.crm.memory {
		if m.ContactID == contact && m.DeletedAt == nil && strings.Contains(strings.ToLower(m.Body), strings.ToLower(query)) {
			out = append(out, m)
		}
	}
	sortMemory(out)
	if offset >= len(out) {
		return []ContactMemory{}, nil
	}
	out = out[offset:]
	if len(out) > 51 {
		out = out[:51]
	}
	return out, nil
}
func (s *Server) memorySave(ctx context.Context, m ContactMemory) (ContactMemory, error) {
	if err := ctx.Err(); err != nil {
		return ContactMemory{}, err
	}
	if s.db != nil {
		_, err := s.db.ExecContext(ctx, `INSERT INTO contact_memory(id,contact_id,kind,body,recorded_by,source_room_id,source_message_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8) ON CONFLICT(id) DO NOTHING`, m.ID, m.ContactID, m.Kind, m.Body, m.RecordedBy, m.SourceRoomID, m.SourceMessageID, m.CreatedAt)
		if err != nil {
			return ContactMemory{}, err
		}
		old, err := scanMemory(s.db.QueryRowContext(ctx, `SELECT `+memoryColumns+` FROM contact_memory WHERE id=$1`, m.ID))
		if err == nil && (old.ContactID != m.ContactID || old.Kind != m.Kind || old.Body != m.Body || old.RecordedBy != m.RecordedBy || old.SourceRoomID != m.SourceRoomID || old.SourceMessageID != m.SourceMessageID || old.DeletedAt != nil) {
			return ContactMemory{}, errors.New("conflict")
		}
		return old, err
	}
	s.crm.Lock()
	defer s.crm.Unlock()
	if s.crm.memory == nil {
		s.crm.memory = map[string]ContactMemory{}
	}
	if old, ok := s.crm.memory[m.ID]; ok {
		if old.ContactID != m.ContactID || old.Kind != m.Kind || old.Body != m.Body || old.RecordedBy != m.RecordedBy || old.SourceRoomID != m.SourceRoomID || old.SourceMessageID != m.SourceMessageID || old.DeletedAt != nil {
			return ContactMemory{}, errors.New("conflict")
		}
		return old, nil
	}
	s.crm.memory[m.ID] = m
	return m, nil
}
func (s *Server) handleContactMemory(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	contact := chi.URLParam(r, "id")
	if _, err := s.crmFind(r.Context(), contact); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondJSONError(w, 404, "Contact not found")
		} else {
			respondJSONError(w, 503, "Could not load contact")
		}
		return
	}
	if r.Method == "GET" {
		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if r.URL.Query().Get("offset") == "" {
			offset = 0
			err = nil
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if err != nil || offset < 0 || offset > 100000 || len(query) > 300 {
			respondJSONError(w, 400, "Invalid search or page")
			return
		}
		items, err := s.memoryList(r.Context(), contact, query, offset)
		if err != nil {
			respondJSONError(w, 503, "Could not load notes")
			return
		}
		more := len(items) > 50
		if more {
			items = items[:50]
		}
		respondJSON(w, 200, map[string]any{"items": items, "has_more": more})
		return
	}
	var p struct {
		ID   string `json:"request_id"`
		Kind string `json:"kind"`
		Body string `json:"body"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 80000))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&p) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondJSONError(w, 400, "Invalid note")
		return
	}
	p.Body = strings.TrimSpace(p.Body)
	if err := validateMemory(p.Kind, p.Body); err != nil {
		respondJSONError(w, 400, err.Error())
		return
	}
	if r.Method == "POST" {
		if !db.IsValidUUID(p.ID) {
			respondJSONError(w, 400, "A valid request ID is required")
			return
		}
		now := time.Now().UTC()
		m, err := s.memorySave(r.Context(), ContactMemory{ID: p.ID, ContactID: contact, Kind: p.Kind, Body: p.Body, RecordedBy: "You", CreatedAt: now, UpdatedAt: now})
		if err != nil {
			if err.Error() == "conflict" {
				respondJSONError(w, 409, "This request ID was already used. Refresh before retrying.")
			} else {
				respondJSONError(w, 503, "Could not save note")
			}
			return
		}
		respondJSON(w, 201, m)
		return
	}
	respondJSONError(w, 405, "Unsupported method")
}
func (s *Server) handleContactMemoryChange(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	contact, id := chi.URLParam(r, "id"), chi.URLParam(r, "memoryID")
	var kind, body string
	if r.Method == "PATCH" {
		var p struct {
			Kind string `json:"kind"`
			Body string `json:"body"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 80000))
		d.DisallowUnknownFields()
		if d.Decode(&p) != nil || d.Decode(&struct{}{}) != io.EOF {
			respondJSONError(w, 400, "Invalid note")
			return
		}
		kind, body = p.Kind, strings.TrimSpace(p.Body)
		if err := validateMemory(kind, body); err != nil {
			respondJSONError(w, 400, err.Error())
			return
		}
	}
	now := time.Now().UTC()
	found := false
	if s.db != nil {
		query := `UPDATE contact_memory SET deleted_at=$1,updated_at=$1,body='[deleted]',source_room_id='',source_message_id='' WHERE id::text=$2 AND contact_id::text=$3 AND deleted_at IS NULL`
		args := []any{now, id, contact}
		if r.Method == "PATCH" {
			query = `UPDATE contact_memory SET updated_at=$1,body=$4,kind=$5 WHERE id::text=$2 AND contact_id::text=$3 AND deleted_at IS NULL`
			args = append(args, body, kind)
		}
		res, err := s.db.ExecContext(r.Context(), query, args...)
		if err != nil {
			respondJSONError(w, 503, "Could not update note")
			return
		}
		n, _ := res.RowsAffected()
		found = n > 0
	} else {
		s.crm.Lock()
		m, ok := s.crm.memory[id]
		if ok && m.ContactID == contact && m.DeletedAt == nil {
			found = true
			m.UpdatedAt = now
			if r.Method == "DELETE" {
				m.DeletedAt = &now
				m.Body = ""
			} else {
				m.Kind = kind
				m.Body = body
			}
			s.crm.memory[id] = m
		}
		s.crm.Unlock()
	}
	if !found {
		respondJSONError(w, 404, "Note not found")
		return
	}
	respondJSON(w, 200, map[string]bool{"ok": true})
}

func sortMemory(items []ContactMemory) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}

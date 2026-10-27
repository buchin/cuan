package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	sdk "github.com/buchin/cuan/pkg/extensions"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
)

type CRMContact struct {
	SearchNote  *CRMSearchNote `json:"search_note,omitempty"`
	IsManual    bool           `json:"is_manual"`
	ID          string         `json:"id"`
	Product     string         `json:"product"`
	GuestUUID   string         `json:"guest_uuid"`
	SessionUUID string         `json:"conversation_id"`
	Name        string         `json:"name"`
	Email       string         `json:"email"`
	Tags        []string       `json:"tags"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type crmStore struct {
	sync.Mutex
	contacts map[string]CRMContact
	memory   map[string]ContactMemory
}

func scanCRM(row interface{ Scan(...any) error }) (CRMContact, error) {
	var c CRMContact
	var tags []byte
	err := row.Scan(&c.ID, &c.Product, &c.GuestUUID, &c.SessionUUID, &c.Name, &c.Email, &tags, &c.CreatedAt, &c.UpdatedAt)
	if err == nil {
		err = json.Unmarshal(tags, &c.Tags)
		c.IsManual = strings.HasPrefix(c.GuestUUID, "manual_")
	}
	return c, err
}

const crmColumns = `id,product,guest_uuid,session_uuid,name,email,tags,created_at,updated_at`

func (s *Server) crmUpsertWithSession(ctx context.Context, guest, product, name, email, sessionUUID string) (contact CRMContact, resultErr error) {
	if sessionUUID == "" {
		sessionUUID = db.GenerateUUID()
	}
	draft := struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}{name, email}
	if err := s.prepareExtension(ctx, "contact.before_save", sdk.Scope{}, draft, &draft); err != nil {
		return CRMContact{}, err
	}
	name, email = strings.TrimSpace(draft.Name), draft.Email
	var normalizeErr error
	email, normalizeErr = normalizeCRMEmail(email)
	if normalizeErr != nil || utf8.RuneCountInString(name) > 120 || strings.ContainsAny(name, "\r\n\x00") {
		return CRMContact{}, errors.New("invalid contact identity")
	}
	defer func() {
		if resultErr == nil {
			s.publishExtension(ctx, "contact.saved", contact.ID, "", contact.UpdatedAt)
		}
	}()
	c := CRMContact{ID: db.GenerateUUID(), Product: product, GuestUUID: guest, SessionUUID: sessionUUID, Name: name, Email: email, Tags: []string{}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if s.db != nil {
		return scanCRM(s.db.QueryRowContext(ctx, `INSERT INTO crm_contacts(id,product,guest_uuid,session_uuid,name,email) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(product,guest_uuid) DO UPDATE SET name=CASE WHEN $5='' THEN crm_contacts.name ELSE $5 END,email=CASE WHEN $6='' THEN crm_contacts.email ELSE $6 END,updated_at=NOW() RETURNING `+crmColumns, c.ID, product, guest, c.SessionUUID, name, email))
	}
	s.crm.Lock()
	defer s.crm.Unlock()
	key := product + "\x00" + guest
	if old, ok := s.crm.contacts[key]; ok {
		c = old
		if name != "" {
			c.Name = name
		}
		if email != "" {
			c.Email = email
		}
		c.UpdatedAt = time.Now().UTC()
	}
	s.crm.contacts[key] = c
	return c, nil
}

func (s *Server) crmUpsert(ctx context.Context, guest, product, name, email string) (contact CRMContact, resultErr error) {
	return s.crmUpsertWithSession(ctx, guest, product, name, email, db.GenerateUUID())
}

func (s *Server) crmFindByGuest(ctx context.Context, product, guest string) (CRMContact, error) {
	if s.db != nil {
		return scanCRM(s.db.QueryRowContext(ctx, `SELECT `+crmColumns+` FROM crm_contacts WHERE product=$1 AND guest_uuid=$2`, product, guest))
	}
	s.crm.Lock()
	defer s.crm.Unlock()
	key := product + "\x00" + guest
	if c, ok := s.crm.contacts[key]; ok {
		return c, nil
	}
	return CRMContact{}, sql.ErrNoRows
}

func (s *Server) hasMessages(roomID string) bool {
	if s.projectStudioRepo != nil {
		msgs, err := s.projectStudioRepo.ListMessagesByRoomID(context.Background(), roomID)
		return err == nil && len(msgs) > 0
	}
	return false
}

func (s *Server) crmList(ctx context.Context) ([]CRMContact, error) {
	out := []CRMContact{}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT `+crmColumns+` FROM crm_contacts c
WHERE (c.name != '' OR c.email != '' OR EXISTS (SELECT 1 FROM project_messages m WHERE m.room_id = c.session_uuid))
ORDER BY c.updated_at DESC`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			c, e := scanCRM(rows)
			if e != nil {
				return nil, e
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}
	s.crm.Lock()
	defer s.crm.Unlock()
	for _, c := range s.crm.contacts {
		if c.Name != "" || c.Email != "" || s.hasMessages(c.SessionUUID) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Server) crmSession(ctx context.Context, id string) (*LiveChatSession, error) {
	var c CRMContact
	found := false
	if s.db != nil {
		var err error
		c, err = scanCRM(s.db.QueryRowContext(ctx, `SELECT `+crmColumns+` FROM crm_contacts WHERE session_uuid::text=$1`, id))
		if err == nil {
			found = true
		}
	} else {
		s.crm.Lock()
		for _, v := range s.crm.contacts {
			if v.SessionUUID == id {
				c = v
				found = true
				break
			}
		}
		s.crm.Unlock()
	}
	if found {
		if c.Name == "" {
			c.Name = "Visitor #" + c.ID[:6]
		}
		return &LiveChatSession{UUID: c.SessionUUID, GuestUUID: c.GuestUUID, Product: c.Product, Name: c.Name, Email: c.Email, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, Status: "offline"}, nil
	}

	// Fallback to active transient session in globalChatManager
	globalChatManager.RLock()
	live, ok := globalChatManager.sessions[id]
	globalChatManager.RUnlock()
	if ok && live != nil {
		return live, nil
	}

	return nil, sql.ErrNoRows
}
func normalizeCRMEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != value || len(value) > 254 {
		return "", errors.New("Invalid email")
	}
	return strings.ToLower(value), nil
}
func (s *Server) handleCRMList(w http.ResponseWriter, r *http.Request) {
	contacts, err := s.crmList(r.Context())
	if err != nil {
		respondJSONError(w, 503, "Could not load contacts")
		return
	}
	respondJSON(w, 200, map[string]any{"contacts": contacts})
}
func (s *Server) handleCRMUpdate(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Tags []string `json:"tags"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&p) != nil || len(p.Tags) > 20 {
		respondJSONError(w, 400, "Up to 20 tags are allowed")
		return
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, tag := range p.Tags {
		tag = strings.TrimSpace(tag)
		if len(tag) == 0 || utf8.RuneCountInString(tag) > 40 {
			respondJSONError(w, 400, "Tags must contain 1–40 characters")
			return
		}
		if !seen[strings.ToLower(tag)] {
			tags = append(tags, tag)
			seen[strings.ToLower(tag)] = true
		}
	}
	id := chi.URLParam(r, "id")
	if s.db != nil {
		encoded, _ := json.Marshal(tags)
		res, err := s.db.ExecContext(r.Context(), `UPDATE crm_contacts SET tags=$1,updated_at=NOW() WHERE id::text=$2`, string(encoded), id)
		if err != nil {
			respondJSONError(w, 503, "Tags could not be saved")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			respondJSONError(w, 404, "Contact not found")
			return
		}
	} else {
		s.crm.Lock()
		found := false
		for key, c := range s.crm.contacts {
			if c.ID == id {
				c.Tags = tags
				c.UpdatedAt = time.Now().UTC()
				s.crm.contacts[key] = c
				found = true
				break
			}
		}
		s.crm.Unlock()
		if !found {
			respondJSONError(w, 404, "Contact not found")
			return
		}
	}
	s.publishExtension(r.Context(), "contact.updated", id, "", time.Now().UTC())
	respondJSON(w, 200, map[string]any{"tags": tags})
}

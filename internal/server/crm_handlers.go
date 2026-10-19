package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	ID          string    `json:"id"`
	Product     string    `json:"product"`
	GuestUUID   string    `json:"guest_uuid"`
	SessionUUID string    `json:"conversation_id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type crmStore struct {
	sync.Mutex
	contacts map[string]CRMContact
}

func scanCRM(row interface{ Scan(...any) error }) (CRMContact, error) {
	var c CRMContact
	var tags []byte
	err := row.Scan(&c.ID, &c.Product, &c.GuestUUID, &c.SessionUUID, &c.Name, &c.Email, &tags, &c.CreatedAt, &c.UpdatedAt)
	if err == nil {
		err = json.Unmarshal(tags, &c.Tags)
	}
	return c, err
}

const crmColumns = `id,product,guest_uuid,session_uuid,name,email,tags,created_at,updated_at`

func (s *Server) crmUpsert(ctx context.Context, guest, product, name, email string) (CRMContact, error) {
	c := CRMContact{ID: db.GenerateUUID(), Product: product, GuestUUID: guest, SessionUUID: db.GenerateUUID(), Name: name, Email: email, Tags: []string{}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
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
func (s *Server) crmList(ctx context.Context) ([]CRMContact, error) {
	out := []CRMContact{}
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT `+crmColumns+` FROM crm_contacts ORDER BY updated_at DESC`)
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
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Server) crmSession(ctx context.Context, id string) (*LiveChatSession, error) {
	var c CRMContact
	if s.db != nil {
		var err error
		c, err = scanCRM(s.db.QueryRowContext(ctx, `SELECT `+crmColumns+` FROM crm_contacts WHERE session_uuid::text=$1`, id))
		if err != nil {
			return nil, err
		}
	} else {
		s.crm.Lock()
		defer s.crm.Unlock()
		found := false
		for _, v := range s.crm.contacts {
			if v.SessionUUID == id {
				c = v
				found = true
				break
			}
		}
		if !found {
			return nil, sql.ErrNoRows
		}
	}
	if c.Name == "" {
		c.Name = "Visitor #" + c.ID[:6]
	}
	return &LiveChatSession{UUID: c.SessionUUID, GuestUUID: c.GuestUUID, Product: c.Product, Name: c.Name, Email: c.Email, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, Status: "offline"}, nil
}
func normalizeCRMEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != value || len(value) > 254 {
		return "", errors.New("email tidak valid")
	}
	return strings.ToLower(value), nil
}
func (s *Server) handleCRMList(w http.ResponseWriter, r *http.Request) {
	contacts, err := s.crmList(r.Context())
	if err != nil {
		respondJSONError(w, 503, "Kontak belum dapat dimuat")
		return
	}
	respondJSON(w, 200, map[string]any{"contacts": contacts})
}
func (s *Server) handleCRMUpdate(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Tags []string `json:"tags"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&p) != nil || len(p.Tags) > 20 {
		respondJSONError(w, 400, "Maksimal 20 tag")
		return
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, tag := range p.Tags {
		tag = strings.TrimSpace(tag)
		if len(tag) == 0 || utf8.RuneCountInString(tag) > 40 {
			respondJSONError(w, 400, "Panjang tag harus 1–40 karakter")
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
			respondJSONError(w, 503, "Tag belum tersimpan")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			respondJSONError(w, 404, "Kontak tidak ditemukan")
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
			respondJSONError(w, 404, "Kontak tidak ditemukan")
			return
		}
	}
	respondJSON(w, 200, map[string]any{"tags": tags})
}

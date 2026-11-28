package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
)

// CannedResponse represents a saved quick reply template for operators.
type CannedResponse struct {
	ID        string    `json:"id"`
	Shortcut  string    `json:"shortcut"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cannedStore struct {
	sync.RWMutex
	items map[string]CannedResponse
}

const cannedColumns = `id, shortcut, title, content, category, created_at, updated_at`

func scanCannedResponse(row interface{ Scan(...any) error }) (CannedResponse, error) {
	var c CannedResponse
	err := row.Scan(&c.ID, &c.Shortcut, &c.Title, &c.Content, &c.Category, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func defaultCannedResponses() []CannedResponse {
	now := time.Now().UTC()
	return []CannedResponse{
		{
			ID:        "00000000-0000-4000-a000-000000000001",
			Shortcut:  "halo",
			Title:     "Salam Pembuka",
			Content:   "Halo! Terima kasih telah menghubungi kami. Ada yang bisa kami bantu hari ini?",
			Category:  "greeting",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "00000000-0000-4000-a000-000000000002",
			Shortcut:  "tunggu",
			Title:     "Mohon Menunggu",
			Content:   "Mohon tunggu sebentar ya, saya sedang mengecek informasi terkait hal ini.",
			Category:  "general",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "00000000-0000-4000-a000-000000000003",
			Shortcut:  "harga",
			Title:     "Informasi Paket & Harga",
			Content:   "Informasi paket langganan dan fitur lengkap dapat dilihat di website resmi kami atau kami bantu jelaskan di sini jika ada paket tertentu yang Anda minati.",
			Category:  "sales",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "00000000-0000-4000-a000-000000000004",
			Shortcut:  "terimakasih",
			Title:     "Penutup & Terima Kasih",
			Content:   "Sama-sama! Senang bisa membantu Anda. Jika ada pertanyaan lain, jangan ragu untuk menghubungi kami kembali. Semoga harimu menyenangkan!",
			Category:  "closing",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func newCannedStore() *cannedStore {
	cs := &cannedStore{
		items: make(map[string]CannedResponse),
	}
	for _, item := range defaultCannedResponses() {
		cs.items[item.ID] = item
	}
	return cs
}

func validateCannedResponse(shortcut, title, content string) error {
	shortcut = strings.TrimSpace(shortcut)
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	if shortcut == "" {
		return errors.New("Shortcut is required")
	}
	// normalize: strip leading slash if present
	shortcut = strings.TrimPrefix(shortcut, "/")
	if shortcut == "" {
		return errors.New("Shortcut cannot be empty or just a slash")
	}
	if utf8.RuneCountInString(shortcut) > 50 || strings.ContainsAny(shortcut, " \t\r\n\x00") {
		return errors.New("Shortcut must not contain spaces or control characters (up to 50 chars)")
	}
	if title == "" {
		return errors.New("Title is required")
	}
	if utf8.RuneCountInString(title) > 120 || strings.ContainsAny(title, "\r\n\x00") {
		return errors.New("Title must be 1–120 characters without newlines")
	}
	if content == "" {
		return errors.New("Content is required")
	}
	if utf8.RuneCountInString(content) > 6000 {
		return errors.New("Content cannot exceed 6000 characters")
	}
	return nil
}

func normalizeShortcut(shortcut string) string {
	s := strings.TrimSpace(shortcut)
	s = strings.TrimPrefix(s, "/")
	return strings.ToLower(s)
}

// Storage helpers
func (s *Server) cannedList(ctx context.Context, query string, category string) ([]CannedResponse, error) {
	query = strings.TrimSpace(query)
	category = strings.TrimSpace(category)

	if s.db != nil {
		sqlStr := `SELECT ` + cannedColumns + ` FROM canned_responses WHERE 1=1`
		args := []any{}
		argIdx := 1
		if query != "" {
			sqlStr += ` AND (strpos(lower(shortcut), lower($` + string(rune('0'+argIdx)) + `)) > 0 OR strpos(lower(title), lower($` + string(rune('0'+argIdx)) + `)) > 0 OR strpos(lower(content), lower($` + string(rune('0'+argIdx)) + `)) > 0)`
			args = append(args, query)
			argIdx++
		}
		if category != "" && category != "all" {
			sqlStr += ` AND lower(category) = lower($` + string(rune('0'+argIdx)) + `)`
			args = append(args, category)
			argIdx++
		}
		sqlStr += ` ORDER BY updated_at DESC`

		rows, err := s.db.QueryContext(ctx, sqlStr, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var list []CannedResponse
		for rows.Next() {
			c, err := scanCannedResponse(rows)
			if err != nil {
				return nil, err
			}
			list = append(list, c)
		}
		return list, rows.Err()
	}

	if s.canned == nil {
		s.canned = newCannedStore()
	}

	s.canned.RLock()
	defer s.canned.RUnlock()

	var list []CannedResponse
	qLower := strings.ToLower(query)
	catLower := strings.ToLower(category)

	for _, item := range s.canned.items {
		if catLower != "" && catLower != "all" && strings.ToLower(item.Category) != catLower {
			continue
		}
		if qLower != "" {
			sMatch := strings.Contains(strings.ToLower(item.Shortcut), qLower)
			tMatch := strings.Contains(strings.ToLower(item.Title), qLower)
			cMatch := strings.Contains(strings.ToLower(item.Content), qLower)
			if !sMatch && !tMatch && !cMatch {
				continue
			}
		}
		list = append(list, item)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list, nil
}

func (s *Server) cannedGet(ctx context.Context, id string) (CannedResponse, error) {
	if s.db != nil {
		row := s.db.QueryRowContext(ctx, `SELECT `+cannedColumns+` FROM canned_responses WHERE id::text=$1`, id)
		return scanCannedResponse(row)
	}

	if s.canned == nil {
		s.canned = newCannedStore()
	}

	s.canned.RLock()
	defer s.canned.RUnlock()

	item, ok := s.canned.items[id]
	if !ok {
		return CannedResponse{}, sql.ErrNoRows
	}
	return item, nil
}

func (s *Server) cannedCreate(ctx context.Context, item CannedResponse) (CannedResponse, error) {
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now
	if item.ID == "" {
		item.ID = db.GenerateUUID()
	}
	item.Shortcut = strings.TrimPrefix(strings.TrimSpace(item.Shortcut), "/")
	if item.Category == "" {
		item.Category = "general"
	}

	if s.db != nil {
		// Check for duplicate shortcut
		var existingID string
		err := s.db.QueryRowContext(ctx, `SELECT id::text FROM canned_responses WHERE lower(shortcut) = lower($1)`, item.Shortcut).Scan(&existingID)
		if err == nil {
			return CannedResponse{}, errors.New("Shortcut already exists")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return CannedResponse{}, err
		}

		row := s.db.QueryRowContext(ctx,
			`INSERT INTO canned_responses (id, shortcut, title, content, category, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+cannedColumns,
			item.ID, item.Shortcut, item.Title, item.Content, item.Category, item.CreatedAt, item.UpdatedAt,
		)
		return scanCannedResponse(row)
	}

	if s.canned == nil {
		s.canned = newCannedStore()
	}

	s.canned.Lock()
	defer s.canned.Unlock()

	normShortcut := normalizeShortcut(item.Shortcut)
	for _, existing := range s.canned.items {
		if normalizeShortcut(existing.Shortcut) == normShortcut {
			return CannedResponse{}, errors.New("Shortcut already exists")
		}
	}

	s.canned.items[item.ID] = item
	return item, nil
}

func (s *Server) cannedUpdate(ctx context.Context, id string, item CannedResponse) (CannedResponse, error) {
	now := time.Now().UTC()
	item.UpdatedAt = now
	item.Shortcut = strings.TrimPrefix(strings.TrimSpace(item.Shortcut), "/")
	if item.Category == "" {
		item.Category = "general"
	}

	if s.db != nil {
		// Check for duplicate shortcut under different ID
		var existingID string
		err := s.db.QueryRowContext(ctx, `SELECT id::text FROM canned_responses WHERE lower(shortcut) = lower($1) AND id::text != $2`, item.Shortcut, id).Scan(&existingID)
		if err == nil {
			return CannedResponse{}, errors.New("Shortcut already exists")
		} else if !errors.Is(err, sql.ErrNoRows) {
			return CannedResponse{}, err
		}

		row := s.db.QueryRowContext(ctx,
			`UPDATE canned_responses SET shortcut=$1, title=$2, content=$3, category=$4, updated_at=$5 WHERE id::text=$6 RETURNING `+cannedColumns,
			item.Shortcut, item.Title, item.Content, item.Category, item.UpdatedAt, id,
		)
		return scanCannedResponse(row)
	}

	if s.canned == nil {
		s.canned = newCannedStore()
	}

	s.canned.Lock()
	defer s.canned.Unlock()

	existing, ok := s.canned.items[id]
	if !ok {
		return CannedResponse{}, sql.ErrNoRows
	}

	normShortcut := normalizeShortcut(item.Shortcut)
	for exID, other := range s.canned.items {
		if exID != id && normalizeShortcut(other.Shortcut) == normShortcut {
			return CannedResponse{}, errors.New("Shortcut already exists")
		}
	}

	item.ID = id
	item.CreatedAt = existing.CreatedAt
	s.canned.items[id] = item
	return item, nil
}

func (s *Server) cannedDelete(ctx context.Context, id string) error {
	if s.db != nil {
		res, err := s.db.ExecContext(ctx, `DELETE FROM canned_responses WHERE id::text=$1`, id)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return sql.ErrNoRows
		}
		return nil
	}

	if s.canned == nil {
		s.canned = newCannedStore()
	}

	s.canned.Lock()
	defer s.canned.Unlock()

	if _, ok := s.canned.items[id]; !ok {
		return sql.ErrNoRows
	}
	delete(s.canned.items, id)
	return nil
}

// HTTP API Handlers (Authenticated)

// GET /api/v1/chat/canned-responses
func (s *Server) handleListCannedResponses(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	q := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")

	items, err := s.cannedList(r.Context(), q, category)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Could not list canned responses")
		return
	}
	if items == nil {
		items = []CannedResponse{}
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"canned_responses": items,
		"total":            len(items),
	})
}

// GET /api/v1/chat/canned-responses/{id}
func (s *Server) handleGetCannedResponse(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	id := chi.URLParam(r, "id")
	item, err := s.cannedGet(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respondJSONError(w, http.StatusNotFound, "Canned response not found")
		return
	}
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Could not fetch canned response")
		return
	}
	respondJSON(w, http.StatusOK, item)
}

// POST /api/v1/chat/canned-responses
func (s *Server) handleCreateCannedResponse(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}

	var payload struct {
		Shortcut string `json:"shortcut"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validateCannedResponse(payload.Shortcut, payload.Title, payload.Content); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	item := CannedResponse{
		Shortcut: strings.TrimSpace(payload.Shortcut),
		Title:    strings.TrimSpace(payload.Title),
		Content:  strings.TrimSpace(payload.Content),
		Category: strings.TrimSpace(payload.Category),
	}

	created, err := s.cannedCreate(r.Context(), item)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			respondJSONError(w, http.StatusConflict, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "Could not save canned response: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, created)
}

// PUT /api/v1/chat/canned-responses/{id}
func (s *Server) handleUpdateCannedResponse(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	id := chi.URLParam(r, "id")

	var payload struct {
		Shortcut string `json:"shortcut"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validateCannedResponse(payload.Shortcut, payload.Title, payload.Content); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	item := CannedResponse{
		Shortcut: strings.TrimSpace(payload.Shortcut),
		Title:    strings.TrimSpace(payload.Title),
		Content:  strings.TrimSpace(payload.Content),
		Category: strings.TrimSpace(payload.Category),
	}

	updated, err := s.cannedUpdate(r.Context(), id, item)
	if errors.Is(err, sql.ErrNoRows) {
		respondJSONError(w, http.StatusNotFound, "Canned response not found")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			respondJSONError(w, http.StatusConflict, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "Could not update canned response: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// DELETE /api/v1/chat/canned-responses/{id}
func (s *Server) handleDeleteCannedResponse(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	id := chi.URLParam(r, "id")

	err := s.cannedDelete(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		respondJSONError(w, http.StatusNotFound, "Canned response not found")
		return
	}
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Could not delete canned response")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

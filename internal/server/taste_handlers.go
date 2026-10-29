package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

// registerTasteRoutes mounts Taste & Agent Memory management endpoints.
func (s *Server) registerTasteRoutes(r chi.Router) {
	r.Get("/taste", s.handleGetTasteProfile)
	r.Get("/taste/", s.handleGetTasteProfile)
	r.Post("/taste/items", s.handleCreateTasteItem)
	r.Put("/taste/items/{id}", s.handleUpdateTasteItem)
	r.Delete("/taste/items/{id}", s.handleDeleteTasteItem)
	r.Post("/taste/reset", s.handleResetTasteProfile)
}

// GET /api/v1/taste
func (s *Server) handleGetTasteProfile(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	profile, err := s.getTasteProfile(owner)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memuat profil taste: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, profile)
}

// POST /api/v1/taste/items
func (s *Server) handleCreateTasteItem(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	var req struct {
		Category string `json:"category"`
		Topic    string `json:"topic"`
		Content  string `json:"content"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10240))
	if err := decoder.Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Payload JSON tidak valid")
		return
	}

	cat, err := normalizeCategory(req.Category)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondJSONError(w, http.StatusBadRequest, "Konten preferensi wajib diisi")
		return
	}
	if utf8.RuneCountInString(content) > 2000 {
		respondJSONError(w, http.StatusBadRequest, "Konten preferensi maksimal 2000 karakter")
		return
	}

	topic := strings.TrimSpace(req.Topic)
	if utf8.RuneCountInString(topic) > 100 {
		respondJSONError(w, http.StatusBadRequest, "Topik preferensi maksimal 100 karakter")
		return
	}

	created, err := s.createTasteItem(owner, cat, topic, content)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, created)
}

// PUT /api/v1/taste/items/{id}
func (s *Server) handleUpdateTasteItem(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "ID item preferensi wajib diisi")
		return
	}

	var req struct {
		Category *string `json:"category"`
		Topic    *string `json:"topic"`
		Content  *string `json:"content"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10240))
	if err := decoder.Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Payload JSON tidak valid")
		return
	}

	var newCat *string
	if req.Category != nil {
		cat, err := normalizeCategory(*req.Category)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		newCat = &cat
	}

	if req.Content != nil {
		c := strings.TrimSpace(*req.Content)
		if c == "" {
			respondJSONError(w, http.StatusBadRequest, "Konten preferensi tidak boleh kosong")
			return
		}
		if utf8.RuneCountInString(c) > 2000 {
			respondJSONError(w, http.StatusBadRequest, "Konten preferensi maksimal 2000 karakter")
			return
		}
		req.Content = &c
	}

	if req.Topic != nil {
		t := strings.TrimSpace(*req.Topic)
		if utf8.RuneCountInString(t) > 100 {
			respondJSONError(w, http.StatusBadRequest, "Topik preferensi maksimal 100 karakter")
			return
		}
		req.Topic = &t
	}

	updated, err := s.updateTasteItem(owner, id, newCat, req.Topic, req.Content)
	if err != nil {
		if errors.Is(err, ErrTasteItemNotFound) {
			respondJSONError(w, http.StatusNotFound, "Item preferensi tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// DELETE /api/v1/taste/items/{id}
func (s *Server) handleDeleteTasteItem(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "ID item preferensi wajib diisi")
		return
	}

	err := s.deleteTasteItem(owner, id)
	if err != nil {
		if errors.Is(err, ErrTasteItemNotFound) {
			respondJSONError(w, http.StatusNotFound, "Item preferensi tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "Gagal menghapus item preferensi: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message": "Item preferensi berhasil dihapus",
		"id":      id,
	})
}

// POST /api/v1/taste/reset
func (s *Server) handleResetTasteProfile(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	profile, err := s.resetTasteProfile(owner)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal mereset preferensi taste: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, profile)
}

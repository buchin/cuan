package server

import (
	"encoding/json"
	"errors"
	"fmt"
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

	// Quota Metrics & Configuration endpoints (Sprint 45 S45-T2)
	r.Get("/taste/quota", s.handleGetTasteQuota)
	r.Get("/taste/quota/", s.handleGetTasteQuota)
	r.Put("/taste/quota", s.handleUpdateTasteConfig)
	r.Get("/taste/config", s.handleGetTasteConfig)
	r.Get("/taste/config/", s.handleGetTasteConfig)
	r.Put("/taste/config", s.handleUpdateTasteConfig)
	r.Post("/taste/config", s.handleUpdateTasteConfig)

	// Candidate Taste & Reflection Loop endpoints (Sprint 44 S44-T2)
	r.Post("/taste/reflect", s.handleReflectTaste)
	r.Get("/taste/candidates", s.handleGetTasteCandidates)
	r.Get("/taste/candidates/", s.handleGetTasteCandidates)
	r.Post("/taste/candidates/{id}/accept", s.handleAcceptTasteCandidate)
	r.Post("/taste/candidates/{id}/dismiss", s.handleDismissTasteCandidate)
	r.Delete("/taste/candidates/{id}", s.handleDismissTasteCandidate)
	r.Post("/taste/candidates/{id}/undo", s.handleUndoTasteCandidate)
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

// GET /api/v1/taste/quota
func (s *Server) handleGetTasteQuota(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	quota, err := s.getTasteQuota(owner)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memuat metriks kuota taste: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, quota)
}

// GET /api/v1/taste/config
func (s *Server) handleGetTasteConfig(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	cfg := s.getTasteConfig(owner)
	respondJSON(w, http.StatusOK, cfg)
}

// PUT or POST /api/v1/taste/config and PUT /api/v1/taste/quota
func (s *Server) handleUpdateTasteConfig(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	var req struct {
		AutoApproveEnabled   *bool    `json:"auto_approve_enabled"`
		AutoApproveThreshold *float64 `json:"auto_approve_threshold"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10240))
	if err := decoder.Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Payload JSON tidak valid")
		return
	}

	cfg, err := s.updateTasteConfig(owner, req.AutoApproveEnabled, req.AutoApproveThreshold)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, cfg)
}

// POST /api/v1/taste/reflect
func (s *Server) handleReflectTaste(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	var req ReflectTasteRequest
	if r.Body != nil {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		_ = decoder.Decode(&req)
	}

	extracted, err := s.reflectTaste(r.Context(), owner, req)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memproses refleksi: "+err.Error())
		return
	}

	pending, _ := s.getTasteCandidates(owner, "pending")

	var autoApproved []TasteCandidate
	for _, c := range extracted {
		if c.Status == string(TasteCandidateStatusAccepted) {
			autoApproved = append(autoApproved, c)
		}
	}
	if autoApproved == nil {
		autoApproved = []TasteCandidate{}
	}

	msg := "Refleksi preferensi berhasil dieksekusi"
	if len(extracted) == 0 {
		msg = "Tidak ada preferensi atau koreksi baru yang terdeteksi"
	} else if len(autoApproved) > 0 {
		msg = fmt.Sprintf("Refleksi preferensi berhasil: %d preferensi otomatis disetujui", len(autoApproved))
	}

	respondJSON(w, http.StatusOK, ReflectTasteResponse{
		Message:      msg,
		Candidates:   extracted,
		Extracted:    extracted,
		AutoApproved: autoApproved,
		Count:        len(extracted),
		Total:        len(pending),
	})
}

// GET /api/v1/taste/candidates
func (s *Server) handleGetTasteCandidates(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	candidates, err := s.getTasteCandidates(owner, status)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memuat kandidat preferensi: "+err.Error())
		return
	}

	// Optional category filter
	if cat := strings.TrimSpace(r.URL.Query().Get("category")); cat != "" {
		var filtered []TasteCandidate
		for _, c := range candidates {
			if strings.EqualFold(c.Category, cat) {
				filtered = append(filtered, c)
			}
		}
		if filtered == nil {
			filtered = []TasteCandidate{}
		}
		candidates = filtered
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"candidates": candidates,
		"items":      candidates,
		"total":      len(candidates),
	})
}

// POST /api/v1/taste/candidates/{id}/accept
func (s *Server) handleAcceptTasteCandidate(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "ID kandidat preferensi wajib diisi")
		return
	}

	var req struct {
		Text     *string `json:"text"`
		Content  *string `json:"content"`
		Topic    *string `json:"topic"`
		Category *string `json:"category"`
	}
	if r.Body != nil {
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10240))
		_ = decoder.Decode(&req)
	}
	if req.Content != nil && req.Text == nil {
		req.Text = req.Content
	}

	candidate, item, err := s.acceptTasteCandidate(owner, id, req.Text, req.Topic, req.Category)
	if err != nil {
		if errors.Is(err, ErrTasteCandidateNotFound) {
			respondJSONError(w, http.StatusNotFound, "Kandidat preferensi tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":   "Kandidat preferensi berhasil disetujui",
		"candidate": candidate,
		"item":      item,
		"id":        item.ID,
	})
}

// POST /api/v1/taste/candidates/{id}/dismiss and DELETE /api/v1/taste/candidates/{id}
func (s *Server) handleDismissTasteCandidate(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "ID kandidat preferensi wajib diisi")
		return
	}

	candidate, err := s.dismissTasteCandidate(owner, id)
	if err != nil {
		if errors.Is(err, ErrTasteCandidateNotFound) {
			respondJSONError(w, http.StatusNotFound, "Kandidat preferensi tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "Gagal mengabaikan kandidat preferensi: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":   "Kandidat preferensi berhasil diabaikan",
		"candidate": candidate,
		"id":        id,
	})
}

// POST /api/v1/taste/candidates/{id}/undo
func (s *Server) handleUndoTasteCandidate(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "ID kandidat preferensi wajib diisi")
		return
	}

	cand, err := s.undoTasteCandidate(owner, id)
	if err != nil {
		if errors.Is(err, ErrTasteCandidateNotFound) {
			respondJSONError(w, http.StatusNotFound, "Kandidat preferensi tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":   "Promosi otomatis berhasil dibatalkan, preferensi dikembalikan ke daftar tinjauan",
		"candidate": cand,
		"id":        id,
	})
}

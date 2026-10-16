package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/sponsorship"
)

// handleDojoWebhook handles incoming webhook events from Dojo payment gateway.
// Verifies webhook secret, performs idempotency check, and transitions order and sponsor status.
func (s *Server) handleDojoWebhook(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	// 1. Extract secret from various supported headers and params
	secret := r.Header.Get("X-Dojo-Secret")
	if secret == "" {
		secret = r.Header.Get("X-Webhook-Secret")
	}
	if secret == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			secret = strings.TrimPrefix(authHeader, "Bearer ")
		} else if authHeader != "" {
			secret = authHeader
		}
	}
	if secret == "" {
		secret = r.URL.Query().Get("secret")
	}

	// 2. Decode payload
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB limit
	var payload sponsorship.DojoWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "format json webhook tidak valid: "+err.Error())
		return
	}

	// 3. Process via Service
	result, err := s.sponsorshipSvc.ProcessDojoWebhook(r.Context(), secret, &payload)
	if err != nil {
		if errors.Is(err, sponsorship.ErrInvalidWebhookSecret) {
			respondJSONError(w, http.StatusUnauthorized, "invalid webhook secret")
			return
		}
		if errors.Is(err, sponsorship.ErrMissingOrderID) {
			respondJSONError(w, http.StatusBadRequest, "missing order_id")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "gagal memproses webhook: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// handleUploadSponsorLogo handles isolated logo upload with MIME validation & SVG anti-XSS sanitization.
func (s *Server) handleUploadSponsorLogo(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	// Limit body to 2MB + multipart overhead
	r.Body = http.MaxBytesReader(w, r.Body, sponsorship.MaxLogoSizeBytes+1024*64)
	if err := r.ParseMultipartForm(sponsorship.MaxLogoSizeBytes); err != nil {
		respondJSONError(w, http.StatusBadRequest, "ukuran upload melebihi batas maksimal 2MB")
		return
	}

	file, header, err := r.FormFile("logo")
	if err != nil {
		file, header, err = r.FormFile("file")
	}
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "file logo wajib disertakan dalam form-data (field 'logo' atau 'file')")
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "gagal membaca file upload: "+err.Error())
		return
	}

	relPath, mimeType, err := s.sponsorshipSvc.SaveLogo(fileBytes, header.Filename)
	if err != nil {
		if errors.Is(err, sponsorship.ErrMaliciousSVG) {
			respondJSONError(w, http.StatusBadRequest, "upload file berbahaya ditolak: terdeteksi skrip berbahaya atau payload XSS")
			return
		}
		if errors.Is(err, sponsorship.ErrUnsupportedMimeType) {
			respondJSONError(w, http.StatusBadRequest, "tipe file tidak didukung: hanya SVG, PNG, dan WebP yang diizinkan")
			return
		}
		if errors.Is(err, sponsorship.ErrFileTooLarge) {
			respondJSONError(w, http.StatusBadRequest, "ukuran file logo melebihi batas maksimal 2MB")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "success",
		"logo_path": relPath,
		"mime_type": mimeType,
	})
}

// handleServeSponsorLogo safely serves uploaded sponsor logos with strict security headers.
func (s *Server) handleServeSponsorLogo(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil || s.sponsorshipSvc.LogoStorage() == nil {
		http.NotFound(w, r)
		return
	}

	path := r.URL.Path
	prefix := "/uploads/sponsors/logos/"
	if !strings.HasPrefix(path, prefix) {
		http.NotFound(w, r)
		return
	}

	filename := strings.TrimPrefix(path, prefix)
	data, mimeType, err := s.sponsorshipSvc.LogoStorage().ReadLogo(filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Security headers to prevent SVG XSS execution in browser context
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	w.Header().Set("Content-Type", mimeType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleSponsorIntake handles prospective founding sponsor submission.
func (s *Server) handleSponsorIntake(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	var req sponsorship.CreateSponsorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "format json tidak valid: "+err.Error())
		return
	}

	sp, err := s.sponsorshipSvc.CreateSponsor(r.Context(), req)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, sp)
}

// handleSponsorCheckout handles PWYW sponsorship intake and creates checkout session.
func (s *Server) handleSponsorCheckout(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	var req sponsorship.CreateCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "format json tidak valid: "+err.Error())
		return
	}

	result, err := s.sponsorshipSvc.CreateCheckoutSession(r.Context(), req)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, result)
}

// handleGetSponsorshipStatus returns real-time order and sponsor curation status.
func (s *Server) handleGetSponsorshipStatus(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	orderID := r.URL.Query().Get("order_id")
	sponsorID := r.URL.Query().Get("sponsor_id")

	status, err := s.sponsorshipSvc.GetSponsorshipStatus(r.Context(), orderID, sponsorID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "gagal memeriksa status sponsorship: "+err.Error())
		return
	}

	if !status.Found {
		respondJSON(w, http.StatusNotFound, status)
		return
	}

	respondJSON(w, http.StatusOK, status)
}

// handleListBatches returns all sponsorship batches for directory archive filtering.
func (s *Server) handleListBatches(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	batches, err := s.sponsorshipSvc.ListBatches(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "gagal memuat daftar batch: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"batches": batches,
		"count":   len(batches),
	})
}

// handleListCurrentSponsors lists approved sponsors for public leaderboard (ORDER BY amount DESC, paid_at ASC).
func (s *Server) handleListCurrentSponsors(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	batchSlug := r.URL.Query().Get("batch")
	if batchSlug == "" {
		batchSlug = "v1.0"
	}

	sponsors, err := s.sponsorshipSvc.ListApprovedSponsors(r.Context(), batchSlug)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "gagal mengambil daftar sponsor: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"batch":    batchSlug,
		"total":    len(sponsors),
		"sponsors": sponsors,
	})
}

// handleCreateLead handles waiting list signups.
func (s *Server) handleCreateLead(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	var req struct {
		Email  string `json:"email"`
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "format json tidak valid: "+err.Error())
		return
	}

	lead, err := s.sponsorshipSvc.SubmitWaitingListLead(r.Context(), req.Email, req.Source)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status":         "success",
		"message":        "Terima kasih telah bergabung di Waiting List Cuan v1.0",
		"lead":           lead,
		"dojo_forwarded": s.sponsorshipSvc.DojoDispatcher() != nil,
	})
}

// handleAdminListSponsors lists all sponsors with status filter for owner curation.
func (s *Server) handleAdminListSponsors(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	batchID := r.URL.Query().Get("batch_id")
	status := r.URL.Query().Get("status")

	sponsors, err := s.sponsorshipSvc.ListSponsors(r.Context(), batchID, status)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sponsors": sponsors,
		"count":    len(sponsors),
	})
}

// handleAdminApproveSponsor approves sponsor and publishes to leaderboard.
func (s *Server) handleAdminApproveSponsor(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	id := chi.URLParam(r, "id")
	if err := s.sponsorshipSvc.ApproveSponsor(r.Context(), id, "owner"); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "approved",
		"id":      id,
		"message": "Sponsor berhasil disetujui dan live di leaderboard",
	})
}

// handleAdminRejectSponsor rejects sponsor with anti-judol violation reason.
func (s *Server) handleAdminRejectSponsor(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	id := chi.URLParam(r, "id")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := s.sponsorshipSvc.RejectSponsor(r.Context(), id, req.Reason, "owner"); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "rejected",
		"id":      id,
		"reason":  req.Reason,
		"message": "Sponsor berhasil ditolak",
	})
}

// handleAdminListLeads lists all registered leads for owner inspection.
func (s *Server) handleAdminListLeads(w http.ResponseWriter, r *http.Request) {
	if s.sponsorshipSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "layanan sponsorship belum diinisialisasi")
		return
	}

	leads, err := s.sponsorshipSvc.ListWaitingListLeads(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"leads": leads,
		"count": len(leads),
	})
}

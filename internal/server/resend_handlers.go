package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/crypto"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/resend"
)

var resendSettingsMu sync.RWMutex

// ResendSettings represents the internal configuration for Resend email provider.
type ResendSettings struct {
	WebhookSecret        string     `json:"webhook_secret,omitempty"`
	APIKey               string     `json:"api_key"` // Plain text in memory when decrypted, never exposed in API DTO
	FromEmail            string     `json:"from_email"`
	FromName             string     `json:"from_name"`
	NotificationEmail    string     `json:"notification_email"`
	IdleThresholdMinutes int        `json:"idle_threshold_minutes"`
	Enabled              bool       `json:"enabled"`
	LastValidatedAt      *time.Time `json:"last_validated_at,omitempty"`
	ValidationStatus     string     `json:"validation_status,omitempty"` // "untested", "valid", "invalid"
}

// ResendSettingsDTO is the public transfer object for Resend settings with zero secret leakage.
type ResendSettingsDTO struct {
	HasWebhookSecret     bool       `json:"has_webhook_secret"`
	HasAPIKey            bool       `json:"has_api_key"`
	MaskedAPIKey         string     `json:"masked_api_key"`
	FromEmail            string     `json:"from_email"`
	FromName             string     `json:"from_name"`
	NotificationEmail    string     `json:"notification_email"`
	IdleThresholdMinutes int        `json:"idle_threshold_minutes"`
	Enabled              bool       `json:"enabled"`
	LastValidatedAt      *time.Time `json:"last_validated_at,omitempty"`
	ValidationStatus     string     `json:"validation_status"`
}

// ToDTO transforms internal settings to safe DTO.
func (s *ResendSettings) ToDTO() ResendSettingsDTO {
	hasKey := strings.TrimSpace(s.APIKey) != ""
	masked := ""
	if hasKey {
		masked = agent.MaskAPIKey(s.APIKey)
	}
	fromEmail := s.FromEmail
	if fromEmail == "" {
		fromEmail = "onboarding@resend.dev"
	}
	fromName := s.FromName
	if fromName == "" {
		fromName = "Cuan Support"
	}
	threshold := s.IdleThresholdMinutes
	if threshold <= 0 {
		threshold = 5
	}
	valStatus := s.ValidationStatus
	if valStatus == "" {
		valStatus = "untested"
	}

	return ResendSettingsDTO{
		HasWebhookSecret:     s.WebhookSecret != "",
		HasAPIKey:            hasKey,
		MaskedAPIKey:         masked,
		FromEmail:            fromEmail,
		FromName:             fromName,
		NotificationEmail:    s.NotificationEmail,
		IdleThresholdMinutes: threshold,
		Enabled:              s.Enabled,
		LastValidatedAt:      s.LastValidatedAt,
		ValidationStatus:     valStatus,
	}
}

// resendSettingsFilePath returns the local fallback file path for resend configuration.
func (s *Server) resendSettingsFilePath() string {
	dir := "data"
	if s.cfg != nil && s.cfg.WorkspacesDir != "" {
		dir = filepath.Dir(s.cfg.WorkspacesDir)
	}
	return filepath.Join(dir, "resend_settings.json")
}

// GetResendSettings loads the Resend configuration from project_integrations repo or local disk cache.
func (s *Server) GetResendSettings(ctx context.Context) (*ResendSettings, error) {
	resendSettingsMu.RLock()
	defer resendSettingsMu.RUnlock()

	secret := s.cfg.AuthSecret
	if secret == "" {
		secret = "cuan-default-resend-vault-secret-key-32b"
	}

	// 1. Try to load from project_integrations repo
	if s.projectIntegrationRepo != nil {
		in, err := s.projectIntegrationRepo.GetIntegrationByProjectAndType(ctx, "default", "resend")
		if err == nil && in != nil && in.ConfigEncrypted != "" {
			decrypted, err := crypto.DecryptStringAES256GCM(in.ConfigEncrypted, secret)
			if err == nil && decrypted != "" {
				var settings ResendSettings
				if err := json.Unmarshal([]byte(decrypted), &settings); err == nil {
					return &settings, nil
				}
			}
		}
	}

	// 2. Fallback to local encrypted disk file cache
	filePath := s.resendSettingsFilePath()
	if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
		decrypted, err := crypto.DecryptStringAES256GCM(string(data), secret)
		if err == nil && decrypted != "" {
			var settings ResendSettings
			if err := json.Unmarshal([]byte(decrypted), &settings); err == nil {
				return &settings, nil
			}
		}
	}

	// 3. Return default unconfigured settings
	return &ResendSettings{
		FromEmail:            "onboarding@resend.dev",
		FromName:             "Cuan Support",
		IdleThresholdMinutes: 5,
		Enabled:              false,
		ValidationStatus:     "untested",
	}, nil
}

// SaveResendSettings encrypts and saves Resend settings to project_integrations and local disk cache.
func (s *Server) SaveResendSettings(ctx context.Context, settings *ResendSettings) error {
	resendSettingsMu.Lock()
	defer resendSettingsMu.Unlock()

	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	secret := s.cfg.AuthSecret
	if secret == "" {
		secret = "cuan-default-resend-vault-secret-key-32b"
	}

	rawJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal resend settings: %w", err)
	}

	encrypted, err := crypto.EncryptStringAES256GCM(string(rawJSON), secret)
	if err != nil {
		return fmt.Errorf("encrypt resend settings: %w", err)
	}

	// 1. Save to project_integrations repo
	if s.projectIntegrationRepo != nil {
		in, err := s.projectIntegrationRepo.GetIntegrationByProjectAndType(ctx, "default", "resend")
		if err == nil && in != nil {
			in.ConfigEncrypted = encrypted
			status := "active"
			if !settings.Enabled {
				status = "inactive"
			}
			in.Status = status
			_ = s.projectIntegrationRepo.UpdateIntegration(ctx, in)
		} else {
			status := "active"
			if !settings.Enabled {
				status = "inactive"
			}
			newIn := &cuanDB.ProjectIntegration{
				ID:              cuanDB.GenerateUUID(),
				ProjectID:       "default",
				IntegrationType: "resend",
				ConfigEncrypted: encrypted,
				Status:          status,
			}
			_ = s.projectIntegrationRepo.CreateIntegration(ctx, newIn)
		}
	}

	// 2. Save encrypted copy to local disk cache
	filePath := s.resendSettingsFilePath()
	_ = os.MkdirAll(filepath.Dir(filePath), 0755)
	_ = os.WriteFile(filePath, []byte(encrypted), 0600)

	// Invalidate any cached client instance so next call uses updated key
	s.resendClient = nil

	return nil
}

// ResendClient returns a ready-to-use Resend client with current settings.
func (s *Server) ResendClient(ctx context.Context) (resend.Client, error) {
	if s.resendClient != nil {
		return s.resendClient, nil
	}

	settings, err := s.GetResendSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve resend settings: %w", err)
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		return nil, resend.ErrEmptyAPIKey
	}

	return resend.NewClient(settings.APIKey), nil
}

// SetResendClient injects a mock or custom resend client for testing.
func (s *Server) SetResendClient(c resend.Client) {
	s.resendClient = c
}

// ============================================================================
// HTTP Handlers
// ============================================================================

// handleGetResendSettings returns the current Resend settings as a safe DTO.
// Route: GET /api/v1/settings/resend
func (s *Server) handleGetResendSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.GetResendSettings(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memuat pengaturan Resend: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"settings": settings.ToDTO(),
	})
}

// handleSaveResendSettings updates and securely encrypts Resend configuration.
// Route: POST /api/v1/settings/resend, PATCH /api/v1/settings/resend
func (s *Server) handleSaveResendSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WebhookSecret        *string `json:"webhook_secret"`
		APIKey               string  `json:"api_key"`
		FromEmail            string  `json:"from_email"`
		FromName             string  `json:"from_name"`
		NotificationEmail    string  `json:"notification_email"`
		IdleThresholdMinutes int     `json:"idle_threshold_minutes"`
		Enabled              *bool   `json:"enabled"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	current, err := s.GetResendSettings(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal membaca konfigurasi yang ada: "+err.Error())
		return
	}

	// Update API key if non-empty string provided
	if input.WebhookSecret != nil {
		secret := strings.TrimSpace(*input.WebhookSecret)
		if secret != "" {
			if _, err := newResendWebhook(secret); err != nil {
				respondJSONError(w, 400, "Invalid webhook signing secret")
				return
			}
		}
		current.WebhookSecret = secret
	}
	rawKey := strings.TrimSpace(input.APIKey)
	if rawKey != "" {
		// Basic format sanity check for Resend API keys
		if !strings.HasPrefix(rawKey, "re_") && len(rawKey) < 10 {
			respondJSONError(w, http.StatusBadRequest, "Format API Key Resend tidak valid. Kunci biasanya diawali dengan 're_'.")
			return
		}
		current.APIKey = rawKey
		current.ValidationStatus = "untested"
	}

	if input.FromEmail != "" {
		current.FromEmail = strings.TrimSpace(input.FromEmail)
	}
	if input.FromName != "" {
		current.FromName = strings.TrimSpace(input.FromName)
	}
	if input.NotificationEmail != "" {
		current.NotificationEmail = strings.TrimSpace(input.NotificationEmail)
	}
	if input.IdleThresholdMinutes > 0 {
		if input.IdleThresholdMinutes < 1 {
			current.IdleThresholdMinutes = 1
		} else if input.IdleThresholdMinutes > 1440 {
			current.IdleThresholdMinutes = 1440
		} else {
			current.IdleThresholdMinutes = input.IdleThresholdMinutes
		}
	}
	if input.Enabled != nil {
		if *input.Enabled && current.APIKey == "" {
			respondJSONError(w, http.StatusBadRequest, "Tidak dapat mengaktifkan integrasi Resend tanpa API Key.")
			return
		}
		current.Enabled = *input.Enabled
	}

	if err := s.SaveResendSettings(r.Context(), current); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal menyimpan pengaturan: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"message":  "Pengaturan Resend berhasil disimpan dengan aman.",
		"settings": current.ToDTO(),
	})
}

// handleValidateResendKey checks if the Resend API key is valid against Resend API.
// Route: POST /api/v1/settings/resend/validate
func (s *Server) handleValidateResendKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		APIKey string `json:"api_key"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input)

	keyToValidate := strings.TrimSpace(input.APIKey)
	current, err := s.GetResendSettings(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if keyToValidate == "" {
		keyToValidate = current.APIKey
	}
	if keyToValidate == "" {
		respondJSONError(w, http.StatusBadRequest, "Masukkan API Key Resend untuk divalidasi.")
		return
	}

	var client resend.Client
	if s.resendClient != nil {
		client = s.resendClient
	} else {
		client = resend.NewClient(keyToValidate)
	}

	valErr := client.ValidateAPIKey(r.Context())
	now := time.Now().UTC()
	current.LastValidatedAt = &now

	if valErr != nil {
		current.ValidationStatus = "invalid"
		_ = s.SaveResendSettings(r.Context(), current)
		respondJSON(w, http.StatusOK, map[string]any{
			"status":            "invalid",
			"valid":             false,
			"error":             valErr.Error(),
			"validation_status": "invalid",
			"last_validated_at": now,
		})
		return
	}

	current.ValidationStatus = "valid"
	_ = s.SaveResendSettings(r.Context(), current)

	respondJSON(w, http.StatusOK, map[string]any{
		"status":            "valid",
		"valid":             true,
		"message":           "API Key Resend valid dan berhasil terhubung!",
		"validation_status": "valid",
		"last_validated_at": now,
	})
}

// handleTestResendEmail sends a verification test email using configured or provided details.
// Route: POST /api/v1/settings/resend/test
func (s *Server) handleTestResendEmail(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RecipientEmail string `json:"recipient_email"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input)

	settings, err := s.GetResendSettings(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		respondJSONError(w, http.StatusBadRequest, "API Key Resend belum dikonfigurasi. Harap simpan API Key terlebih dahulu.")
		return
	}

	recipient := strings.TrimSpace(input.RecipientEmail)
	if recipient == "" {
		recipient = strings.TrimSpace(settings.NotificationEmail)
	}
	if recipient == "" && s.authSvc != nil {
		if info, err := s.authSvc.OwnerIdentity(r.Context()); err == nil && info.Email != "" {
			recipient = info.Email
		}
	}
	if recipient == "" {
		respondJSONError(w, http.StatusBadRequest, "Masukkan alamat email penerima untuk pengujian.")
		return
	}

	fromName := settings.FromName
	if fromName == "" {
		fromName = "Cuan Support"
	}
	fromEmail := settings.FromEmail
	if fromEmail == "" {
		fromEmail = "onboarding@resend.dev"
	}
	fromHeader := fmt.Sprintf("%s <%s>", fromName, fromEmail)

	client, err := s.ResendClient(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal menginisialisasi client Resend: "+err.Error())
		return
	}

	testHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;padding:24px;background:#f8fafc;color:#1e293b;">
  <div style="max-width:540px;margin:0 auto;background:#fff;border-radius:10px;padding:24px;border:1px solid #e2e8f0;">
    <h2 style="color:#0f172a;margin-top:0;">✅ Pengujian Integrasi Resend Berhasil!</h2>
    <p>Halo,</p>
    <p>Ini adalah email konfirmasi pengujian dari platform <strong>Cuan</strong>. Integrasi Resend Anda telah aktif dan siap digunakan untuk mengirimkan transkrip notifikasi obrolan saat Live Chat idle.</p>
    <div style="background:#f1f5f9;border-left:4px solid #0284c7;padding:12px;margin:16px 0;font-size:14px;">
      <strong>Waktu Uji Coba:</strong> %s WIB<br>
      <strong>Pengirim:</strong> %s
    </div>
    <p style="font-size:12px;color:#94a3b8;margin-bottom:0;">Cuan · Otomasi Toko Online</p>
  </div>
</body>
</html>`, time.Now().UTC().Format("02 Jan 2006 15:04"), fromHeader)

	testText := fmt.Sprintf("Pengujian Integrasi Resend Berhasil!\n\nEmail ini membuktikan bahwa konfigurasi Resend di Cuan telah aktif dan siap mengirimkan notifikasi chat idle.\nWaktu: %s", time.Now().UTC().Format("15:04 WIB"))

	resp, err := client.SendEmail(r.Context(), resend.EmailRequest{
		From:    fromHeader,
		To:      []string{recipient},
		Subject: "✅ [Cuan] Uji Coba Integrasi Email Resend Berhasil",
		HTML:    testHTML,
		Text:    testText,
	})
	if err != nil {
		respondJSONError(w, http.StatusBadGateway, "Gagal mengirim email pengujian via Resend: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":          "success",
		"message":         "Email pengujian berhasil dikirim ke " + recipient,
		"resend_email_id": resp.ID,
		"recipient":       recipient,
	})
}

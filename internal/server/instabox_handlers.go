package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/crypto"
	cuanDB "github.com/buchin/cuan/internal/db"
)

var instaboxSettingsMu sync.RWMutex

// InstaboxSettings represents the configuration for the remote Instabox browser service.
type InstaboxSettings struct {
	EndpointURL      string     `json:"endpoint_url"`
	APIKey           string     `json:"api_key,omitempty"` // Plain text in memory when decrypted, never exposed in API DTO
	Enabled          bool       `json:"enabled"`
	LastValidatedAt  *time.Time `json:"last_validated_at,omitempty"`
	ValidationStatus string     `json:"validation_status,omitempty"` // "untested", "valid", "invalid"
}

// InstaboxSettingsDTO is the public safe transfer object with zero secret leakage.
type InstaboxSettingsDTO struct {
	HasAPIKey        bool       `json:"has_api_key"`
	MaskedAPIKey     string     `json:"masked_api_key"`
	EndpointURL      string     `json:"endpoint_url"`
	Enabled          bool       `json:"enabled"`
	LastValidatedAt  *time.Time `json:"last_validated_at,omitempty"`
	ValidationStatus string     `json:"validation_status"`
}

// ToDTO transforms internal settings to safe DTO.
func (s *InstaboxSettings) ToDTO() InstaboxSettingsDTO {
	hasKey := strings.TrimSpace(s.APIKey) != ""
	masked := ""
	if hasKey {
		masked = agent.MaskAPIKey(s.APIKey)
	}
	valStatus := s.ValidationStatus
	if valStatus == "" {
		valStatus = "untested"
	}
	return InstaboxSettingsDTO{
		HasAPIKey:        hasKey,
		MaskedAPIKey:     masked,
		EndpointURL:      s.EndpointURL,
		Enabled:          s.Enabled,
		LastValidatedAt:  s.LastValidatedAt,
		ValidationStatus: valStatus,
	}
}

func (s *Server) instaboxSettingsFilePath() string {
	dir := "data"
	if s.cfg != nil && s.cfg.WorkspacesDir != "" {
		dir = filepath.Dir(s.cfg.WorkspacesDir)
	}
	return filepath.Join(dir, "instabox_settings.json")
}

// GetInstaboxSettings reads encrypted settings from project_integrations repo or local cache.
func (s *Server) GetInstaboxSettings(ctx context.Context) (*InstaboxSettings, error) {
	instaboxSettingsMu.RLock()
	defer instaboxSettingsMu.RUnlock()

	secret := s.cfg.AuthSecret
	if secret == "" {
		secret = "cuan-default-instabox-vault-secret-32b"
	}

	// 1. Try to load from project_integrations repo
	if s.projectIntegrationRepo != nil {
		in, err := s.projectIntegrationRepo.GetIntegrationByProjectAndType(ctx, "default", "instabox")
		if err == nil && in != nil && in.ConfigEncrypted != "" {
			decrypted, err := crypto.DecryptStringAES256GCM(in.ConfigEncrypted, secret)
			if err == nil && decrypted != "" {
				var settings InstaboxSettings
				if err := json.Unmarshal([]byte(decrypted), &settings); err == nil {
					return &settings, nil
				}
			}
		}
	}

	// 2. Fallback to local encrypted disk file cache
	filePath := s.instaboxSettingsFilePath()
	if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
		decrypted, err := crypto.DecryptStringAES256GCM(string(data), secret)
		if err == nil && decrypted != "" {
			var settings InstaboxSettings
			if err := json.Unmarshal([]byte(decrypted), &settings); err == nil {
				return &settings, nil
			}
		}
	}

	// 3. Fallback to ambient env var CUAN_REMOTE_BROWSER_URL if present
	if envURL := os.Getenv("CUAN_REMOTE_BROWSER_URL"); envURL != "" {
		return &InstaboxSettings{
			EndpointURL:      envURL,
			Enabled:          true,
			ValidationStatus: "untested",
		}, nil
	}

	// Return empty default
	return &InstaboxSettings{
		ValidationStatus: "untested",
	}, nil
}

// SaveInstaboxSettings encrypts and saves Instabox settings to both DB and local fallback cache.
func (s *Server) SaveInstaboxSettings(ctx context.Context, settings *InstaboxSettings) error {
	instaboxSettingsMu.Lock()
	defer instaboxSettingsMu.Unlock()

	secret := s.cfg.AuthSecret
	if secret == "" {
		secret = "cuan-default-instabox-vault-secret-32b"
	}

	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	encrypted, err := crypto.EncryptStringAES256GCM(string(raw), secret)
	if err != nil {
		return fmt.Errorf("encrypt settings: %w", err)
	}

	// 1. Save to DB if repo available
	if s.projectIntegrationRepo != nil {
		in, err := s.projectIntegrationRepo.GetIntegrationByProjectAndType(ctx, "default", "instabox")
		status := "active"
		if !settings.Enabled {
			status = "inactive"
		}
		if err == nil && in != nil {
			in.ConfigEncrypted = encrypted
			in.Status = status
			in.UpdatedAt = time.Now()
			_ = s.projectIntegrationRepo.UpdateIntegration(ctx, in)
		} else {
			newIn := &cuanDB.ProjectIntegration{
				ID:              cuanDB.GenerateUUID(),
				ProjectID:       "default",
				IntegrationType: "instabox",
				ConfigEncrypted: encrypted,
				Status:          status,
			}
			_ = s.projectIntegrationRepo.CreateIntegration(ctx, newIn)
		}
	}

	// 2. Save to local encrypted disk file
	filePath := s.instaboxSettingsFilePath()
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err == nil {
		_ = os.WriteFile(filePath, []byte(encrypted), 0600)
	}

	// Update host env if enabled
	if settings.Enabled && settings.EndpointURL != "" {
		_ = os.Setenv("CUAN_REMOTE_BROWSER_URL", settings.EndpointURL)
	} else if !settings.Enabled {
		_ = os.Unsetenv("CUAN_REMOTE_BROWSER_URL")
	}

	return nil
}

// handleGetInstaboxSettings returns current Instabox settings.
// Route: GET /api/v1/settings/instabox
func (s *Server) handleGetInstaboxSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.GetInstaboxSettings(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memuat pengaturan Instabox: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"settings": settings.ToDTO(),
	})
}

// handleSaveInstaboxSettings updates and securely encrypts Instabox configuration.
// Route: POST /api/v1/settings/instabox, PATCH /api/v1/settings/instabox
func (s *Server) handleSaveInstaboxSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		EndpointURL string `json:"endpoint_url"`
		APIKey      string `json:"api_key"`
		Enabled     *bool  `json:"enabled"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	current, err := s.GetInstaboxSettings(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal membaca konfigurasi yang ada: "+err.Error())
		return
	}

	if strings.TrimSpace(input.EndpointURL) != "" {
		current.EndpointURL = strings.TrimSpace(input.EndpointURL)
	}
	if strings.TrimSpace(input.APIKey) != "" {
		current.APIKey = strings.TrimSpace(input.APIKey)
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}

	if err := s.SaveInstaboxSettings(r.Context(), current); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal menyimpan konfigurasi: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"message":  "Pengaturan Instabox berhasil disimpan",
		"settings": current.ToDTO(),
	})
}

// handleValidateInstaboxEndpoint validates reachability of the Instabox browser service endpoint.
// Route: POST /api/v1/settings/instabox/validate
func (s *Server) handleValidateInstaboxEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EndpointURL string `json:"endpoint_url"`
		APIKey      string `json:"api_key"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req)

	endpoint := strings.TrimSpace(req.EndpointURL)
	if endpoint == "" {
		current, _ := s.GetInstaboxSettings(r.Context())
		if current != nil {
			endpoint = current.EndpointURL
			if req.APIKey == "" {
				req.APIKey = current.APIKey
			}
		}
	}

	if endpoint == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"valid":   false,
			"message": "Endpoint URL wajib diisi untuk pengujian koneksi",
		})
		return
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "ws" && parsed.Scheme != "wss") {
		respondJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"valid":   false,
			"message": "Skema endpoint harus http://, https://, ws://, atau wss://",
		})
		return
	}

	// For WebSocket endpoints, convert to HTTP/HTTPS for health probe
	probeURL := endpoint
	if parsed.Scheme == "ws" {
		probeURL = "http://" + parsed.Host + parsed.Path
	} else if parsed.Scheme == "wss" {
		probeURL = "https://" + parsed.Host + parsed.Path
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, probeURL, nil)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"valid":   false,
			"message": "Gagal menyiapkan probe HTTP: " + err.Error(),
		})
		return
	}

	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}

	resp, err := client.Do(httpReq)
	now := time.Now()
	current, _ := s.GetInstaboxSettings(r.Context())
	if err != nil {
		if current != nil {
			current.ValidationStatus = "invalid"
			current.LastValidatedAt = &now
			_ = s.SaveInstaboxSettings(r.Context(), current)
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"valid":   false,
			"message": fmt.Sprintf("Koneksi gagal: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	// Any response code below 500 confirms endpoint reachability
	isValid := resp.StatusCode < 500
	statusText := "valid"
	if !isValid {
		statusText = "invalid"
	}
	if current != nil {
		current.ValidationStatus = statusText
		current.LastValidatedAt = &now
		_ = s.SaveInstaboxSettings(r.Context(), current)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":      "success",
		"valid":       isValid,
		"http_status": resp.StatusCode,
		"message":     fmt.Sprintf("Endpoint terhubung dengan respons HTTP %d", resp.StatusCode),
	})
}

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

type inboxBrainAction struct {
	Action   string `json:"action"` // "list", "set_model", "configure", "test"
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
	Mode     string `json:"mode,omitempty"` // "byok", "subscription"
	Enabled  *bool  `json:"enabled,omitempty"`
}

func parseInboxBrain(content string) (string, *inboxBrainAction, error) {
	const marker = "```cuan-brain\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid brain action.")
	if strings.Count(normalized, marker) != 1 || !strings.HasSuffix(normalized, "```") {
		return "", nil, bad
	}
	raw := normalized[start+len(marker) : len(normalized)-3]
	var a inboxBrainAction
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32000 || decoder.Decode(&a) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	if a.Action == "" {
		a.Action = "list"
	}
	switch a.Action {
	case "set_model":
		if strings.TrimSpace(a.Provider) == "" {
			return "", nil, errors.New("Provider is required to set default model.")
		}
		if strings.TrimSpace(a.Model) == "" {
			return "", nil, errors.New("Model is required to set default model.")
		}
	case "configure":
		if strings.TrimSpace(a.Provider) == "" {
			return "", nil, errors.New("Provider is required for configuration.")
		}
	case "test":
		if strings.TrimSpace(a.Provider) == "" {
			return "", nil, errors.New("Provider is required for connection test.")
		}
	case "list":
		// No specific field required
	default:
		return "", nil, errors.New("Unsupported brain action.")
	}
	return strings.TrimSpace(normalized[:start]), &a, nil
}

func (s *Server) applyInboxBrain(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, action, err := parseInboxBrain(content)
	if err != nil {
		return err.Error()
	}
	if action == nil {
		return content
	}
	store := s.AIProviderStore()
	if store == nil {
		return cleanText + "\n\n⚠️ AI Provider Store tidak tersedia."
	}

	switch action.Action {
	case "list":
		bm := s.BinaryManager()
		reg := s.ProviderRegistry()
		catalog := store.ExportCatalogDTO(bm, reg)

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString("### AI Brains & Model Providers\n")
		if len(catalog) == 0 {
			sb.WriteString("_Belum ada penyedia AI yang terkonfigurasi._\n")
			return strings.TrimSpace(sb.String())
		}

		for _, p := range catalog {
			statusSymbol := "⚪"
			statusText := "Belum Diatur"
			if p.Status == "ready" {
				statusSymbol = "🟢"
				statusText = "Siap Digunakan"
			} else if p.Status == "configured" {
				statusSymbol = "🟡"
				statusText = "Terkonfigurasi"
			}

			modelText := p.DefaultModel
			if modelText == "" {
				modelText = "default otomatis"
			}

			modeText := strings.ToUpper(p.Mode)
			if modeText == "" {
				modeText = "BYOK"
			}

			sb.WriteString(fmt.Sprintf("%s **%s** (`%s`) — %s\n", statusSymbol, p.Name, p.ID, statusText))
			sb.WriteString(fmt.Sprintf("  • Mode: %s | Model Default: `%s`\n", modeText, modelText))
			if len(p.Models) > 0 {
				var modelNames []string
				for _, m := range p.Models {
					if len(modelNames) < 4 {
						modelNames = append(modelNames, m.ID)
					}
				}
				if len(p.Models) > 4 {
					modelNames = append(modelNames, fmt.Sprintf("+%d model lainnya", len(p.Models)-4))
				}
				sb.WriteString(fmt.Sprintf("  • Model Tersedia: %s\n", strings.Join(modelNames, ", ")))
			}
		}
		return strings.TrimSpace(sb.String())

	case "set_model":
		providerID := strings.ToLower(strings.TrimSpace(action.Provider))
		modelID := strings.TrimSpace(action.Model)

		cfg := store.GetConfig(providerID)
		cfg.DefaultModel = modelID
		if err := store.SaveConfig(cfg); err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal memperbarui model default: %v", err)
		}

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("✓ Model default untuk **%s** berhasil diatur ke `%s`.", providerID, modelID))
		return strings.TrimSpace(sb.String())

	case "configure":
		providerID := strings.ToLower(strings.TrimSpace(action.Provider))
		cfg := store.GetConfig(providerID)

		if action.Mode != "" {
			cfg.Mode = strings.ToLower(strings.TrimSpace(action.Mode))
		}
		if action.BaseURL != "" {
			cfg.BaseURL = strings.TrimSpace(action.BaseURL)
		}
		if action.Enabled != nil {
			cfg.Enabled = *action.Enabled
		}
		if action.APIKey != "" {
			encKey, err := agent.EncryptCredential(strings.TrimSpace(action.APIKey))
			if err != nil {
				return cleanText + fmt.Sprintf("\n\n⚠️ Gagal mengenkripsi kunci API: %v", err)
			}
			cfg.EncryptedAPIKey = encKey
		}

		if err := store.SaveConfig(cfg); err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal menyimpan konfigurasi provider: %v", err)
		}

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("✓ Konfigurasi provider AI **%s** berhasil diperbarui.", providerID))
		return strings.TrimSpace(sb.String())

	case "test":
		providerID := strings.ToLower(strings.TrimSpace(action.Provider))
		apiKey := strings.TrimSpace(action.APIKey)
		if apiKey == "" {
			storedKey, err := store.GetDecryptedAPIKey(providerID)
			if err == nil && storedKey != "" {
				apiKey = storedKey
			}
		}

		reg := s.ProviderRegistry()
		var provider agent.Provider
		if reg != nil {
			provider, _ = reg.Get(providerID)
		}
		if provider == nil {
			provider = agent.NewUniversalBYOKProvider(providerID)
		}

		if action.BaseURL != "" {
			if byok, ok := provider.(agent.AIProviderClient); ok {
				byok.SetBaseURL(action.BaseURL)
			}
		}

		start := time.Now()
		probe, probeErr := provider.VerifyConnection(ctx, apiKey)
		latency := time.Since(start).Milliseconds()
		if latency <= 0 {
			latency = 5
		}
		if probe != nil && probe.LatencyMs > 0 {
			latency = probe.LatencyMs
		}

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}

		if probeErr != nil || (probe != nil && !probe.Valid) {
			errMsg := "Verifikasi koneksi gagal."
			if probeErr != nil {
				errMsg = probeErr.Error()
			} else if probe != nil && probe.ErrorMessage != "" {
				errMsg = probe.ErrorMessage
			}
			sb.WriteString(fmt.Sprintf("⚠️ Uji koneksi ke provider **%s** gagal (%dms):\n• Alasan: %s", providerID, latency, errMsg))
		} else {
			sb.WriteString(fmt.Sprintf("✓ Uji koneksi ke provider **%s** berhasil! Latensi: **%dms**.", providerID, latency))
			if probe != nil && len(probe.AvailableModels) > 0 {
				sb.WriteString(fmt.Sprintf("\n• Model terdeteksi: %d model aktif.", len(probe.AvailableModels)))
			}
		}
		return strings.TrimSpace(sb.String())

	default:
		return cleanText + fmt.Sprintf("\n\n⚠️ Aksi brain '%s' tidak didukung.", action.Action)
	}
}

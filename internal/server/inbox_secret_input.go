package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/mcp"
	"github.com/buchin/cuan/internal/project"
	"github.com/go-chi/chi/v5"
)

const (
	secretInputMarker   = "```cuan-secret\n"
	secretReceiptMarker = "```cuan-secret-receipt\n"
)

// InboxSecretInput represents an interactive masked password input rendered inside an assistant chat bubble.
type InboxSecretInput struct {
	Label       string            `json:"label"`
	Placeholder string            `json:"placeholder,omitempty"`
	SubmitLabel string            `json:"submit_label,omitempty"`
	Target      string            `json:"target,omitempty"` // "mcp_connect", "brain_configure", "project_env", "vault"
	Params      map[string]string `json:"params,omitempty"`
	Submitted   bool              `json:"submitted,omitempty"`
}

type inboxSecretReceipt struct {
	MessageID string `json:"message_id"`
}

func parseInboxSecretInput(content string) (string, *InboxSecretInput, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.LastIndex(trimmed, secretInputMarker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid secret_input action.")
	if !strings.HasSuffix(trimmed, "```") || strings.Count(trimmed, secretInputMarker) != 1 {
		return "", nil, bad
	}
	raw := strings.TrimSpace(trimmed[start+len(secretInputMarker) : len(trimmed)-3])
	if len(raw) > 16000 {
		return "", nil, bad
	}
	var s InboxSecretInput
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&s) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	s.Label = strings.TrimSpace(s.Label)
	if s.Label == "" || len([]rune(s.Label)) > 120 {
		return "", nil, errors.New("Secret input label is required (max 120 characters).")
	}
	s.Placeholder = strings.TrimSpace(s.Placeholder)
	if len([]rune(s.Placeholder)) > 120 {
		return "", nil, errors.New("Secret input placeholder exceeds 120 characters.")
	}
	s.SubmitLabel = strings.TrimSpace(s.SubmitLabel)
	if len([]rune(s.SubmitLabel)) > 40 {
		return "", nil, errors.New("Secret input submit_label exceeds 40 characters.")
	}
	s.Target = strings.ToLower(strings.TrimSpace(s.Target))
	if s.Target == "" {
		s.Target = "vault"
	}
	switch s.Target {
	case "mcp_connect":
		if strings.TrimSpace(s.Params["url"]) == "" {
			return "", nil, errors.New("params.url is required for mcp_connect secret input.")
		}
	case "brain_configure":
		if strings.TrimSpace(s.Params["provider"]) == "" {
			return "", nil, errors.New("params.provider is required for brain_configure secret input.")
		}
	case "project_env":
		if strings.TrimSpace(s.Params["project_id"]) == "" || strings.TrimSpace(s.Params["env_key"]) == "" {
			return "", nil, errors.New("params.project_id and params.env_key are required for project_env secret input.")
		}
	case "vault":
		// General encrypted storage; no required params
	default:
		return "", nil, errors.New("Unsupported secret_input target.")
	}
	return strings.TrimSpace(trimmed[:start]), &s, nil
}

func splitInboxSecretInput(author, content string) (string, *InboxSecretInput) {
	if author != "agent" {
		return content, nil
	}
	clean, secret, err := parseInboxSecretInput(content)
	if err != nil || secret == nil {
		return content, nil
	}
	return clean, secret
}

func splitInboxSecretReceipt(content string) (string, string) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.LastIndex(trimmed, secretReceiptMarker)
	if start < 0 || !strings.HasSuffix(trimmed, "```") || strings.Count(trimmed, secretReceiptMarker) != 1 {
		return content, ""
	}
	raw := strings.TrimSpace(trimmed[start+len(secretReceiptMarker) : len(trimmed)-3])
	var r inboxSecretReceipt
	if json.Unmarshal([]byte(raw), &r) != nil || strings.TrimSpace(r.MessageID) == "" {
		return content, ""
	}
	return strings.TrimSpace(trimmed[:start]), strings.TrimSpace(r.MessageID)
}

type submitInboxSecretRequest struct {
	Value string `json:"value"`
}

// handleSubmitInboxSecret handles POST /api/v1/chat/conversations/{id}/messages/{messageID}/secret.
// It processes sensitive input directly to its secure target without ever storing or echoing the raw secret in chat history.
func (s *Server) handleSubmitInboxSecret(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	convID := strings.TrimSpace(chi.URLParam(r, "id"))
	messageID := strings.TrimSpace(chi.URLParam(r, "messageID"))
	if !db.IsValidUUID(messageID) {
		respondJSONError(w, http.StatusBadRequest, "ID pesan tidak valid")
		return
	}

	resolved := s.resolveConversationContext(r.Context(), convID, "")
	roomID := strings.TrimSpace(resolved.RoomID)
	if roomID == "" {
		roomID = convID
	}
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Penyimpanan percakapan tidak tersedia")
		return
	}

	var req submitInboxSecretRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Format payload JSON tidak valid")
		return
	}
	secretValue := strings.TrimSpace(req.Value)
	if secretValue == "" {
		respondJSONError(w, http.StatusBadRequest, "Nilai rahasia tidak boleh kosong")
		return
	}

	rawMsgs, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), roomID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memuat pesan percakapan")
		return
	}

	var targetMsg *db.ProjectMessage
	alreadySubmitted := false
	for i := range rawMsgs {
		m := &rawMsgs[i]
		if m.ID == messageID {
			targetMsg = m
		}
		if _, receiptMsgID := splitInboxSecretReceipt(m.Content); receiptMsgID == messageID {
			alreadySubmitted = true
		}
	}
	if targetMsg == nil {
		respondJSONError(w, http.StatusNotFound, "Pesan tidak ditemukan")
		return
	}

	_, secretInput := splitInboxSecretInput(targetMsg.AuthorType, targetMsg.Content)
	if secretInput == nil {
		respondJSONError(w, http.StatusBadRequest, "Pesan ini tidak memiliki formulir input rahasia")
		return
	}
	if alreadySubmitted {
		respondJSON(w, http.StatusOK, map[string]any{
			"status":    "already_submitted",
			"submitted": true,
		})
		return
	}

	confirmation, statusCode, processErr := s.processInboxSecretTarget(r.Context(), roomID, messageID, secretInput, secretValue)
	if processErr != nil {
		respondJSONError(w, statusCode, processErr.Error())
		return
	}

	receiptBytes, _ := json.Marshal(inboxSecretReceipt{MessageID: messageID})
	now := time.Now().UTC()
	receiptMsg := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "system",
		AuthorID:   "cuan_secret",
		AuthorName: "Cuan",
		Content:    confirmation + "\n" + secretReceiptMarker + string(receiptBytes) + "\n```",
		ReplyToID:  &messageID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.projectStudioRepo.CreateMessage(r.Context(), receiptMsg); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Kredensial diproses, tetapi gagal mencatat konfirmasi")
		return
	}

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(roomID, "inbox.updated", nil)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"submitted": true,
		"message":   confirmation,
	})
}

func (s *Server) processInboxSecretTarget(ctx context.Context, roomID, messageID string, input *InboxSecretInput, secretValue string) (string, int, error) {
	switch input.Target {
	case "mcp_connect":
		if s.mcpRegistry == nil {
			return "", http.StatusServiceUnavailable, errors.New("MCP registry tidak tersedia")
		}
		name := strings.TrimSpace(input.Params["name"])
		if name == "" {
			name = "Custom MCP Service"
		}
		cfg := mcp.ServerConfig{
			Name:        name,
			BaseURL:     strings.TrimSpace(input.Params["url"]),
			Transport:   mcp.TransportType(strings.TrimSpace(input.Params["transport"])),
			APIKey:      secretValue,
			Description: strings.TrimSpace(input.Params["description"]),
			Enabled:     true,
		}
		app, tools, err := s.mcpRegistry.RegisterRemoteMCP(ctx, cfg)
		if err != nil {
			return "", http.StatusBadRequest, fmt.Errorf("Gagal menghubungkan MCP server: %v", err)
		}
		if err := s.persistRemoteMCP(ctx, cfg, app); err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("MCP terhubung, tetapi gagal menyimpan konfigurasi terenkripsi: %v", err)
		}
		return fmt.Sprintf("✓ **Konektor MCP Terhubung:** **%s** (`%s`) dengan %d tools aktif siap digunakan.", app.Name, app.ID, len(tools)), http.StatusOK, nil

	case "brain_configure":
		store := s.AIProviderStore()
		if store == nil {
			return "", http.StatusServiceUnavailable, errors.New("AI Provider Store tidak tersedia")
		}
		providerID := strings.ToLower(strings.TrimSpace(input.Params["provider"]))
		cfg := store.GetConfig(providerID)
		if mode := strings.ToLower(strings.TrimSpace(input.Params["mode"])); mode != "" {
			cfg.Mode = mode
		}
		if baseURL := strings.TrimSpace(input.Params["base_url"]); baseURL != "" {
			cfg.BaseURL = baseURL
		}
		if strings.EqualFold(strings.TrimSpace(input.Params["enabled"]), "true") {
			cfg.Enabled = true
		} else if cfg.Mode == "" {
			cfg.Enabled = true
		}
		encKey, err := agent.EncryptCredential(secretValue)
		if err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("Gagal mengenkripsi kredensial: %v", err)
		}
		cfg.EncryptedAPIKey = encKey
		if err := store.SaveConfig(cfg); err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("Gagal menyimpan konfigurasi provider: %v", err)
		}
		return fmt.Sprintf("✓ Kredensial provider AI **%s** berhasil disimpan secara terenkripsi.", providerID), http.StatusOK, nil

	case "project_env":
		if s.projectSvc == nil {
			return "", http.StatusServiceUnavailable, errors.New("Project service tidak tersedia")
		}
		projID := strings.TrimSpace(input.Params["project_id"])
		envKey := strings.TrimSpace(input.Params["env_key"])
		curr, err := s.projectSvc.Get(ctx, projID)
		if err != nil || curr == nil {
			return "", http.StatusNotFound, errors.New("Project tidak ditemukan")
		}
		settings := project.SettingsFromMetadata(curr.Metadata)
		envMap := parseEnvLines(settings.Environment)
		envMap[envKey] = secretValue
		_, err = s.projectSvc.UpdateSettings(ctx, curr.ID, project.UpdateSettingsRequest{
			Name:              curr.Name,
			RepoURL:           curr.RepoURL,
			DefaultBranch:     curr.DefaultBranch,
			Goal:              settings.Goal,
			Instructions:      settings.Instructions,
			DeployBranch:      settings.DeployBranch,
			DeployScript:      settings.DeployScript,
			AutoDeploy:        settings.AutoDeploy,
			Environment:       serializeEnvLines(envMap),
			ExpectedUpdatedAt: curr.UpdatedAt,
		})
		if err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("Gagal menyimpan environment variable: %v", err)
		}
		return fmt.Sprintf("✓ Environment variable `%s` untuk project **%s** berhasil disimpan dengan aman.", envKey, curr.Name), http.StatusOK, nil

	default:
		encrypted, err := crypto.EncryptStringAES256GCM(secretValue, s.remoteMCPSecret())
		if err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("Gagal mengenkripsi kredensial: %v", err)
		}
		s.secretVault.Store(roomID+":"+messageID, encrypted)
		return fmt.Sprintf("✓ **%s** berhasil dikirim dan disimpan secara terenkripsi.", input.Label), http.StatusOK, nil
	}
}

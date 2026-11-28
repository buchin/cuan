package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/mcp"
)

func TestSplitInboxSecretInput(t *testing.T) {
	raw := "Silakan masukkan API key Tebar MCP.\n```cuan-secret\n{\"label\":\"API Key Tebar MCP\",\"placeholder\":\"Tempel API key...\",\"target\":\"mcp_connect\",\"params\":{\"url\":\"https://api.tebar.app/mcp\",\"name\":\"Tebar MCP\"}}\n```"

	// Non-agent author must be ignored
	cleanUser, secUser := splitInboxSecretInput("user", raw)
	if secUser != nil || cleanUser != raw {
		t.Fatalf("expected non-agent message to ignore cuan-secret")
	}

	clean, sec := splitInboxSecretInput("agent", raw)
	if sec == nil {
		t.Fatalf("expected parsed secret input")
	}
	if clean != "Silakan masukkan API key Tebar MCP." {
		t.Fatalf("unexpected clean text: %q", clean)
	}
	if sec.Label != "API Key Tebar MCP" || sec.Target != "mcp_connect" || sec.Params["url"] != "https://api.tebar.app/mcp" {
		t.Fatalf("unexpected secret input: %+v", sec)
	}

	// Missing url for mcp_connect must be rejected
	bad := "Masukkan key.\n```cuan-secret\n{\"label\":\"API Key\",\"target\":\"mcp_connect\"}\n```"
	if _, parsed := splitInboxSecretInput("agent", bad); parsed != nil {
		t.Fatalf("expected invalid mcp_connect without url to be rejected")
	}
}

func TestDecodeInboxCoordinatorResponseSecretInput(t *testing.T) {
	payload := map[string]any{
		"reply":       "Masukkan API key Anda di bawah ini.",
		"topic_title": "",
		"action": map[string]any{
			"kind": "secret_input",
			"payload": map[string]any{
				"label":       "OpenAI API Key",
				"placeholder": "sk-...",
				"target":      "brain_configure",
				"params": map[string]string{
					"provider": "openai",
				},
			},
		},
	}
	rawBytes, _ := json.Marshal(payload)
	out, err := decodeInboxCoordinatorResponse(string(rawBytes))
	if err != nil {
		t.Fatalf("decodeInboxCoordinatorResponse failed: %v", err)
	}
	clean, sec := splitInboxSecretInput("agent", out)
	if sec == nil || sec.Target != "brain_configure" || sec.Params["provider"] != "openai" {
		t.Fatalf("unexpected decoded secret_input: %+v", sec)
	}
	if clean != "Masukkan API key Anda di bawah ini." {
		t.Fatalf("unexpected reply: %q", clean)
	}
}

func TestHandleSubmitInboxSecret_MCPConnectAndNoPlaintextInChat(t *testing.T) {
	const testSecret = "sk-super-secret-tebar-998877"
	var seenAuth string
	mockMCP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		if seenAuth != "Bearer "+testSecret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"Tebar","version":"1.0"}}}`))
		case "tools/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"send_wa","description":"Send WhatsApp","inputSchema":{"type":"object"}}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{}}`))
		}
	}))
	defer mockMCP.Close()

	srv, token := setupTestChatServer(t)
	srv.mcpRegistry = mcp.NewRegistry()
	repo := srv.projectStudioRepo

	roomID := db.GenerateUUID()
	msgID := db.GenerateUUID()
	_ = repo.CreateRoom(context.Background(), &db.ProjectRoom{ID: roomID, ProjectID: "default", Title: "Tebar"})

	secretBlock, _ := json.Marshal(InboxSecretInput{
		Label:       "API Key Tebar MCP",
		Placeholder: "Masukkan API key...",
		Target:      "mcp_connect",
		Params: map[string]string{
			"url":  mockMCP.URL,
			"name": "Tebar MCP",
		},
	})
	_ = repo.CreateMessage(context.Background(), &db.ProjectMessage{
		ID:         msgID,
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   "agent_cuan",
		AuthorName: "Cuan",
		Content:    "Masukkan API key Tebar MCP di bawah ini.\n```cuan-secret\n" + string(secretBlock) + "\n```",
		CreatedAt:  time.Now().UTC(),
	})

	bodyBytes, _ := json.Marshal(submitInboxSecretRequest{Value: testSecret})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+msgID+"/secret", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if seenAuth != "Bearer "+testSecret {
		t.Fatalf("expected mock MCP to receive Bearer secret, got %q", seenAuth)
	}

	// Verify raw secret is NEVER stored in any chat message
	msgs, err := repo.ListMessagesByRoomID(context.Background(), roomID)
	if err != nil {
		t.Fatalf("ListMessagesByRoomID failed: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m.Content, testSecret) {
			t.Fatalf("SECURITY VIOLATION: raw secret found in persisted chat message %s", m.ID)
		}
	}

	// Verify formatted DTO marks secret_input as submitted and strips fences
	dtos := srv.getFormattedMessages(context.Background(), roomID, "")
	if len(dtos) != 2 {
		t.Fatalf("expected 2 formatted messages, got %d", len(dtos))
	}
	if dtos[0].SecretInput == nil || !dtos[0].SecretInput.Submitted {
		t.Fatalf("expected first message to have secret_input.submitted=true, got %+v", dtos[0].SecretInput)
	}
	if strings.Contains(dtos[1].Content, "cuan-secret-receipt") {
		t.Fatalf("expected receipt fence stripped from confirmation message, got %q", dtos[1].Content)
	}

	// Verify idempotent re-submission
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+msgID+"/secret", bytes.NewReader(bodyBytes))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), "already_submitted") {
		t.Fatalf("expected idempotent already_submitted response, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestHandleSubmitInboxSecret_BrainAndVault(t *testing.T) {
	srv, token := setupTestChatServer(t)
	store := agent.NewAIProviderStore(filepath.Join(t.TempDir(), "providers.json"))
	srv.aiProviderStore = store
	repo := srv.projectStudioRepo

	roomID := db.GenerateUUID()
	vaultMsgID := db.GenerateUUID()
	brainMsgID := db.GenerateUUID()
	_ = repo.CreateRoom(context.Background(), &db.ProjectRoom{ID: roomID, ProjectID: "default", Title: "Secrets"})

	_ = repo.CreateMessage(context.Background(), &db.ProjectMessage{
		ID:         vaultMsgID,
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   "agent_cuan",
		AuthorName: "Cuan",
		Content:    "Masukkan password VPS.\n```cuan-secret\n{\"label\":\"Password VPS\",\"target\":\"vault\"}\n```",
		CreatedAt:  time.Now().UTC(),
	})
	_ = repo.CreateMessage(context.Background(), &db.ProjectMessage{
		ID:         brainMsgID,
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   "agent_cuan",
		AuthorName: "Cuan",
		Content:    "Masukkan OpenAI API Key.\n```cuan-secret\n{\"label\":\"OpenAI Key\",\"target\":\"brain_configure\",\"params\":{\"provider\":\"openai\"}}\n```",
		CreatedAt:  time.Now().UTC(),
	})

	// 1. Vault target
	const vpsPass = "vps-root-pass-123!"
	body1, _ := json.Marshal(submitInboxSecretRequest{Value: vpsPass})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+vaultMsgID+"/secret", bytes.NewReader(body1))
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for vault, got %d: %s", rec1.Code, rec1.Body.String())
	}
	rawEnc, ok := srv.secretVault.Load(roomID + ":" + vaultMsgID)
	if !ok {
		t.Fatalf("expected encrypted secret stored in secretVault")
	}
	decrypted, err := crypto.DecryptStringAES256GCM(rawEnc.(string), srv.remoteMCPSecret())
	if err != nil || decrypted != vpsPass {
		t.Fatalf("expected decrypted secret %q, got %q (err=%v)", vpsPass, decrypted, err)
	}

	// 2. Brain configure target
	const openaiKey = "sk-openai-test-987654"
	body2, _ := json.Marshal(submitInboxSecretRequest{Value: openaiKey})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+brainMsgID+"/secret", bytes.NewReader(body2))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for brain_configure, got %d: %s", rec2.Code, rec2.Body.String())
	}
	storedKey, err := store.GetDecryptedAPIKey("openai")
	if err != nil || storedKey != openaiKey {
		t.Fatalf("expected stored openai key %q, got %q (err=%v)", openaiKey, storedKey, err)
	}

	// Verify neither secret appears in any chat message
	msgs, _ := repo.ListMessagesByRoomID(context.Background(), roomID)
	for _, m := range msgs {
		if strings.Contains(m.Content, vpsPass) || strings.Contains(m.Content, openaiKey) {
			t.Fatalf("SECURITY VIOLATION: raw secret found in chat message %s", m.ID)
		}
	}
}

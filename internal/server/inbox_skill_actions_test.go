package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/mcp"
)

func TestParseInboxSkill(t *testing.T) {
	// 1. Valid toggle enable
	valid := "Aktifkan skill receipt extractor.\n```cuan-skill\n{\"action\":\"toggle\",\"agent_id\":\"agent_dev\",\"skill_id\":\"vision_receipt_extractor\",\"enabled\":true}\n```"
	clean, act, err := parseInboxSkill(valid)
	if err != nil || act == nil {
		t.Fatalf("expected valid skill parse, got: %v", err)
	}
	if clean != "Aktifkan skill receipt extractor." {
		t.Fatalf("unexpected clean text: %s", clean)
	}
	if act.Action != "toggle" || act.AgentID != "agent_dev" || act.SkillID != "vision_receipt_extractor" || act.Enabled == nil || !*act.Enabled {
		t.Fatalf("unexpected parsed action: %+v", act)
	}

	// 2. Valid install
	validInstall := "Pasang skill customer service.\n```cuan-skill\n{\"action\":\"install\",\"name\":\"CS Shopee\",\"content\":\"# SOP CS\"}\n```"
	clean, act, err = parseInboxSkill(validInstall)
	if err != nil || act == nil || act.Action != "install" || act.Name != "CS Shopee" {
		t.Fatalf("unexpected install parsed: %+v, err: %v", act, err)
	}

	// 3. Valid connect_mcp
	validMCP := "Hubungkan layanan Tebar.\n```cuan-skill\n{\"action\":\"connect_mcp\",\"name\":\"Tebar WA\",\"url\":\"https://api.tebar.app\"}\n```"
	clean, act, err = parseInboxSkill(validMCP)
	if err != nil || act == nil || act.Action != "connect_mcp" || act.URL != "https://api.tebar.app" {
		t.Fatalf("unexpected connect_mcp parsed: %+v, err: %v", act, err)
	}

	// 4. Invalid missing agent_id on toggle
	badAgent := "```cuan-skill\n{\"action\":\"toggle\",\"agent_id\":\"\",\"skill_id\":\"vision_receipt_extractor\",\"enabled\":true}\n```"
	if _, _, err := parseInboxSkill(badAgent); err == nil {
		t.Fatal("expected error for empty agent_id")
	}

	// 5. Invalid missing url/content on install
	badInstall := "```cuan-skill\n{\"action\":\"install\",\"url\":\"\"}\n```"
	if _, _, err := parseInboxSkill(badInstall); err == nil {
		t.Fatal("expected error for empty install params")
	}
}

func TestApplyInboxSkill(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestChatServer(t)
	s.mcpRegistry = mcp.NewRegistry()
	s.mcpRegistry.SetSkillsDir(t.TempDir())

	q := db.InboxRequest{
		RoomID:    "room-test",
		OwnerID:   "owner-1",
		MessageID: "msg-1",
	}

	// 1. Toggle skill to false
	toggleOff := "Nonaktifkan skill.\n```cuan-skill\n{\"action\":\"toggle\",\"agent_id\":\"agent_dev\",\"skill_id\":\"vision_receipt_extractor\",\"enabled\":false}\n```"
	reply := s.applyInboxSkill(ctx, q, toggleOff)
	if !strings.Contains(reply, "Skill Diperbarui") || !strings.Contains(reply, "dinonaktifkan") || !strings.Contains(reply, "vision_receipt_extractor") {
		t.Fatalf("unexpected toggle off reply: %s", reply)
	}

	// 2. Toggle skill to true
	toggleOn := "Aktifkan kembali skill.\n```cuan-skill\n{\"action\":\"toggle\",\"agent_id\":\"agent_dev\",\"skill_id\":\"vision_receipt_extractor\",\"enabled\":true}\n```"
	replyOn := s.applyInboxSkill(ctx, q, toggleOn)
	if !strings.Contains(replyOn, "Skill Diperbarui") || !strings.Contains(replyOn, "diaktifkan") {
		t.Fatalf("unexpected toggle on reply: %s", replyOn)
	}

	// 3. Install skill from content
	installContent := "Pasang skill copywriting.\n```cuan-skill\n{\"action\":\"install\",\"name\":\"Copywriting Pro\",\"content\":\"# SOP Copywriting\"}\n```"
	replyInstall := s.applyInboxSkill(ctx, q, installContent)
	if !strings.Contains(replyInstall, "Skill Baru Berhasil Dipasang") || !strings.Contains(replyInstall, "Copywriting Pro") {
		t.Fatalf("unexpected install reply: %s", replyInstall)
	}

	// 4. Connect Remote MCP
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp/tools" {
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []mcp.ToolDefinition{
					{Name: "tebar_send", Description: "Send WA"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	connectContent := "Hubungkan konektor.\n```cuan-skill\n{\"action\":\"connect_mcp\",\"name\":\"Tebar WA\",\"url\":\"" + mockServer.URL + "\"}\n```"
	replyConnect := s.applyInboxSkill(ctx, q, connectContent)
	if !strings.Contains(replyConnect, "Konektor MCP Terhubung") || !strings.Contains(replyConnect, "Tebar WA") {
		t.Fatalf("unexpected connect reply: %s", replyConnect)
	}

	// 5. Uninstall skill
	uninstallContent := "Hapus skill.\n```cuan-skill\n{\"action\":\"uninstall\",\"skill_id\":\"copywriting_pro\"}\n```"
	replyUninstall := s.applyInboxSkill(ctx, q, uninstallContent)
	if !strings.Contains(replyUninstall, "Skill Berhasil Dihapus") {
		t.Fatalf("unexpected uninstall reply: %s", replyUninstall)
	}
}

func TestApplyInboxSkill_ConnectMCPAuthRequiredAndSuccess(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestChatServer(t)
	s.mcpRegistry = mcp.NewRegistry()
	s.mcpRegistry.SetSkillsDir(t.TempDir())

	q := db.InboxRequest{
		RoomID:    "room-test-auth",
		OwnerID:   "owner-1",
		MessageID: "msg-auth-1",
	}

	tebarMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tb_live_ok" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized","error_description":"Send Authorization: Bearer <API key or OAuth access token>"}`))
			return
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"protocolVersion": "2025-03-26"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"tools": []map[string]any{
						{"name": "tebar_post", "description": "Create Tebar post"},
					},
				},
			})
		}
	}))
	defer tebarMock.Close()

	// 1. Missing api_key -> user-friendly prompt asking for API key / Bearer token (no raw 401 dump)
	noKeyContent := "Sambungkan Tebar MCP.\n```cuan-skill\n{\"action\":\"connect_mcp\",\"name\":\"Tebar MCP\",\"url\":\"" + tebarMock.URL + "/mcp\"}\n```"
	noKeyReply := s.applyInboxSkill(ctx, q, noKeyContent)
	if !strings.Contains(noKeyReply, "memerlukan autentikasi") || !strings.Contains(noKeyReply, "API key") || !strings.Contains(noKeyReply, "Bearer token") {
		t.Fatalf("expected user-friendly auth prompt, got: %s", noKeyReply)
	}
	if strings.Contains(noKeyReply, "mcp initialize failed") || strings.Contains(noKeyReply, "status 401") {
		t.Fatalf("expected raw 401 error to be hidden from user reply, got: %s", noKeyReply)
	}

	// 2. Wrong api_key -> user-friendly invalid/expired token prompt
	badKeyContent := "Sambungkan Tebar MCP.\n```cuan-skill\n{\"action\":\"connect_mcp\",\"name\":\"Tebar MCP\",\"url\":\"" + tebarMock.URL + "/mcp\",\"api_key\":\"tb_wrong\"}\n```"
	badKeyReply := s.applyInboxSkill(ctx, q, badKeyContent)
	if !strings.Contains(badKeyReply, "tidak valid") || !strings.Contains(badKeyReply, "API key") {
		t.Fatalf("expected invalid key prompt, got: %s", badKeyReply)
	}

	// 3. Valid api_key -> connects and persists encrypted in DB
	okContent := "Sambungkan Tebar MCP.\n```cuan-skill\n{\"action\":\"connect_mcp\",\"name\":\"Tebar MCP\",\"url\":\"" + tebarMock.URL + "/mcp\",\"api_key\":\"tb_live_ok\"}\n```"
	okReply := s.applyInboxSkill(ctx, q, okContent)
	if !strings.Contains(okReply, "Konektor MCP Terhubung") || !strings.Contains(okReply, "Tebar MCP") {
		t.Fatalf("expected connected reply, got: %s", okReply)
	}
}

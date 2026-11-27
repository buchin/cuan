package diagnostics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInboxChatMCP_ExposesAllNativeToolsAndValidSchemas(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	raw, _ := json.Marshal(InboxChatSession{
		OwnerID:    "owner_1",
		RoomID:     "room_1",
		RequestID:  "req_1",
		APIURL:     "http://127.0.0.1:1",
		AuthToken:  "tok_1",
		ObservedAt: time.Now().UTC(),
	})
	if err := os.WriteFile(sessionPath, raw, 0600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunInboxChatMCP(sessionPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &out, &errOut)
	if code != 0 {
		t.Fatalf("RunInboxChatMCP failed (%d): %s", code, errOut.String())
	}

	var resp struct {
		Result struct {
			Tools []mcpToolDefinition `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("invalid tools/list JSON: %v (%s)", err, out.String())
	}
	if len(resp.Result.Tools) != len(nativeMCPTools) {
		t.Fatalf("expected %d tools, got %d", len(nativeMCPTools), len(resp.Result.Tools))
	}

	seen := map[string]bool{}
	for _, tool := range resp.Result.Tools {
		seen[tool.Name] = true
		if strings.TrimSpace(tool.Name) == "" || strings.TrimSpace(tool.Description) == "" {
			t.Fatalf("tool %+v has empty name or description", tool)
		}
		if typ, _ := tool.InputSchema["type"].(string); typ != "object" {
			t.Fatalf("tool %s schema type = %q, want object", tool.Name, typ)
		}
	}
	for _, expected := range []string{"list_inbox_conversations", "get_inbox_messages", "list_tasks", "get_task", "list_projects", "get_contact"} {
		if !seen[expected] {
			t.Fatalf("missing expected tool %s", expected)
		}
	}
}

func TestInboxChatMCP_AuthenticatedCallsAndRoomTaskInspection(t *testing.T) {
	t.Setenv("CUAN_AUTH_TOKEN", "ambient-leaked-token")
	t.Setenv("DATABASE_URL", "postgres://invalid:invalid@127.0.0.1:1/invalid")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer scoped-chat-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/chat/contacts":
			_, _ = w.Write([]byte(`{"contacts":[{"conversation_id":"room-xyz","name":"Regresi Snapshot Verifier","project_id":"proj_1","task_count":2,"active_tasks":1}]}`))
		case "/api/v1/chat/conversations/room-xyz/messages":
			_, _ = w.Write([]byte(`{"messages":[{"id":"msg-1","room_id":"room-xyz","author_type":"user","author_name":"User","content":"cek status"}]}`))
		case "/api/v1/tasks":
			_, _ = w.Write([]byte(`{"tasks":[{"id":"task_1","project_id":"proj_1","title":"Fix MCP","status":"doing","priority":"high"}]}`))
		case "/api/v1/contacts":
			_, _ = w.Write([]byte(`{"contacts":[{"id":"contact_cur","session_uuid":"room-xyz","name":"Budi","email":"budi@example.invalid"}]}`))
		case "/api/v1/contacts/contact_cur/memory":
			_, _ = w.Write([]byte(`{"items":[],"has_more":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	workspace := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	raw, _ := json.Marshal(InboxChatSession{
		OwnerID:          "owner_1",
		RoomID:           "room-xyz",
		RequestID:        "req_1",
		Workspace:        workspace,
		CurrentContactID: "contact_cur",
		APIURL:           srv.URL,
		AuthToken:        "scoped-chat-token",
		ObservedAt:       time.Now().UTC(),
	})
	if err := os.WriteFile(sessionPath, raw, 0600); err != nil {
		t.Fatal(err)
	}

	requests := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_inbox_conversations","arguments":{"limit":10}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_inbox_messages","arguments":{"room_id":"room-xyz","limit":10}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_tasks","arguments":{"limit":10}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_contact","arguments":{"contact_id":"current"}}}`,
	}, "\n") + "\n"

	var out, errOut bytes.Buffer
	code := RunInboxChatMCP(sessionPath, strings.NewReader(requests), &out, &errOut)
	if code != 0 {
		t.Fatalf("RunInboxChatMCP failed (%d): %s", code, errOut.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 responses, got %d: %s", len(lines), out.String())
	}
	expectedSubstrings := []string{
		"Regresi Snapshot Verifier",
		"cek status",
		"Fix MCP",
		"budi@example.invalid",
	}
	for i, want := range expectedSubstrings {
		if strings.Contains(lines[i], `"isError":true`) || !strings.Contains(lines[i], want) {
			t.Fatalf("response %d failed: want %q in %s", i+1, want, lines[i])
		}
	}
}

func TestInboxChatMCP_RejectsExpiredOrInvalidSessionAndCrossWorkspaceEscape(t *testing.T) {
	dir := t.TempDir()
	expiredPath := filepath.Join(dir, "expired.json")
	rawExpired, _ := json.Marshal(InboxChatSession{
		OwnerID:    "owner_1",
		RoomID:     "room_1",
		RequestID:  "req_1",
		APIURL:     "http://127.0.0.1:1",
		AuthToken:  "tok_1",
		ObservedAt: time.Now().Add(-30 * time.Minute),
	})
	_ = os.WriteFile(expiredPath, rawExpired, 0600)

	var out, errOut bytes.Buffer
	if code := RunInboxChatMCP(expiredPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &out, &errOut); code == 0 {
		t.Fatal("expected expired session to be rejected")
	}

	workspace := filepath.Join(dir, "ws-a")
	otherWorkspace := filepath.Join(dir, "ws-b")
	_ = os.MkdirAll(workspace, 0700)
	_ = os.MkdirAll(otherWorkspace, 0700)

	validPath := filepath.Join(dir, "valid.json")
	rawValid, _ := json.Marshal(InboxChatSession{
		OwnerID:    "owner_1",
		RoomID:     "room_1",
		RequestID:  "req_1",
		Workspace:  workspace,
		APIURL:     "http://127.0.0.1:1",
		AuthToken:  "tok_1",
		ObservedAt: time.Now().UTC(),
	})
	_ = os.WriteFile(validPath, rawValid, 0600)

	out.Reset()
	errOut.Reset()
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"inspect_git_status","arguments":{"path":"` + otherWorkspace + `"}}}` + "\n"
	if code := RunInboxChatMCP(validPath, strings.NewReader(req), &out, &errOut); code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"isError":true`) || !strings.Contains(out.String(), "outside the authorized workspace") {
		t.Fatalf("expected cross-workspace escape rejection, got: %s", out.String())
	}
}

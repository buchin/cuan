package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/diagnostics"
)

func TestInboxChatSession_PerTurnEphemeralAuthAndSandboxIsolation(t *testing.T) {
	s, ownerToken := setupTestChatServer(t)
	owner, err := s.authSvc.ValidateToken(context.Background(), ownerToken)
	if err != nil || owner == nil {
		t.Fatalf("failed to validate owner token: %v", err)
	}

	apiSrv := httptest.NewServer(s.Router())
	defer apiSrv.Close()
	s.cfg.RunnerServerURL = apiSrv.URL

	workspace := t.TempDir()
	ctx := context.WithValue(context.Background(), tasteMemberKey{}, owner.ID)
	ctx = context.WithValue(ctx, inboxRequestTraceKey{}, inboxRequestTrace{
		RoomID:    "room-chat-test",
		RequestID: "req-chat-test",
		AgentID:   "agent_cuan",
	})

	sessionPath, cleanup, err := s.prepareInboxChatSession(ctx, workspace, "")
	if err != nil {
		t.Fatalf("prepareInboxChatSession failed: %v", err)
	}
	if sessionPath == "" {
		t.Fatal("expected non-empty chat session path")
	}

	info, err := os.Stat(sessionPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 session file, got err=%v mode=%v", err, info.Mode().Perm())
	}
	raw, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	var session diagnostics.InboxChatSession
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatal(err)
	}
	if session.OwnerID != owner.ID || session.Workspace != workspace || session.APIURL != apiSrv.URL || session.AuthToken == "" {
		t.Fatalf("unexpected session payload: %+v", session)
	}
	if session.AuthToken == ownerToken {
		t.Fatal("chat session must mint an ephemeral token rather than reusing ownerToken")
	}

	// Verify the ephemeral token authenticates against the live server API.
	req, _ := http.NewRequest(http.MethodGet, apiSrv.URL+"/api/v1/chat/contacts", nil)
	req.Header.Set("Authorization", "Bearer "+session.AuthToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK with ephemeral token, got %d", resp.StatusCode)
	}

	// Verify sandbox args mount /cuan-inbox-chat.json and mcp_config.json uses --inbox-chat without auth_token.
	home := t.TempDir()
	args, err := inboxSandboxArgsWithChat("/bin/sh", home, workspace, nil, "", sessionPath)
	if err != nil {
		t.Fatalf("inboxSandboxArgsWithChat failed: %v", err)
	}
	joined := strings.Join(args, " ")
	canonicalSession, _ := filepath.EvalSymlinks(sessionPath)
	if !strings.Contains(joined, "--ro-bind "+sessionPath+" /cuan-inbox-chat.json") &&
		!strings.Contains(joined, "--ro-bind "+canonicalSession+" /cuan-inbox-chat.json") {
		t.Fatalf("expected /cuan-inbox-chat.json mount in sandbox args: %s", joined)
	}

	mcpConfigPath, err := prepareInboxChatMCPBoundary(home)
	if err != nil {
		t.Fatal(err)
	}
	mcpRaw, _ := os.ReadFile(mcpConfigPath)
	if !strings.Contains(string(mcpRaw), "--inbox-chat") || strings.Contains(string(mcpRaw), "auth_token") || strings.Contains(string(mcpRaw), session.AuthToken) {
		t.Fatalf("invalid chat MCP config: %s", string(mcpRaw))
	}

	// Cleanup must delete the session file and revoke the ephemeral token.
	cleanup()
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatal("expected session file to be removed on cleanup")
	}
	req2, _ := http.NewRequest(http.MethodGet, apiSrv.URL+"/api/v1/chat/contacts", nil)
	req2.Header.Set("Authorization", "Bearer "+session.AuthToken)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after cleanup revocation, got %d", resp2.StatusCode)
	}
}

func TestInboxRuntimeStream_AllowsAllNativeMCPTools(t *testing.T) {
	tools := []string{
		"cuan-inspect/list_inbox_conversations",
		"cuan-inspect/get_inbox_messages",
		"cuan-inspect/delete_inbox_message",
		"cuan-inspect/list_tasks",
		"cuan-inspect/get_task",
		"cuan-inspect/list_projects",
		"cuan-inspect/get_project",
		"cuan-inspect/list_project_deployments",
		"cuan-inspect/get_project_deployment",
		"cuan-inspect/list_provider_accounts",
		"cuan-inspect/list_team_members",
		"cuan-inspect/get_brand_memory",
	}
	for _, tool := range tools {
		if !isSafeInboxTool(tool) {
			t.Fatalf("expected %s to be allowed by isSafeInboxTool", tool)
		}
	}
}

package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillMarkdown(t *testing.T) {
	raw := `---
name: Shopee CS Expert
description: Ahli membalas chat customer shopee dengan ramah.
category: customer_service
icon: message-square
tools:
  - shopee_send_chat
  - shopee_check_order
---

# Shopee CS Instructions

1. Selalu awali dengan sapaan ramah.
2. Cek status order sebelum menjawab komplain.
`
	meta, body := parseSkillMarkdown(raw)
	if meta["name"] != "Shopee CS Expert" {
		t.Fatalf("expected name 'Shopee CS Expert', got: %v", meta["name"])
	}
	if meta["description"] != "Ahli membalas chat customer shopee dengan ramah." {
		t.Fatalf("unexpected description: %v", meta["description"])
	}
	if meta["category"] != "customer_service" {
		t.Fatalf("unexpected category: %v", meta["category"])
	}
	tools, ok := meta["tools"].([]string)
	if !ok || len(tools) != 2 || tools[0] != "shopee_send_chat" || tools[1] != "shopee_check_order" {
		t.Fatalf("unexpected tools parsed: %v", meta["tools"])
	}
	if !strings.Contains(body, "# Shopee CS Instructions") {
		t.Fatalf("unexpected body: %s", body)
	}

	// Without frontmatter
	noFm := "# My Custom Prompt\nLakukan analisa mendalam."
	meta2, body2 := parseSkillMarkdown(noFm)
	if meta2["name"] != "My Custom Prompt" {
		t.Fatalf("expected name from heading: %v", meta2["name"])
	}
	if body2 != noFm {
		t.Fatalf("unexpected body: %s", body2)
	}
}

func TestInstallAndUninstallSkill(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	tempDir := t.TempDir()
	reg.SetSkillsDir(tempDir)

	// Install skill from content
	skill, err := reg.InstallSkillFromContent(
		ctx,
		"skill_test",
		"Test Skill",
		"Deskripsi test skill",
		"testing",
		"# Body content",
		[]string{"agent_dev"},
	)
	if err != nil {
		t.Fatalf("failed to install skill: %v", err)
	}
	if skill.ID != "skill_test" || skill.Name != "Test Skill" {
		t.Fatalf("unexpected skill returned: %+v", skill)
	}

	// Check file written to disk
	expectedFile := filepath.Join(tempDir, "skill_test", "SKILL.md")
	data, err := os.ReadFile(expectedFile)
	if err != nil || !strings.Contains(string(data), "Test Skill") {
		t.Fatalf("expected SKILL.md written to %s, err: %v", expectedFile, err)
	}

	// Check agent binding
	bindings, err := reg.GetAgentSkills("agent_dev")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range bindings {
		if b.SkillID == "skill_test" && b.Enabled {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected skill_test to be enabled for agent_dev")
	}

	// Uninstall skill
	if err := reg.UninstallSkill(ctx, "skill_test"); err != nil {
		t.Fatalf("failed to uninstall skill: %v", err)
	}
	if _, err := reg.GetSkill("skill_test"); err == nil {
		t.Fatal("expected skill to be deleted from registry")
	}
	if _, err := os.Stat(filepath.Join(tempDir, "skill_test")); !os.IsNotExist(err) {
		t.Fatal("expected skill directory to be deleted from disk")
	}
}

func TestRegisterRemoteMCP(t *testing.T) {
	ctx := context.Background()

	// Mock remote MCP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp/tools" && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]any{
				"tools": []ToolDefinition{
					{
						Name:        "remote_ping",
						Description: "Ping remote server",
					},
					{
						Name:        "remote_send_message",
						Description: "Send message via remote provider",
					},
				},
			})
			return
		}
		if r.URL.Path == "/mcp/tools/call" && r.Method == http.MethodPost {
			var req ToolCallRequest
			json.NewDecoder(r.Body).Decode(&req)
			json.NewEncoder(w).Encode(ToolCallResponse{
				IsError: false,
				Content: []ToolContent{
					{Type: "text", Text: "Pong from remote server for tool " + req.ToolName},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	reg := NewRegistry()
	cfg := ServerConfig{
		ID:        "mock_tebar",
		Name:      "Mock Tebar Service",
		BaseURL:   server.URL,
		Transport: TransportHTTP,
	}

	app, tools, err := reg.RegisterRemoteMCP(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to register remote MCP: %v", err)
	}
	if app.ID != "mock_tebar" || len(tools) != 2 {
		t.Fatalf("unexpected app or tools: %+v, tools: %v", app, tools)
	}

	// Execute discovered tool via registry
	resp, err := reg.ExecuteTool(ctx, "remote_ping", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("failed to execute remote tool: %v", err)
	}
	if resp.IsError || len(resp.Content) == 0 || !strings.Contains(resp.Content[0].Text, "Pong from remote") {
		t.Fatalf("unexpected tool response: %+v", resp)
	}

	// Companion skill exists
	compSkill, err := reg.GetSkill("skill_mock_tebar")
	if err != nil || len(compSkill.Tools) != 2 {
		t.Fatalf("companion skill missing or invalid: %+v, err: %v", compSkill, err)
	}

	// Unregister
	if err := reg.UnregisterRemoteMCP(ctx, "mock_tebar"); err != nil {
		t.Fatalf("failed to unregister remote MCP: %v", err)
	}
	if _, err := reg.GetApp("mock_tebar"); err == nil {
		t.Fatal("expected app to be deleted")
	}
	if _, err := reg.ExecuteTool(ctx, "remote_ping", nil); err == nil {
		t.Fatal("expected remote tool to be unregistered")
	}
}

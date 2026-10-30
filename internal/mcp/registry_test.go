package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRegistry_AppsLifecycle(t *testing.T) {
	reg := NewRegistry()

	apps := reg.ListApps()
	if len(apps) == 0 {
		t.Fatalf("expected at least 1 app, got 0")
	}

	app, err := reg.GetApp("google_sheets")
	if err != nil {
		t.Fatalf("unexpected error getting google_sheets app: %v", err)
	}
	if app.Status != "disconnected" {
		t.Errorf("expected initial status disconnected, got %s", app.Status)
	}

	// Connect app
	connected, err := reg.ConnectApp("google_sheets", "warung.berkah@gmail.com")
	if err != nil {
		t.Fatalf("failed to connect app: %v", err)
	}
	if connected.Status != "connected" {
		t.Errorf("expected status connected, got %s", connected.Status)
	}
	if connected.Account != "warung.berkah@gmail.com" {
		t.Errorf("expected account warung.berkah@gmail.com, got %s", connected.Account)
	}
	if connected.ConnectedAt == nil {
		t.Errorf("expected non-nil connected_at")
	}

	// Disconnect app
	disconnected, err := reg.DisconnectApp("google_sheets")
	if err != nil {
		t.Fatalf("failed to disconnect app: %v", err)
	}
	if disconnected.Status != "disconnected" {
		t.Errorf("expected status disconnected, got %s", disconnected.Status)
	}
	if disconnected.Account != "" {
		t.Errorf("expected empty account, got %s", disconnected.Account)
	}
}

func TestRegistry_SkillsAndAgentBindings(t *testing.T) {
	reg := NewRegistry()

	skills := reg.ListSkills()
	if len(skills) < 2 {
		t.Fatalf("expected at least 2 skills, got %d", len(skills))
	}

	// Check agent bindings
	views, err := reg.GetAgentSkills("agent_sm")
	if err != nil {
		t.Fatalf("failed to get agent skills: %v", err)
	}
	if len(views) < 2 {
		t.Fatalf("expected at least 2 skill views, got %d", len(views))
	}

	// Verify toggle skill
	err = reg.ToggleAgentSkill("agent_sm", SkillVisionReceiptExtractorID, false)
	if err != nil {
		t.Fatalf("failed to toggle skill: %v", err)
	}

	updatedViews, err := reg.GetAgentSkills("agent_sm")
	if err != nil {
		t.Fatalf("failed to get updated agent skills: %v", err)
	}
	for _, v := range updatedViews {
		if v.SkillID == SkillVisionReceiptExtractorID && v.Enabled {
			t.Errorf("expected vision skill to be disabled for agent_sm")
		}
	}

	// Update multiple skills
	newViews, err := reg.UpdateAgentSkills("agent_sm", []AgentSkillInput{
		{SkillID: SkillVisionReceiptExtractorID, Enabled: true},
		{SkillID: SkillGoogleSheetsSyncID, Enabled: false},
	})
	if err != nil {
		t.Fatalf("failed to update agent skills: %v", err)
	}
	for _, v := range newViews {
		if v.SkillID == SkillVisionReceiptExtractorID && !v.Enabled {
			t.Errorf("expected vision skill to be enabled")
		}
		if v.SkillID == SkillGoogleSheetsSyncID && v.Enabled {
			t.Errorf("expected sheets skill to be disabled")
		}
	}
}

func TestRegistry_BuiltInConnectorsAndTools(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()

	tools, err := reg.DiscoverTools(ctx)
	if err != nil {
		t.Fatalf("failed to discover tools: %v", err)
	}
	if len(tools) < 5 {
		t.Errorf("expected at least 5 tools, got %d", len(tools))
	}

	// 1. Test Google Sheets disconnected error
	resp, err := reg.ExecuteTool(ctx, "google_sheets_append_row", map[string]any{
		"values": []any{"2026-10-30", "Indomaret", 173000},
	})
	if err == nil {
		t.Errorf("expected error when google sheets is disconnected")
	}
	if resp == nil || !resp.IsError {
		t.Errorf("expected resp.IsError = true")
	}

	// Connect Google Sheets and test again
	_, _ = reg.ConnectApp("google_sheets", "pemilik.warung@gmail.com")
	resp, err = reg.ExecuteTool(ctx, "google_sheets_append_row", map[string]any{
		"values": []any{"2026-10-30", "Indomaret", 173000},
	})
	if err != nil {
		t.Fatalf("expected success after connecting, got %v", err)
	}
	if resp.IsError || len(resp.Content) == 0 {
		t.Fatalf("expected non-error response with content: %+v", resp)
	}

	// 2. Test Vision Receipt Extractor
	receiptSample := `
INDOMARET SUDIRMAN
Tgl: 2026-10-30
1x MINYAK GORENG 2L  38.000
2x BERAS ROJOLELE 5KG 135.000
TOTAL: Rp 173.000
TUNAI: Rp 200.000
`
	visionResp, err := reg.ExecuteTool(ctx, "vision_extract_receipt", map[string]any{
		"receipt_text": receiptSample,
	})
	if err != nil {
		t.Fatalf("failed to execute vision extractor: %v", err)
	}
	if visionResp.IsError || len(visionResp.Content) == 0 {
		t.Fatalf("vision extractor returned error: %+v", visionResp)
	}

	var extraction ReceiptExtraction
	if err := json.Unmarshal([]byte(visionResp.Content[0].Text), &extraction); err != nil {
		t.Fatalf("failed to unmarshal extraction JSON: %v", err)
	}
	if extraction.Merchant != "Indomaret" {
		t.Errorf("expected merchant Indomaret, got %s", extraction.Merchant)
	}
	if extraction.TotalAmount != 173000 {
		t.Errorf("expected total amount 173000, got %v", extraction.TotalAmount)
	}
	if extraction.Category != "Belanja Operasional" {
		t.Errorf("expected category Belanja Operasional, got %s", extraction.Category)
	}
	if len(extraction.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(extraction.Items))
	}

	// 3. Test Co-pilot internal tools
	appsResp, err := reg.ExecuteTool(ctx, "list_apps", map[string]any{})
	if err != nil {
		t.Fatalf("list_apps failed: %v", err)
	}
	if !strings.Contains(appsResp.Content[0].Text, "google_sheets") {
		t.Errorf("expected list_apps output to contain google_sheets")
	}

	skillsResp, err := reg.ExecuteTool(ctx, "list_skills", map[string]any{})
	if err != nil {
		t.Fatalf("list_skills failed: %v", err)
	}
	if !strings.Contains(skillsResp.Content[0].Text, "vision_receipt_extractor") {
		t.Errorf("expected list_skills output to contain vision_receipt_extractor")
	}

	toggleResp, err := reg.ExecuteTool(ctx, "toggle_agent_skill", map[string]any{
		"agent_id": "agent_dev",
		"skill_id": "vision_receipt_extractor",
		"enabled":  false,
	})
	if err != nil {
		t.Fatalf("toggle_agent_skill failed: %v", err)
	}
	if !strings.Contains(toggleResp.Content[0].Text, "disabled") {
		t.Errorf("expected toggle output to confirm disabled: %s", toggleResp.Content[0].Text)
	}
}

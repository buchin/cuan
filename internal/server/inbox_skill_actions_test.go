package server

import (
	"context"
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
	if act.Action != "toggle" || act.AgentID != "agent_dev" || act.SkillID != "vision_receipt_extractor" || !act.Enabled {
		t.Fatalf("unexpected parsed action: %+v", act)
	}

	// 2. Invalid missing agent_id
	badAgent := "```cuan-skill\n{\"action\":\"toggle\",\"agent_id\":\"\",\"skill_id\":\"vision_receipt_extractor\",\"enabled\":true}\n```"
	if _, _, err := parseInboxSkill(badAgent); err == nil {
		t.Fatal("expected error for empty agent_id")
	}

	// 3. Invalid missing skill_id
	badSkill := "```cuan-skill\n{\"action\":\"toggle\",\"agent_id\":\"agent_dev\",\"skill_id\":\"\",\"enabled\":true}\n```"
	if _, _, err := parseInboxSkill(badSkill); err == nil {
		t.Fatal("expected error for empty skill_id")
	}
}

func TestApplyInboxSkill(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestChatServer(t)
	s.mcpRegistry = mcp.NewRegistry()

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
}

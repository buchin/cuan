package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"testing"
)

func TestHistoricalRuntimeRoutesUseEditablePublicAssistant(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	current := &db.Agent{ID: db.AgentIDCuan, Name: "@teman", Harness: "agy", Provider: "agy", Enabled: true, ConnectionID: "current-brains"}
	if err := s.agentRepo.CreateAgent(ctx, current); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"agent_sm", "agent_dev", "agent_support"} {
		_ = s.agentRepo.CreateAgent(ctx, &db.Agent{ID: id, Name: "@old", Harness: "agy", Provider: "agy", Enabled: true, ConnectionID: "quota-limited-old"})
		got, err := s.runtimeAgent(ctx, id)
		if err != nil || got.ID != db.AgentIDCuan || got.ConnectionID != current.ConnectionID || got.Name != "@teman" {
			t.Fatal("historical request pinned to old connection", got, err)
		}
	}
	custom, err := s.runtimeAgent(ctx, "agent_lifecycle")
	if err != nil || custom.ID != "agent_lifecycle" {
		t.Fatal("custom agent unexpectedly remapped", err)
	}
	current.Enabled = false
	current.Status = db.AgentStatusInactive
	_ = s.agentRepo.UpdateAgent(ctx, current)
	got, err := s.runtimeAgent(ctx, "agent_sm")
	if err != nil || got.Enabled {
		t.Fatal("disabled public assistant bypassed", err)
	}
}

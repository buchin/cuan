package server

import (
	"context"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestRecoveryAgentIdentityUsesRegisteredCanonicalAlias(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	for _, pair := range [][2]string{{"agent_sm", "agent_cuan"}, {"agent_dev", "agent_cuan"}, {"missing", "agent_lifecycle"}, {"", ""}} {
		if s.sameRuntimeAgent(ctx, pair[0], pair[1]) {
			t.Fatal("unregistered canonical identity accepted", pair)
		}
	}
	if err := s.agentRepo.CreateAgent(ctx, &db.Agent{ID: db.AgentIDCuan, Name: "@custom", Harness: "agy", Provider: "agy", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"agent_sm", "agent_dev", "agent_support", "@cuan"} {
		if !s.sameRuntimeAgent(ctx, alias, db.AgentIDCuan) {
			t.Fatal("legacy approval blocked after canonical migration", alias)
		}
	}
	if s.sameRuntimeAgent(ctx, "agent_lifecycle", db.AgentIDCuan) || s.sameRuntimeAgent(ctx, "missing", db.AgentIDCuan) {
		t.Fatal("unrelated agent became owner-approved agent")
	}
}

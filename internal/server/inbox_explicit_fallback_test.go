package server

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

func TestInboxExplicitFallbackPreservesPreferredModelAcrossAccounts(t *testing.T) {
	s, _, _, primary, backup, _, _ := setupFailoverFixture(t)
	ctx := context.Background()
	for _, p := range []*agent.ProviderAccount{primary, backup} {
		if _, err := agent.UpdateProviderAccountSettings(p.OwnerID, p.ID, p.Label, "claude-sonnet-test"); err != nil {
			t.Fatal(err)
		}
		if _, err := agent.SetProviderAccountFallbackModel(p.OwnerID, p.ID, "gemini-test"); err != nil {
			t.Fatal(err)
		}
	}
	oldFetch := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = oldFetch }()
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage {
		return agent.AccountModelUsage{Available: true, Models: []agent.ModelUsage{{Model: "claude-sonnet-test", Remaining: 1}, {Model: "gemini-test", Remaining: 100}, {Model: "claude-opus-test", Remaining: 99}}}
	}
	facade := &db.Agent{ID: db.AgentIDCuan, Name: "@cuan", Provider: "agy", Harness: "agy", Enabled: true, FallbackModelID: "claude-opus-test"}
	routes := s.providerRoutes(ctx, primary.OwnerID, facade, "", false)
	if len(routes) != 4 {
		t.Fatal("explicit routing leaked other models", routes)
	}
	for i, route := range routes {
		want := "claude-sonnet-test"
		if i >= 2 {
			want = "gemini-test"
		}
		if route.model != want {
			t.Fatal("quota ranking overrode selected primary priority", routes)
		}
	}
	for _, p := range []*agent.ProviderAccount{primary, backup} {
		if err := agent.RecordProviderModelQuotaWithDuration(p.OwnerID, p.ID, "claude-sonnet-test", time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	routes = s.providerRoutes(ctx, primary.OwnerID, facade, "", false)
	if len(routes) != 2 {
		t.Fatal("cooldown did not leave only permitted Gemini routes", routes)
	}
	for _, route := range routes {
		if route.model != "gemini-test" {
			t.Fatal("cooled preferred route admitted")
		}
	}
	chosen := routes[0]
	release, err := s.leaseProviderRoute(ctx, primary.OwnerID, facade, chosen)
	if err != nil {
		t.Fatal("allowed Gemini route unavailable", err)
	}
	release()
	if _, err := agent.SetProviderAccountModelFallback(primary.OwnerID, chosen.accountID, false); err != nil {
		t.Fatal(err)
	}
	if release, err := s.leaseProviderRoute(ctx, primary.OwnerID, facade, chosen); err == nil {
		release()
		t.Fatal("revoked fallback was still leased")
	}
}

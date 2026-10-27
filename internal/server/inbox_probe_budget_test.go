package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

func TestInboxSlowPrimaryDoesNotStarveAllowedBackupProbe(t *testing.T) {
	s, _, registered, primary, backup, home, _ := setupFailoverFixture(t)
	owner := primary.OwnerID
	if _, err := agent.SetProviderAccountFailover(owner, backup.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.SetProviderAccountModelFallback(owner, backup.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := agent.RecordProviderModelRestriction(owner, backup.ID, ""); err != nil {
		t.Fatal(err)
	}
	original := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = original }()
	var backupChecked atomic.Bool
	fetchInboxModelUsage = func(ctx context.Context, _, id string) agent.AccountModelUsage {
		if ctx.Err() != nil {
			return agent.AccountModelUsage{}
		}
		if id == primary.ID {
			<-ctx.Done()
			return agent.AccountModelUsage{}
		}
		backupChecked.Store(true)
		return agent.AccountModelUsage{Available: true, Models: []agent.ModelUsage{{Model: "claude-backup-fixture", Remaining: 25}}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	routes := s.providerRoutes(ctx, owner, registered, home, false)
	if !backupChecked.Load() {
		t.Fatal("slow primary consumed backup preflight window")
	}
	for _, route := range routes {
		if route.accountID == backup.ID && route.model == "claude-backup-fixture" {
			return
		}
	}
	t.Fatal("permitted backup evidence lost after slow primary", routes)
}

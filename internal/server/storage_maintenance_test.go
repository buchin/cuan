package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/scheduler"
)

func TestStorageMaintenanceUsesExistingSchedulerAndCompletedEventBus(t *testing.T) {
	s := NewServer(&config.Config{Environment: "testing"}, nil)
	defer s.terminalManager.CloseAll()
	bus := runner.NewEventBus()
	s.runnerSvc.SetEventBus(bus)
	calls := 0
	s.registerStorageMaintenanceHandler(true, func(context.Context) error { calls++; return nil })
	before, _ := s.scheduler.GetJob(storageMaintenanceJob)
	bus.PublishCompleted(context.Background(), runner.AttemptCompletedEvent{Status: "failed"})
	deadline := time.Now().Add(time.Second)
	for {
		job, _ := s.scheduler.GetJob(storageMaintenanceJob)
		if job.NextRunAt.Before(*before.NextRunAt) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("completion event did not request maintenance")
		}
		time.Sleep(time.Millisecond)
	}
	scheduled, _ := s.scheduler.GetJob(storageMaintenanceJob)
	for i := 0; i < 100; i++ {
		s.requestStorageMaintenance()
	}
	after, _ := s.scheduler.GetJob(storageMaintenanceJob)
	if !after.NextRunAt.Equal(*scheduled.NextRunAt) || calls != 0 {
		t.Fatal("events started parallel cleanup or postponed request")
	}
	if err := s.scheduler.Trigger(context.Background(), storageMaintenanceJob); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("existing scheduler did not execute worker")
	}
	if after.Interval != 15*time.Minute || after.ScheduleType != scheduler.ScheduleInterval {
		t.Fatal("periodic recovery missing")
	}
}

func TestPruneIdleStorageArtifacts_PreservesRecentAndKeepNewest(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	oldTime := now.Add(-72 * time.Hour)
	recentTime := now.Add(-2 * time.Hour)

	// Create 4 old dirs and 1 recent dir; keepNewest=2 keeps [recent, newest-old] -> 3 old dirs removed.
	for _, name := range []string{"old-1", "old-2", "old-3", "old-4"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}
	recentDir := filepath.Join(root, "recent")
	if err := os.MkdirAll(recentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recentDir, recentTime, recentTime); err != nil {
		t.Fatal(err)
	}

	removed := pruneOldArtifactDirs(root, 24*time.Hour, 2, now)
	if removed != 3 {
		t.Fatalf("expected 3 old dirs removed, got %d", removed)
	}
	if _, err := os.Stat(recentDir); err != nil {
		t.Fatalf("expected recent dir preserved, got %v", err)
	}

	// Test provider account session pruning (conversations + brain).
	accountsRoot := t.TempDir()
	convOld := filepath.Join(accountsRoot, "acct_123", "home", ".gemini", "antigravity-cli", "conversations", "old-conv.pb")
	convKeep := filepath.Join(accountsRoot, "acct_123", "home", ".gemini", "antigravity-cli", "conversations", "new-conv.pb")
	brainOld := filepath.Join(accountsRoot, "acct_123", "home", ".gemini", "antigravity-cli", "brain", "old-brain")
	if err := os.MkdirAll(filepath.Dir(convOld), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(convOld, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(convKeep, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(brainOld, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(convOld, oldTime, oldTime)
	_ = os.Chtimes(convKeep, recentTime, recentTime)
	_ = os.Chtimes(brainOld, oldTime, oldTime)

	acctRemoved := pruneProviderAccountArtifacts(accountsRoot, 48*time.Hour, 0, now)
	if acctRemoved != 2 {
		t.Fatalf("expected 2 old account artifacts removed, got %d", acctRemoved)
	}
	if _, err := os.Stat(convKeep); err != nil {
		t.Fatalf("expected recent conversation kept, got %v", err)
	}
}

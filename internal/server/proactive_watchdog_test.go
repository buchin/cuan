package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestProactiveWatchdogCycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)

	cfg := &config.Config{
		Environment:   "testing",
		WorkspacesDir: t.TempDir(),
	}

	srv := &Server{
		cfg:     cfg,
		taskSvc: taskSvc,
	}

	// Verify cycle runs without panic or deadlocks
	srv.RunProactiveWatchdogCycle(ctx)

	// Verify StartProactiveWatchdog starts and stops cleanly with context
	srv.StartProactiveWatchdog(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
}

func TestProactiveWatchdogStorageAlertProposal(t *testing.T) {
	ctx := context.Background()
	repo := db.NewMemoryProjectStudioRepository()

	// Reset alert timestamp
	watchdogAlertMu.Lock()
	lastStorageAlertTime = time.Time{}
	watchdogAlertMu.Unlock()

	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_test",
		Title:     "General Ops",
	}
	if err := repo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		projectStudioRepo: repo,
	}

	// 1. Below threshold (85%): should not generate alert proposal
	srv.checkProactiveStorageAlert(ctx, ServerTelemetry{DiskUsedPercent: 86.0, DiskFreeBytes: 2 * 1024 * 1024 * 1024})
	messages, err := repo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages below 88%% threshold, got %d", len(messages))
	}

	// 2. Above threshold (92%): should generate proposal card
	srv.checkProactiveStorageAlert(ctx, ServerTelemetry{DiskUsedPercent: 92.0, DiskFreeBytes: 1024 * 1024 * 1024})
	messages, err = repo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 proposal message generated, got %d", len(messages))
	}
	if messages[0].AuthorName != "Cuan Watchdog" {
		t.Errorf("expected author Cuan Watchdog, got %s", messages[0].AuthorName)
	}
	if !strings.Contains(messages[0].Content, "```cuan-approval") {
		t.Errorf("expected cuan-approval proposal card in content, got: %s", messages[0].Content)
	}
	if !strings.Contains(messages[0].Content, "Optimasi & Pembersihan Ruang Penyimpanan") {
		t.Errorf("expected proposal title in content, got: %s", messages[0].Content)
	}

	// 3. Deduplication: second run immediately should NOT create duplicate message
	srv.checkProactiveStorageAlert(ctx, ServerTelemetry{DiskUsedPercent: 94.0, DiskFreeBytes: 512 * 1024 * 1024})
	messages, err = repo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected still 1 message due to deduplication, got %d", len(messages))
	}
}

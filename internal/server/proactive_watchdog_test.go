package server

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
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

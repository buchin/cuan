package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/runner"
)

func TestRunnerSupervisor_Disabled(t *testing.T) {
	cfg := &config.Config{
		RunnerAutoSupervise: false,
	}
	srv := &Server{cfg: cfg}
	rs := NewRunnerSupervisor(srv)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should return immediately without blocking or spawning
	rs.Start(ctx)
	if rs.isRunning {
		t.Fatal("supervisor should not be running when disabled")
	}
}

func TestRunnerSupervisor_IsExternalRunnerAlive(t *testing.T) {
	srv, _ := setupTestChatServer(t)
	rs := NewRunnerSupervisor(srv)

	ctx := context.Background()

	// Initially, no runners exist in memory store
	if rs.isExternalRunnerAlive(ctx) {
		t.Fatal("expected isExternalRunnerAlive to be false when no runners registered")
	}

	// Register a fresh runner with heartbeat now
	_ = srv.runnerSvc.Store().RegisterOrUpdateHeartbeat(ctx, &runner.RunnerInfo{
		ID:              "ext-runner-1",
		Status:          runner.StatusIdle,
		LastHeartbeatAt: time.Now().UTC(),
	})

	if !rs.isExternalRunnerAlive(ctx) {
		t.Fatal("expected isExternalRunnerAlive to be true after fresh heartbeat")
	}

	// Register a stale runner (> 30s ago)
	_ = srv.runnerSvc.Store().RegisterOrUpdateHeartbeat(ctx, &runner.RunnerInfo{
		ID:              "ext-runner-1",
		Status:          runner.StatusIdle,
		LastHeartbeatAt: time.Now().UTC().Add(-45 * time.Second),
	})

	if rs.isExternalRunnerAlive(ctx) {
		t.Fatal("expected isExternalRunnerAlive to be false for stale heartbeat")
	}
}

func TestRunnerSupervisor_ResolveRunnerBinary(t *testing.T) {
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "cuan-runner")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to create fake runner: %v", err)
	}

	cfg := &config.Config{
		RunnerBinPath: fakeBin,
	}
	srv := &Server{cfg: cfg}
	rs := NewRunnerSupervisor(srv)

	resolved := rs.resolveRunnerBinary()
	if resolved != fakeBin {
		t.Fatalf("expected resolved binary to be %s, got %s", fakeBin, resolved)
	}
}

func TestServer_ReconcileStaleRunners_NilDB(t *testing.T) {
	srv := &Server{}
	if err := srv.ReconcileStaleRunners(context.Background()); err != nil {
		t.Fatalf("expected nil error on nil db, got: %v", err)
	}
}

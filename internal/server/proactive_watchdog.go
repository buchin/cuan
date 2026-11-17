package server

import (
	"context"
	"log"
	"time"
)

// StartProactiveWatchdog starts a periodic background watchdog daemon to proactively
// monitor disk pressure, background task queue health, and perform autonomous self-healing.
func (s *Server) StartProactiveWatchdog(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RunProactiveWatchdogCycle(ctx)
			}
		}
	}()
}

// RunProactiveWatchdogCycle performs one inspection and self-healing sweep:
// 1. Proactive Disk Health: check free space; request automatic maintenance if > 85% full.
// 2. Autonomous Task Self-Healing: sweep blocked inbox tasks and recover unstarted or missing-workspace launches.
// 3. Queue Saturation Check: kick pending task launches to ensure no tasks linger in queue.
func (s *Server) RunProactiveWatchdogCycle(ctx context.Context) {
	if s.inboxStopping.Load() {
		return
	}

	// 1. Disk Health & Proactive Prune
	storagePath := "/"
	if s.cfg != nil && s.cfg.WorkspacesDir != "" {
		storagePath = s.cfg.WorkspacesDir
	}
	telemetry := s.getServerTelemetry(ctx, storagePath)
	if telemetry.DiskWarning || telemetry.DiskUsedPercent > 85.0 {
		log.Printf("[Watchdog] Proactive maintenance triggered: disk space at %.1f%% used (%s free)",
			telemetry.DiskUsedPercent, formatBytes(telemetry.DiskFreeBytes))
		s.requestStorageMaintenance()
	}

	// 2. Autonomous Self-Healing for Stuck/Blocked Inbox Tasks
	if s.taskSvc != nil && s.projectStudioRepo != nil && s.executesInbox() {
		s.deliverInboxTaskUpdates(ctx)
	}

	// 3. Ensure queue launches are drained and orphan launches reconciled
	if s.taskSvc != nil {
		_ = s.taskSvc.ReconcileOrphanLaunches(ctx, time.Now().UTC())
	}
	s.kickTaskLaunches()
}

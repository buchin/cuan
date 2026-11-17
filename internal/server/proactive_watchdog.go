package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
)

var (
	watchdogAlertMu      sync.Mutex
	lastStorageAlertTime time.Time
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
		s.checkProactiveStorageAlert(ctx, telemetry)
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

func (s *Server) checkProactiveStorageAlert(ctx context.Context, telemetry ServerTelemetry) {
	if telemetry.DiskUsedPercent <= 88.0 || s.projectStudioRepo == nil {
		return
	}
	watchdogAlertMu.Lock()
	defer watchdogAlertMu.Unlock()
	if !lastStorageAlertTime.IsZero() && time.Since(lastStorageAlertTime) < 6*time.Hour {
		return
	}

	rooms, err := s.projectStudioRepo.ListAllRooms(ctx)
	if err != nil || len(rooms) == 0 {
		return
	}

	targetRoomID := rooms[0].ID
	projID := rooms[0].ProjectID
	if projID == "" {
		projID = "default"
	}

	alertMsg := fmt.Sprintf("Halo, kapasitas penyimpanan server mulai menipis (penggunaan ruang disk mencapai %.1f%%, tersisa %s). Agar sistem tetap berjalan optimal dan terhindar dari kendala layanan, disarankan untuk menjalankan pembersihan berkala terhadap file cache dan arsip sementara.\n\n```cuan-approval\n{\"title\":\"Optimasi & Pembersihan Ruang Penyimpanan\",\"description\":\"Menjalankan pembersihan otomatis terhadap cache build compiler, artefak sementara, dan file log usang untuk memulihkan kapasitas disk server.\",\"project_id\":\"%s\",\"required_stages\":[\"implement\"]}\n```",
		telemetry.DiskUsedPercent, formatBytes(telemetry.DiskFreeBytes), projID)

	msg := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     targetRoomID,
		AuthorType: "agent",
		AuthorID:   "agent_cuan",
		AuthorName: "Cuan Watchdog",
		Content:    alertMsg,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, msg); err == nil {
		lastStorageAlertTime = time.Now().UTC()
		log.Printf("[Watchdog] Proactive proposal card posted to room %s for disk pressure %.1f%%", targetRoomID, telemetry.DiskUsedPercent)
	}
}

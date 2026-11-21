package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

var (
	watchdogAlertMu      sync.Mutex
	lastStorageAlertTime time.Time
)

// StartProactiveWatchdog starts a periodic background watchdog daemon to proactively
// monitor disk pressure, background task queue health, and perform autonomous self-healing.
func (s *Server) StartProactiveWatchdog(ctx context.Context) {
	go func() {
		// Run first inspection cycle immediately upon startup
		s.RunProactiveWatchdogCycle(ctx)

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
		s.orchestrateBlockedInboxTasks(ctx)
		s.autoResolveCompletedConversations(ctx)
	}

	// 3. Ensure queue launches are drained and orphan launches reconciled
	if s.taskSvc != nil {
		_ = s.taskSvc.ReconcileOrphanLaunches(ctx, time.Now().UTC())
	}
	s.kickTaskLaunches()

	// 4. Background Watchdog Git Sync saat Idle:
	// Memeriksa origin/dev secara berkala saat tidak ada runner yang aktif (lock bebas)
	// dan dev lokal bersih, lalu menyelaraskan commit secara hening.
	s.syncIdleProjectsGit(ctx)
}

// syncIdleProjectsGit checks origin/dev (or default branch) periodically when no
// runner is active (lock free) and working tree is clean, silently fast-forwarding local commits.
func (s *Server) syncIdleProjectsGit(ctx context.Context) {
	if s.projectSvc == nil {
		return
	}

	// 1. Invariant check: only sync when no runner is actively executing
	if s.taskSvc != nil {
		doingTasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: task.TaskStatusDoing})
		if err == nil && len(doingTasks) > 0 {
			return
		}
	}

	projects, err := s.projectSvc.List(ctx)
	if err != nil || len(projects) == 0 {
		return
	}

	for _, p := range projects {
		if p == nil || p.RepoURL == "" || p.LocalPath == "" || p.IsArchived {
			continue
		}

		// 2. Safe check: directory must exist and contain git metadata
		gitDir := filepath.Join(p.LocalPath, ".git")
		if fi, err := os.Stat(gitDir); err != nil || (!fi.IsDir() && !fi.Mode().IsRegular()) {
			continue
		}

		// 3. Acquire verification lease non-blockingly (lock bebas)
		lease, err := git.TryAcquireVerificationLease(ctx, p.LocalPath)
		if err != nil {
			// Lock is busy (held by runner/verifier/task) or unavailable; skip silently
			continue
		}

		// 4. Preflight sync silently (safe fallback non-blocking on network / dirty working tree)
		_ = s.projectSvc.PreflightSyncProject(ctx, p)
		_ = lease.Close()
	}
}

// orchestrateBlockedInboxTasks inspects tasks in status 'blocked' and performs:
// 1. Autonomous Self-Healing: If within attempt budget (< 5 attempts) and actionable revision
//    is available with manager continuation allowed, queues a correction attempt immediately.
// 2. 1-Click Action Card (cuan-approval): If human input/decision is needed (or budget threshold reached),
//    posts a clean, deduplicated continuation proposal card in the conversation room so the user
//    can unblock work with a single tap (Zero Cognitive Fatigue).
func (s *Server) orchestrateBlockedInboxTasks(ctx context.Context) {
	if s.taskSvc == nil || s.projectStudioRepo == nil || !s.executesInbox() {
		return
	}

	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: task.TaskStatusBlocked})
	if err != nil || len(tasks) == 0 {
		return
	}

	repo, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return
	}

	for _, t := range tasks {
		if t.SourceRoomID == "" || !strings.HasPrefix(t.Instructions, "Cuan managed delegation v1.\n") {
			continue
		}

		// Don't interfere if an attempt is currently running or queued
		detail, err := s.taskSvc.GetTaskDetail(ctx, t.ID)
		if err != nil || detail == nil || detail.LatestAttempt == nil {
			continue
		}
		if detail.LatestAttempt.Status == task.AttemptStatusRunning || detail.LatestAttempt.Status == task.AttemptStatusQueued {
			continue
		}
		if detail.LatestAttempt.CancelRequested || detail.LatestAttempt.CanceledAt != nil {
			continue
		}

		// Check if there is an active launch in progress
		if launch, e := s.taskSvc.LatestLaunch(ctx, t.ID); e == nil && launch != nil {
			if launch.State == "queued" || launch.State == "claimed" || launch.State == "started" {
				continue
			}
		}

		// Find the latest review assessment on this task
		var latestAssessment *inboxReviewEvaluation
		var assessmentRequestID string
		for i := len(detail.Activities) - 1; i >= 0; i-- {
			act := detail.Activities[i]
			if act == nil || act.ActivityType != reviewAssessmentActivity {
				continue
			}
			var record reviewAssessmentRecord
			if err := json.Unmarshal([]byte(act.Message), &record); err == nil && record.Assessment.Decision != "" {
				latestAssessment = &record.Assessment
				assessmentRequestID = record.RequestID
				break
			}
		}

		requests, err := repo.ListInboxRequests(ctx, detail.SourceRoomID)
		if err != nil {
			continue
		}

		// Resolve the task's root owner authorization request.
		// System review turns create evaluation context, but never fresh consent.
		// All continuation paths require the original user authorization lineage.
		var rootRequest *db.InboxRequest
		for i := range requests {
			if requests[i].MessageID == detail.SourceMessageID && requests[i].State != "canceled" && requests[i].State != "cancelled" && requests[i].State != "cancelling" {
				rootRequest = &requests[i]
				break
			}
		}
		if rootRequest == nil {
			for i := range requests {
				if requests[i].MessageID == detail.SourceMessageID {
					rootRequest = &requests[i]
					break
				}
			}
		}
		if rootRequest == nil && len(requests) > 0 {
			for i := range requests {
				if requests[i].State != "canceled" && requests[i].State != "cancelled" && requests[i].State != "cancelling" {
					rootRequest = &requests[i]
					break
				}
			}
			if rootRequest == nil {
				rootRequest = &requests[0]
			}
		}
		if rootRequest == nil {
			continue
		}

		origin := *rootRequest
		if origin.AgentID == "" {
			if detail.AssignedAgent != "" {
				origin.AgentID = detail.AssignedAgent
			} else {
				origin.AgentID = "agent_cuan"
			}
		}
		if origin.OwnerID == "" {
			for i := range requests {
				if requests[i].OwnerID != "" {
					origin.OwnerID = requests[i].OwnerID
					break
				}
			}
		}

		attemptCount := taskRevisionAttemptCount(detail)

		// 1. Autonomous Self-Healing:
		// When within budget (< 5 attempts), manager indicates continue/replan, and actionable revision is available
		if latestAssessment != nil && attemptCount < 5 &&
			(latestAssessment.ManagerAction == "continue" || latestAssessment.ManagerAction == "replan") &&
			latestAssessment.Decision == "revise" &&
			latestAssessment.RequestedOutcome != "needs_input" &&
			strings.TrimSpace(latestAssessment.Revision) != "" {

			sourceRequestID := origin.RequestID
			launchID := delegationMessageID(fmt.Sprintf("watchdog-continuation:%s:%s:%d", detail.ID, sourceRequestID, len(detail.Attempts)+1))
			notice := s.startAuthorizedLaunch(ctx, origin, detail, launchID, sourceRequestID, latestAssessment.Revision)
			s.kickTaskLaunches()
			log.Printf("[Watchdog AI Orchestrator] Autonomously launched continuation for blocked task %s (attempt %d/5, notice=%s): %s",
				detail.ID, attemptCount+1, notice, latestAssessment.Revision)
			continue
		}

		// 2. 1-Click Action Card (Zero Cognitive Fatigue):
		// If autonomous continuation was not possible (needs owner decision, permission, or budget threshold reached),
		// present a 1-click continuation approval card in the conversation room so user can unblock in 1 tap.
		if latestAssessment != nil {
			s.postContinuationActionCard(ctx, detail, assessmentRequestID, *latestAssessment)
		}
	}
}

func (s *Server) postContinuationActionCard(ctx context.Context, detail *task.TaskDetailResponse, requestID string, assessment inboxReviewEvaluation) {
	// Deduplicate: check if an action card for this attempt has already been posted to the room
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, detail.SourceRoomID)
	if err != nil {
		return
	}

	if detail.LatestAttempt != nil {
		for _, m := range messages {
			if strings.Contains(m.Content, "cuan-approval") && strings.Contains(m.Content, detail.LatestAttempt.ID) {
				// Already posted for this attempt
				return
			}
		}
	}

	var planBlock string
	if assessment.Decision == "revise" && strings.TrimSpace(assessment.Revision) != "" {
		planBlock = continuationPlan(detail, requestID, assessment)
	}
	if planBlock == "" {
		planTitle := "Lanjutkan: " + detail.Title
		planDesc := strings.TrimSpace(assessment.Explanation)
		if planDesc == "" {
			planDesc = strings.TrimSpace(assessment.Finding)
		}
		if planDesc == "" {
			planDesc = "Pekerjaan memerlukan keputusan Anda untuk melanjutkan."
		}
		if len(planDesc) > 500 {
			planDesc = planDesc[:497] + "..."
		}
		attemptID := ""
		if detail.LatestAttempt != nil {
			attemptID = detail.LatestAttempt.ID
		}
		continuationRevision := strings.TrimSpace(assessment.Revision)
		if continuationRevision == "" {
			continuationRevision = fmt.Sprintf("Lanjutkan perbaikan %s sesuai persetujuan pemilik, terapkan perubahan pada repositori dan jalankan verifikasi.", detail.Title)
		}
		continuationCategory := detail.ExecutionCategory
		if continuationCategory == "" {
			continuationCategory = task.ExecutionCategoryCode
		}
		plan := inboxApprovalPlan{
			Title:             planTitle,
			Description:       planDesc,
			ProjectID:         detail.ProjectID,
			ExecutionCategory: continuationCategory,
			RequiredStages:    []string{"implement"},
			Continuation: &inboxContinuationPlan{
				TaskID:              detail.ID,
				AttemptID:           attemptID,
				AssessmentRequestID: requestID,
				Revision:            continuationRevision,
				RemainingWork:       assessment.RemainingWork,
			},
		}
		raw, _ := json.Marshal(plan)
		planBlock = "\n\n```cuan-approval\n" + string(raw) + "\n```"
	}

	finding := strings.TrimSpace(assessment.Finding)
	if finding == "" {
		finding = strings.TrimSpace(assessment.Explanation)
	}
	if finding == "" {
		finding = "Pekerjaan memerlukan keputusan Anda untuk melanjutkan."
	}

	content := fmt.Sprintf("%s\n\nUntuk melanjutkan perbaikan, silakan konfirmasi melalui tombol di bawah:%s\n\n[View work](/tasks/%s)", finding, planBlock, detail.ID)

	msg := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     detail.SourceRoomID,
		AuthorType: "agent",
		AuthorID:   detail.AssignedAgent,
		AuthorName: "Cuan",
		Content:    content,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if msg.AuthorID == "" {
		msg.AuthorID = "agent_cuan"
	}

	if err := s.projectStudioRepo.CreateMessage(ctx, msg); err == nil {
		log.Printf("[Watchdog AI Orchestrator] Posted 1-click continuation action card for task %s to room %s",
			detail.ID, detail.SourceRoomID)
		s.publishInbox(detail.SourceRoomID)
	}
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

// autoResolveCompletedConversations periodically scans conversations whose work has finished (all tasks completed/canceled)
// and autonomously archives them after a tranquil 24-hour grace period of inactivity without human intervention.
func (s *Server) autoResolveCompletedConversations(ctx context.Context) {
	if s.taskSvc == nil || s.projectStudioRepo == nil || !s.executesInbox() {
		return
	}

	rooms, err := s.projectStudioRepo.ListAllRooms(ctx)
	if err != nil || len(rooms) == 0 {
		return
	}

	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	for _, room := range rooms {
		msgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
		if err != nil || len(msgs) == 0 {
			continue
		}

		latestMsg := msgs[len(msgs)-1]
		if room.IsResolved(latestMsg.CreatedAt) {
			continue
		}

		// Check if there are tasks for this room
		tasks, err := s.taskSvc.Store().ListTasks(ctx, task.TaskFilter{SourceRoomID: room.ID})
		if err != nil {
			continue
		}
		if len(tasks) == 0 {
			legacy, err := s.projectStudioRepo.ListTasksByRoomID(ctx, room.ID)
			if err != nil || len(legacy) == 0 {
				continue
			}
		}

		// Ensure there are no active, running, queued, or blocked tasks/launches/requests
		blocker, err := s.conversationResolutionBlocker(ctx, room.ID)
		if err != nil || blocker != "" {
			continue
		}

		// Enforce read-aware grace period: if the latest message was sent by an agent/system,
		// the 24-hour countdown must only begin AFTER the operator has actually read it.
		// If unread, it must remain in the Inbox so the operator never misses the completed result.
		if latestMsg.AuthorType == "agent" || latestMsg.AuthorType == "system" {
			var readAt *time.Time

			// 1. Check message receipts for confirmed read
			if receiptRepo, ok := s.projectStudioRepo.(db.ChatReceiptRepository); ok {
				if receipts, err := receiptRepo.MessageReceipts(ctx, room.ID); err == nil {
					if r, exists := receipts[latestMsg.ID]; exists && r.ReadConfirmedAt != nil {
						copied := *r.ReadConfirmedAt
						readAt = &copied
					}
				}
			}

			// 2. Check operator conversation item preferences
			preferences := itemPreferences{}
			if s.authSvc != nil {
				if ownerID, err := s.authSvc.OwnerID(ctx); err == nil && ownerID != "" {
					itemPreferencesMu.Lock()
					saved, _ := readItemPreferences(ownerID)
					itemPreferencesMu.Unlock()
					preferences = saved
				}
			}
			state := preferences["conversation:"+room.ID]
			if state.Unread {
				// Operator explicitly marked conversation as unread; do not auto-resolve
				continue
			}
			if state.ReadAt != "" {
				if prefCutoff, err := time.Parse(time.RFC3339Nano, state.ReadAt); err == nil {
					if !latestMsg.CreatedAt.After(prefCutoff) {
						if readAt == nil || prefCutoff.After(*readAt) {
							readAt = &prefCutoff
						}
					}
				}
			}

			// If the completion message has never been read, keep it in Inbox!
			if readAt == nil {
				continue
			}

			// 24-hour grace period must elapse AFTER the message was read
			if readAt.After(cutoff) {
				continue
			}
		} else {
			// If latest message was sent by the user, verify it is older than 24 hours
			if latestMsg.CreatedAt.After(cutoff) {
				continue
			}
		}

		// Also check all tasks: their UpdatedAt must be older than cutoff
		allTasksOld := true
		for _, t := range tasks {
			if t.UpdatedAt.After(cutoff) {
				allTasksOld = false
				break
			}
		}
		if !allTasksOld {
			continue
		}

		now := time.Now().UTC()
		if err := s.projectStudioRepo.SetRoomResolved(ctx, room.ID, &now); err != nil {
			log.Printf("[Watchdog] Auto-resolve conversation %s failed: %v", room.ID, err)
			continue
		}

		s.publishInbox(room.ID)
		s.requestStorageMaintenance()
		log.Printf("[Watchdog] Proactively auto-resolved completed conversation %s after 24h grace period", room.ID)
	}
}

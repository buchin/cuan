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
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

var (
	watchdogAlertMu      sync.Mutex
	lastStorageAlertTime time.Time
	lastIdleGitSync      time.Time
	watchdogTaskCooldown = make(map[string]time.Time)
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

	// 2. Autonomous Self-Healing for Stuck/Blocked/Review Inbox Tasks & Queued Requests Sweeper (Heartbeat)
	if s.taskSvc != nil && s.projectStudioRepo != nil && s.executesInbox() {
		s.deliverInboxTaskUpdates(ctx)
		s.heartbeatSweepQueuedInboxRequests(ctx)
		s.reconcileVerifiedPublicationTasks(ctx)
		s.orchestrateBlockedInboxTasks(ctx)
		s.autoResolveCompletedConversations(ctx)
	}

	// 3. Ensure queue launches are drained and orphan launches reconciled
	if s.taskSvc != nil {
		_ = s.taskSvc.ReconcileOrphanLaunches(ctx, time.Now().UTC())
	}
	s.evaluateIdleRunnerQueues(ctx)
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

	watchdogAlertMu.Lock()
	if time.Since(lastIdleGitSync) < 3*time.Minute {
		watchdogAlertMu.Unlock()
		return
	}
	lastIdleGitSync = time.Now().UTC()
	watchdogAlertMu.Unlock()

	// 1. Invariant check: only sync when no runner is actively executing
	if s.taskSvc != nil {
		doingTasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: task.TaskStatusDoing})
		if err == nil && len(doingTasks) > 0 {
			return
		}
	}

	projects, err := s.projectSvc.Store().ListProjectsFiltered(ctx, project.ArchivedFilterActive)
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
		_ = s.projectSvc.PreflightSyncStoredProject(ctx, p)
		_ = lease.Close()
	}
}

// heartbeatSweepQueuedInboxRequests ensures rooms with queued inbox requests
// are promptly woken up and dispatched (PaperclipAI Heartbeat pattern).
func (s *Server) heartbeatSweepQueuedInboxRequests(ctx context.Context) {
	if !s.executesInbox() || s.projectStudioRepo == nil {
		return
	}
	if dispatch, ok := s.projectStudioRepo.(db.InboxDispatchRepository); ok {
		rooms, err := dispatch.ListInboxDispatchRooms(ctx, time.Now().UTC())
		if err == nil {
			for _, roomID := range rooms {
				if ctx.Err() != nil {
					return
				}
				s.kickInboxRoom(roomID)
			}
		}
	}
}

// orchestrateBlockedInboxTasks inspects tasks in status 'blocked' and 'review', and performs:
// 1. Autonomous Self-Healing: If within attempt budget (< 5 attempts) and actionable revision
//    is available with manager continuation allowed, queues a correction attempt immediately.
// 2. Autonomous Publication Reconciliation: If task changes are verified on an auto_deploy project,
//    triggers auto-apply and deploy autonomously without freezing idle.
// 3. 1-Click Action Card (cuan-approval): If human input/decision is needed (or budget threshold reached),
//    posts a clean, deduplicated continuation proposal card in the conversation room so the user
//    can unblock work with a single tap (Zero Cognitive Fatigue).
func (s *Server) orchestrateBlockedInboxTasks(ctx context.Context) {
	if s.taskSvc == nil || s.projectStudioRepo == nil || !s.executesInbox() {
		return
	}

	blockedTasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: task.TaskStatusBlocked})
	reviewTasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: task.TaskStatusReview})
	var tasks []*task.TaskItemResponse
	tasks = append(tasks, blockedTasks...)
	tasks = append(tasks, reviewTasks...)
	if len(tasks) == 0 {
		return
	}

	repo, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return
	}

	candidateIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		if t.SourceRoomID != "" && strings.HasPrefix(t.Instructions, "Cuan managed delegation v1.\n") {
			candidateIDs = append(candidateIDs, t.ID)
		}
	}
	if len(candidateIDs) == 0 {
		return
	}
	latestMap := s.taskSvc.LatestAttemptsMapForIDs(ctx, candidateIDs)

	requestsByRoom := make(map[string][]db.InboxRequest)
	for _, t := range tasks {
		if t.SourceRoomID == "" || !strings.HasPrefix(t.Instructions, "Cuan managed delegation v1.\n") {
			continue
		}

		watchdogAlertMu.Lock()
		cooldown, inCooldown := watchdogTaskCooldown[t.ID]
		watchdogAlertMu.Unlock()
		if inCooldown && time.Now().Before(cooldown) {
			continue
		}

		// Don't interfere if an attempt is currently running or queued
		latest := latestMap[t.ID]
		if latest == nil {
			continue
		}
		if latest.Status == task.AttemptStatusRunning || latest.Status == task.AttemptStatusQueued {
			continue
		}
		if latest.CancelRequested || latest.CanceledAt != nil {
			continue
		}

		// Check if there is an active launch in progress
		if launch, e := s.taskSvc.LatestLaunch(ctx, t.ID); e == nil && launch != nil {
			if launch.State == "queued" || launch.State == "claimed" || launch.State == "started" {
				continue
			}
		}

		detail, err := s.taskSvc.GetTaskDetail(ctx, t.ID)
		if err != nil || detail == nil {
			continue
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

		requests, ok := requestsByRoom[detail.SourceRoomID]
		if !ok {
			var err error
			requests, err = repo.ListInboxRequests(ctx, detail.SourceRoomID)
			if err != nil {
				continue
			}
			requestsByRoom[detail.SourceRoomID] = requests
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

		// 1. Autonomous publication reconciliation:
		// When code is verified but uncontained in running release on an auto_deploy project,
		// trigger auto-deploy autonomously.
		if latestAssessment != nil &&
			(latestAssessment.RemainingWork == "publication" || latestAssessment.RemainingWork == "none" || strings.HasPrefix(latestAssessment.Revision, "Native publication reconciliation:")) &&
			(latestAssessment.SourceReview == "pass" || latestAssessment.Decision == "pass" || strings.HasPrefix(latestAssessment.Revision, "Native publication reconciliation:")) &&
			s.projectSvc != nil && detail.ProjectID != "" {
			if proj, err := s.projectSvc.Get(ctx, detail.ProjectID); err == nil && proj != nil {
				if project.SettingsFromMetadata(proj.Metadata).IsAutoDeployEnabled() {
					log.Printf("[Watchdog AI Orchestrator] Autonomously triggering auto-deploy for verified publication task %s", detail.ID)
					s.autoApplyAndDeployTask(ctx, detail.ID, detail.ProjectID)
					continue
				}
			}
		}

		// 2. Autonomous Self-Healing:
		// When within budget (< 3 attempts), manager indicates continue/replan, actionable revision is available,
		// and work is internal (not external).
		if latestAssessment != nil && attemptCount < 3 &&
			latestAssessment.RemainingWork != "external" &&
			!strings.HasPrefix(latestAssessment.Revision, "Native publication reconciliation:") &&
			(latestAssessment.ManagerAction == "continue" || latestAssessment.ManagerAction == "replan") &&
			latestAssessment.Decision == "revise" &&
			latestAssessment.RequestedOutcome != "needs_input" &&
			strings.TrimSpace(latestAssessment.Revision) != "" {

			sourceRequestID := origin.RequestID
			baseKey := fmt.Sprintf("watchdog-continuation:%s:%s:%d", detail.ID, sourceRequestID, len(detail.Attempts)+1)
			launchID := delegationMessageID(baseKey)
			for retrySeq := 1; retrySeq <= 10; retrySeq++ {
				if existing, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(detail.ID, launchID)); err == nil && existing != nil {
					if existing.State == "stopped" || existing.State == "cancelled" {
						launchID = delegationMessageID(fmt.Sprintf("%s:retry:%d", baseKey, retrySeq))
						continue
					}
				}
				break
			}
			notice := s.startAuthorizedLaunch(ctx, origin, detail, launchID, sourceRequestID, latestAssessment.Revision)
			s.kickTaskLaunches()
			log.Printf("[Watchdog AI Orchestrator] Autonomously launched continuation for blocked task %s (attempt %d/3, notice=%s): %s",
				detail.ID, attemptCount+1, notice, latestAssessment.Revision)

			if launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(detail.ID, launchID)); err == nil && launch != nil && (launch.State == "queued" || launch.State == "claimed" || launch.State == "started") {
				watchdogAlertMu.Lock()
				delete(watchdogTaskCooldown, detail.ID)
				watchdogAlertMu.Unlock()
			} else {
				watchdogAlertMu.Lock()
				watchdogTaskCooldown[detail.ID] = time.Now().Add(5 * time.Minute)
				watchdogAlertMu.Unlock()
			}
			continue
		}

		// Autonomous native release handoff recovery:
		if latestAssessment != nil && latestAssessment.SourceReview == "pass" &&
			(latestAssessment.RemainingWork == "publication" || latestAssessment.RemainingWork == "none") {
			if s.recoverNativeReleaseHandoff(ctx, detail, requests) {
				s.kickTaskLaunches()
				log.Printf("[Watchdog AI Orchestrator] Autonomously recovered native release handoff for task %s", detail.ID)
				continue
			}
		}

		// Autonomous review queueing if attempt succeeded but review evaluation is missing
		if t.Status == task.TaskStatusReview && latestAssessment == nil && detail.LatestAttempt != nil && detail.LatestAttempt.Status == task.AttemptStatusSucceeded {
			log.Printf("[Watchdog AI Orchestrator] Autonomously queueing review evaluation for review task %s", detail.ID)
			s.queueInboxTaskReview(ctx, detail)
			continue
		}

		// 2. 1-Click Action Card (Zero Cognitive Fatigue):
		// If autonomous continuation was not possible (needs owner decision, permission, or budget threshold reached),
		// present a 1-click continuation approval card in the conversation room so user can unblock in 1 tap.
		// ONLY for blocked tasks—never spam the room for internal review states.
		if t.Status == task.TaskStatusBlocked && latestAssessment != nil {
			s.postContinuationActionCard(ctx, detail, assessmentRequestID, *latestAssessment)
		}
	}
}

func (s *Server) postContinuationActionCard(ctx context.Context, detail *task.TaskDetailResponse, requestID string, assessment inboxReviewEvaluation) {
	// Deduplicate: check if an action card or notification for this attempt has already been posted to the room
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, detail.SourceRoomID)
	if err != nil {
		return
	}

	if detail.LatestAttempt != nil {
		for _, m := range messages {
			if (strings.Contains(m.Content, "```cuan-approval") || strings.Contains(m.Content, "Tandai Tuntas")) &&
				(strings.Contains(m.Content, detail.LatestAttempt.ID) || strings.Contains(m.Content, detail.ID)) {
				// Already posted an action card or notification for this attempt/task
				return
			}
		}
	}

	// 1. If blocked by external work (e.g. cuan-landing without VPS deployment, GitHub PR merge needed),
	// never fabricate a cuan-approval card because external actions cannot be performed by the local code runner.
	// Instead, post an informative, actionable message with direct resolution links.
	if assessment.RemainingWork == "external" {
		finding := strings.TrimSpace(assessment.Finding)
		if finding == "" {
			finding = strings.TrimSpace(assessment.Explanation)
		}
		if finding == "" {
			finding = "Perubahan kode telah diterapkan pada repositori, namun memerlukan deployment atau tindakan eksternal."
		}
		content := fmt.Sprintf("%s\n\nPerubahan kode telah siap di GitHub/repositori. Anda dapat menandai tugas ini selesai jika deployment dilakukan terpisah, atau memeriksa konfigurasi deployment proyek:\n\n• [Tandai Tuntas](/tasks/%s)\n• [Pengaturan Proyek](/projects/%s/settings)",
			finding, detail.ID, detail.ProjectID)
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
			log.Printf("[Watchdog AI Orchestrator] Posted external blocker notification for task %s to room %s",
				detail.ID, detail.SourceRoomID)
			s.publishInbox(detail.SourceRoomID)
		}
		return
	}

	// 2. Internal work: build authentic continuation plan matching continuationPresentation
	if strings.TrimSpace(assessment.Revision) == "" {
		if strings.TrimSpace(assessment.Explanation) != "" {
			assessment.Revision = strings.TrimSpace(assessment.Explanation)
		} else {
			assessment.Revision = strings.TrimSpace(assessment.Finding)
		}
	}
	planBlock := continuationPlan(detail, requestID, assessment)

	finding := strings.TrimSpace(assessment.Finding)
	if finding == "" {
		finding = strings.TrimSpace(assessment.Explanation)
	}
	if finding == "" {
		finding = "Pekerjaan memerlukan keputusan Anda untuk melanjutkan."
	}

	var content string
	if planBlock != "" {
		content = fmt.Sprintf("%s\n\nUntuk melanjutkan perbaikan, silakan konfirmasi melalui tombol di bawah:%s\n\n[View work](/tasks/%s)", finding, planBlock, detail.ID)
	} else {
		content = fmt.Sprintf("%s\n\n[View work](/tasks/%s)", finding, detail.ID)
	}

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
	if (telemetry.DiskUsedPercent <= 88.0 && !telemetry.DiskWarning) || s.projectStudioRepo == nil {
		return
	}
	watchdogAlertMu.Lock()
	defer watchdogAlertMu.Unlock()
	if !lastStorageAlertTime.IsZero() && time.Since(lastStorageAlertTime) < 6*time.Hour {
		return
	}

	// 1. Search for an existing unresolved system storage alert room
	var targetRoomID string
	projID := "default"

	rooms, err := s.projectStudioRepo.ListAllRooms(ctx)
	if err == nil {
		for _, r := range rooms {
			if r.ResolvedAt != nil || s.isCustomerRoom(ctx, r.ID) {
				continue
			}
			if r.Topic == "agent-topic:system:storage" || r.Title == "Peringatan Kapasitas Server" {
				targetRoomID = r.ID
				if r.ProjectID != "" {
					projID = r.ProjectID
				}
				break
			}
		}
	}

	// 2. If no existing dedicated room found, create a new room specifically for storage alerts
	if targetRoomID == "" {
		now := time.Now().UTC()
		targetRoomID = db.GenerateUUID()
		newRoom := &db.ProjectRoom{
			ID:        targetRoomID,
			ProjectID: projID,
			Title:     "Peringatan Kapasitas Server",
			Topic:     "agent-topic:system:storage",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.projectStudioRepo.CreateRoom(ctx, newRoom); err != nil {
			log.Printf("[Watchdog] Failed to create storage alert room: %v", err)
			return
		}
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
		s.publishInbox(targetRoomID)
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

	var allRecentMessages map[string][]db.ProjectMessage
	if s.projectStudioRepo != nil {
		allRecentMessages, _ = s.projectStudioRepo.ListRecentMessages(ctx, 1)
	}

	preferences := itemPreferences{}
	if s.authSvc != nil {
		if ownerID, err := s.authSvc.OwnerID(ctx); err == nil && ownerID != "" {
			itemPreferencesMu.Lock()
			saved, _ := readItemPreferences(ownerID)
			itemPreferencesMu.Unlock()
			preferences = saved
		}
	}

	for _, room := range rooms {
		msgs := allRecentMessages[room.ID]
		if len(msgs) == 0 {
			continue
		}

		latestMsg := msgs[len(msgs)-1]
		if room.IsResolved(latestMsg.CreatedAt) {
			continue
		}

		// Ensure there are no active, running, queued, or blocked tasks/launches/requests
		blocker, err := s.conversationResolutionBlocker(ctx, room.ID)
		if err != nil || blocker != "" {
			continue
		}

		// Check if there are tasks for this room
		tasks, _ := s.taskSvc.Store().ListTasks(ctx, task.TaskFilter{SourceRoomID: room.ID})
		hasTasks := len(tasks) > 0
		if !hasTasks {
			legacy, _ := s.projectStudioRepo.ListTasksByRoomID(ctx, room.ID)
			hasTasks = len(legacy) > 0
		}

		roomCutoff := cutoff // default 24h for customer chats without tasks
		if hasTasks {
			// For completed tasks: 15-minute grace period after completion/read
			roomCutoff = time.Now().UTC().Add(-15 * time.Minute)
			allTasksOld := true
			for _, t := range tasks {
				if t.UpdatedAt.After(roomCutoff) {
					allTasksOld = false
					break
				}
			}
			if !allTasksOld {
				continue
			}
		} else {
			// Fast path for non-task chats: check 24h cutoff
			if latestMsg.CreatedAt.After(roomCutoff) {
				continue
			}
			// Normal chat without tasks:
			// Unanswered customer messages must NEVER be auto-resolved
			if latestMsg.AuthorType != "agent" && latestMsg.AuthorType != "system" {
				continue
			}
		}

		// Enforce read-aware grace period: if the latest message was sent by an agent/system,
		// the grace countdown must only begin AFTER the operator has actually read it.
		// If unread, it must remain in the Inbox so the operator never misses the completed result.
		if latestMsg.AuthorType == "agent" || latestMsg.AuthorType == "system" {
			var readAt *time.Time

			if isSystemLifecycleNotice(latestMsg) {
				copied := latestMsg.CreatedAt
				readAt = &copied
			}

			// 1. Check message receipts for confirmed read
			if readAt == nil {
				if receiptRepo, ok := s.projectStudioRepo.(db.ChatReceiptRepository); ok {
					if receipts, err := receiptRepo.MessageReceipts(ctx, room.ID); err == nil {
						if r, exists := receipts[latestMsg.ID]; exists && r.ReadConfirmedAt != nil {
							copied := *r.ReadConfirmedAt
							readAt = &copied
						}
					}
				}
			}

			// 2. Check operator conversation item preferences
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

			// If the completion message has never been explicitly marked with a read receipt:
			if readAt == nil {
				// If customer live chat and answered by system/agent > 24h ago with no visitor replies:
				if !hasTasks && latestMsg.CreatedAt.Before(cutoff) {
					readAt = &latestMsg.CreatedAt
				} else if hasTasks && latestMsg.CreatedAt.Before(roomCutoff) {
					readAt = &latestMsg.CreatedAt
				} else {
					continue
				}
			}

			// Grace period must elapse AFTER the message was read
			if readAt.After(roomCutoff) {
				continue
			}
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

func extractNativeReleaseHandoffCommit(activities []*task.TaskActivity) string {
	for _, a := range activities {
		if a != nil && a.ActivityType == "native_release_handoff" {
			var p struct {
				SourceCommit string `json:"source_commit"`
			}
			if json.Unmarshal([]byte(a.Message), &p) == nil && releaseObjectID.MatchString(p.SourceCommit) {
				return p.SourceCommit
			}
		}
	}
	return ""
}

// reconcileVerifiedPublicationTasks scans tasks in status 'review' or 'blocked'
// and checks if their applied or result commit is already verified and contained
// in the running live release. If so, it autonomously transitions the task to 'done',
// marks all DoD items completed, and posts the completion delivery report to the chat room.
func (s *Server) reconcileVerifiedPublicationTasks(ctx context.Context) {
	if s.taskSvc == nil || s.projectStudioRepo == nil {
		return
	}

	reviewTasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: task.TaskStatusReview})
	blockedTasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: task.TaskStatusBlocked})

	var candidates []*task.TaskItemResponse
	candidates = append(candidates, reviewTasks...)
	candidates = append(candidates, blockedTasks...)
	if len(candidates) == 0 {
		return
	}

	for _, item := range candidates {
		if item == nil || item.ID == "" {
			continue
		}
		detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
		if err != nil || detail == nil || detail.Status == task.TaskStatusDone || detail.Status == "cancelled" || detail.Status == "canceled" {
			continue
		}

		// 1. Code tasks: Check if result commit, applied commit, or handoff commit is verified in live release
		if detail.ExecutionCategory == "" || detail.ExecutionCategory == task.ExecutionCategoryCode {
			handoffCommit := extractNativeReleaseHandoffCommit(detail.Activities)
			obs := s.inboxReviewPublication(ctx, detail)
			isVerified := false
			verifiedCommit := ""

			if obs != nil && obs.Verified && obs.Provenance != "verified_current_project_snapshot" && (obs.ResultCommitContained == nil || *obs.ResultCommitContained) {
				isVerified = true
				verifiedCommit = obs.Commit
			} else if s.projectSvc != nil && detail.ProjectID != "" {
				if running := s.runningProjectRelease(ctx, detail); running != nil && running.Commit != "" {
					checkList := []string{detail.CommitSHA, extractAppliedCommitSHA(detail.CompleteNote), handoffCommit}
					for _, cand := range checkList {
						if cand != "" && releaseObjectID.MatchString(cand) {
							if cand == running.Commit {
								isVerified = true
								verifiedCommit = cand
								break
							}
							if contained, err := s.projectSvc.PublishedCommitContained(ctx, detail.ProjectID, cand, running.Commit); err == nil && contained {
								isVerified = true
								verifiedCommit = cand
								break
							}
						}
					}
				}
			}

			if isVerified {
				completedDoD := make([]task.DoDItem, len(detail.DoDChecklist))
				for i, itm := range detail.DoDChecklist {
					completedDoD[i] = task.DoDItem{
						ID:        itm.ID,
						Text:      itm.Text,
						Evidence:  itm.Evidence,
						Completed: true,
					}
				}
				if detail.Status != task.TaskStatusReview {
					reviewStatus := task.TaskStatusReview
					_, _ = s.taskSvc.UpdateTask(ctx, detail.ID, "scrum_master", task.UpdateTaskRequest{
						Status:       &reviewStatus,
						DoDChecklist: &completedDoD,
					})
				}

				doneStatus := task.TaskStatusDone
				note := detail.CompleteNote
				displayCommit := verifiedCommit
				if len(displayCommit) > 8 {
					displayCommit = displayCommit[:8]
				}
				verifiedMarker := fmt.Sprintf("🚀 Perubahan telah diverifikasi aktif pada rilis live (%s).", displayCommit)
				if !strings.Contains(note, "telah diverifikasi aktif pada rilis live") && !strings.Contains(note, "Perubahan telah berhasil diterapkan") {
					if note != "" {
						note += "\n\n"
					}
					note += verifiedMarker
				}
				summary := strings.TrimSpace(detail.Summary)
				if summary == "" {
					summary = fmt.Sprintf("Pekerjaan '%s' telah selesai dan diverifikasi aktif pada rilis produksi.", detail.Title)
				}

				updated, err := s.taskSvc.UpdateTask(ctx, detail.ID, "scrum_master", task.UpdateTaskRequest{
					Status:       &doneStatus,
					Summary:      &summary,
					CompleteNote: &note,
					DoDChecklist: &completedDoD,
				})
				if err == nil && updated != nil {
					log.Printf("[Watchdog AI Orchestrator] Autonomously reconciled verified task %s -> DONE (commit: %s)",
						detail.ID, verifiedCommit)
					detail.Task = *updated
					_ = s.PostTaskCompletionMessage(ctx, updated)
					s.DispatchTaskCompletedPush(ctx, updated)
					if detail.SourceRoomID != "" {
						if blocker, err := s.conversationResolutionBlocker(ctx, detail.SourceRoomID); err == nil && blocker == "" {
							now := time.Now().UTC()
							_ = s.projectStudioRepo.SetRoomResolved(ctx, detail.SourceRoomID, &now)
						}
						s.publishInbox(detail.SourceRoomID)
					}
					continue
				}
			}
		}

		// 2. Audit or non-code/ops tasks: Check if completed with full verification report
		if (isInvestigationOrNonCodeText(detail.Title) || strings.Contains(strings.ToLower(detail.Title), "pemeliharaan") || strings.Contains(strings.ToLower(detail.Title), "audit") || detail.ExecutionCategory == task.ExecutionCategoryOps || detail.ExecutionCategory == task.ExecutionCategoryReadOnly) &&
			strings.TrimSpace(detail.Summary) != "" && strings.TrimSpace(detail.CompleteNote) != "" &&
			(strings.Contains(detail.CompleteNote, "berhasil") || strings.Contains(detail.CompleteNote, "lulus") || strings.Contains(detail.CompleteNote, "100%")) {
			completedDoD := make([]task.DoDItem, len(detail.DoDChecklist))
			for i, itm := range detail.DoDChecklist {
				completedDoD[i] = task.DoDItem{
					ID:        itm.ID,
					Text:      itm.Text,
					Evidence:  itm.Evidence,
					Completed: true,
				}
			}
			if detail.Status != task.TaskStatusReview {
				reviewStatus := task.TaskStatusReview
				_, _ = s.taskSvc.UpdateTask(ctx, detail.ID, "scrum_master", task.UpdateTaskRequest{
					Status:       &reviewStatus,
					DoDChecklist: &completedDoD,
				})
			}
			doneStatus := task.TaskStatusDone
			summary := strings.TrimSpace(detail.Summary)
			note := detail.CompleteNote
			updated, err := s.taskSvc.UpdateTask(ctx, detail.ID, "scrum_master", task.UpdateTaskRequest{
				Status:       &doneStatus,
				Summary:      &summary,
				CompleteNote: &note,
				DoDChecklist: &completedDoD,
			})
			if err == nil && updated != nil {
				log.Printf("[Watchdog AI Orchestrator] Autonomously reconciled completed audit task %s -> DONE", detail.ID)
				detail.Task = *updated
				_ = s.PostTaskCompletionMessage(ctx, updated)
				s.DispatchTaskCompletedPush(ctx, updated)
				if detail.SourceRoomID != "" {
					if blocker, err := s.conversationResolutionBlocker(ctx, detail.SourceRoomID); err == nil && blocker == "" {
						now := time.Now().UTC()
						_ = s.projectStudioRepo.SetRoomResolved(ctx, detail.SourceRoomID, &now)
					}
					s.publishInbox(detail.SourceRoomID)
				}
			}
		}
	}
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
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
	}

	// 3. Ensure queue launches are drained and orphan launches reconciled
	if s.taskSvc != nil {
		_ = s.taskSvc.ReconcileOrphanLaunches(ctx, time.Now().UTC())
	}
	s.kickTaskLaunches()
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
		var origin *db.InboxRequest
		if assessmentRequestID != "" {
			for i := range requests {
				if requests[i].RequestID == assessmentRequestID {
					origin = &requests[i]
					break
				}
			}
		}
		if origin == nil {
			for i := range requests {
				if requests[i].MessageID == detail.SourceMessageID {
					origin = &requests[i]
					break
				}
			}
		}
		if origin == nil && len(requests) > 0 {
			origin = &requests[0]
		}
		if origin == nil {
			continue
		}

		attemptCount := taskRevisionAttemptCount(detail)

		// 1. Autonomous Self-Healing:
		// When within budget (< 5 attempts), manager indicates continue/replan, and actionable revision is available
		if latestAssessment != nil && attemptCount < 5 &&
			(latestAssessment.ManagerAction == "continue" || latestAssessment.ManagerAction == "replan") &&
			latestAssessment.Decision == "revise" &&
			latestAssessment.RequestedOutcome != "needs_input" &&
			strings.TrimSpace(latestAssessment.Revision) != "" {

			if origin.AgentID == "" {
				if detail.AssignedAgent != "" {
					origin.AgentID = detail.AssignedAgent
				} else {
					origin.AgentID = "agent_cuan"
				}
			}

			launchID := delegationMessageID(fmt.Sprintf("watchdog-continuation:%s:%s:%d", detail.ID, origin.RequestID, len(detail.Attempts)+1))
			notice := s.startAuthorizedLaunch(ctx, *origin, detail, launchID, origin.RequestID, latestAssessment.Revision)
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
		plan := inboxApprovalPlan{
			Title:          planTitle,
			Description:    planDesc,
			ProjectID:      detail.ProjectID,
			RequiredStages: []string{"implement"},
			Continuation: &inboxContinuationPlan{
				TaskID:              detail.ID,
				AttemptID:           attemptID,
				AssessmentRequestID: requestID,
				Revision:            assessment.Revision,
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

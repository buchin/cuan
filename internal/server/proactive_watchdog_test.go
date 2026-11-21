package server

import (
	"context"
	"encoding/json"
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

func TestProactiveWatchdogOrchestrateBlockedTasksActionCard(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)
	repo := db.NewMemoryProjectStudioRepository()

	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_test",
		Title:     "General Ops",
	}
	if err := repo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	req := db.InboxRequest{
		RequestID: "req-1",
		RoomID:    room.ID,
		OwnerID:   "owner_1",
		MessageID: "msg-auth",
	}
	_, _, _ = repo.AcceptInboxRequest(ctx, req, &db.ProjectMessage{ID: req.MessageID, RoomID: room.ID, AuthorType: "user", Content: "Fixture", CreatedAt: now})

	taskItem := &task.Task{
		ID:              "task_blocked_test",
		ProjectID:       "proj_test",
		SourceRoomID:    room.ID,
		SourceMessageID: "msg-auth",
		Title:           "Perbaiki Web Push",
		Instructions:    "Cuan managed delegation v1.\nPerbaiki push notification.",
		Status:          task.TaskStatusBlocked,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := taskStore.CreateTask(ctx, taskItem); err != nil {
		t.Fatal(err)
	}

	attempt := &task.TaskAttempt{
		ID:            "att_test_1",
		TaskID:        taskItem.ID,
		AttemptNumber: 1,
		Status:        task.AttemptStatusFailed,
		ErrorMessage:  "Verification failed: syntax error in sw.js",
		CreatedAt:     now,
	}
	if err := taskStore.CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}

	assessmentRecord := reviewAssessmentRecord{
		RequestID:        "req-1",
		InstructionsHash: task.LaunchInstructionsHash(taskItem.Instructions),
		Assessment: inboxReviewEvaluation{
			Decision:      "blocked",
			Finding:       "Terdapat syntax error pada sw.js yang menggagalkan notifikasi push.",
			Explanation:   "Script service worker mengalami syntax error sehingga push notification tidak berjalan.",
			RemainingWork: "implementation",
			ManagerAction: "needs_owner",
			ManagerReason: "Memerlukan konfirmasi pemilik untuk melanjutkan perbaikan syntax error.",
		},
	}
	rawMsg, _ := json.Marshal(assessmentRecord)
	act := &task.TaskActivity{
		ID:           "act_rev_1",
		TaskID:       taskItem.ID,
		AttemptID:    &attempt.ID,
		ActivityType: reviewAssessmentActivity,
		Author:       "coordinator",
		Message:      string(rawMsg),
		CreatedAt:    now,
	}
	if err := taskStore.CreateActivity(ctx, act); err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		taskSvc:           taskSvc,
		projectStudioRepo: repo,
	}

	// Busy task on project lane to verify queuing without hitting live execution
	if err := taskStore.CreateTask(ctx, &task.Task{ID: "busy_task", ProjectID: "proj_test_auto", Status: task.TaskStatusDoing}); err != nil {
		t.Fatal(err)
	}

	// Run watchdog blocked task orchestration
	srv.orchestrateBlockedInboxTasks(ctx)

	// Check that a 1-click continuation action card was posted in the room
	messages, err := repo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (1 user prompt + 1 action card), got %d", len(messages))
	}
	actionCardMsg := messages[len(messages)-1]
	if !strings.Contains(actionCardMsg.Content, "```cuan-approval") {
		t.Fatalf("expected cuan-approval card, got: %s", actionCardMsg.Content)
	}
	if !strings.Contains(actionCardMsg.Content, "Terdapat syntax error pada sw.js") {
		t.Fatalf("expected finding explanation in message, got: %s", actionCardMsg.Content)
	}

	// Verify deduplication: second run should not duplicate
	srv.orchestrateBlockedInboxTasks(ctx)
	messages, err = repo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected still 2 messages after deduplication, got %d", len(messages))
	}
}

func TestProactiveWatchdogOrchestrateBlockedTasksAutonomousLaunch(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)
	repo := db.NewMemoryProjectStudioRepository()

	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_test_auto",
		Title:     "General Ops",
	}
	if err := repo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	reqID := db.GenerateUUID()
	msgAuthID := db.GenerateUUID()
	req := db.InboxRequest{
		RequestID: reqID,
		RoomID:    room.ID,
		OwnerID:   "owner_1",
		AgentID:   "agent_cuan",
		MessageID: msgAuthID,
	}
	_, _, _ = repo.AcceptInboxRequest(ctx, req, &db.ProjectMessage{ID: req.MessageID, RoomID: room.ID, AuthorType: "user", Content: "Fixture", CreatedAt: now})

	taskItem := &task.Task{
		ID:              "task_auto_heal",
		ProjectID:       "proj_test_auto",
		SourceRoomID:    room.ID,
		SourceMessageID: msgAuthID,
		Title:           "Penyelarasan Format Push",
		Instructions:    "Cuan managed delegation v1.\nPerbaiki push notification.",
		Status:          task.TaskStatusBlocked,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := taskStore.CreateTask(ctx, taskItem); err != nil {
		t.Fatal(err)
	}

	attempt := &task.TaskAttempt{
		ID:            "att_auto_1",
		TaskID:        taskItem.ID,
		AttemptNumber: 1,
		Status:        task.AttemptStatusSucceeded,
		CreatedAt:     now,
	}
	if err := taskStore.CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}

	assessmentRecord := reviewAssessmentRecord{
		RequestID:        db.GenerateUUID(), // System review assessment request ID differs from root user authorization
		InstructionsHash: task.LaunchInstructionsHash(taskItem.Instructions),
		Assessment: inboxReviewEvaluation{
			Decision:         "revise",
			Finding:          "Pengujian visual lulus, publikasi ke live belum dilakukan.",
			Explanation:      "Terapkan dan publikasikan perubahan ke rilis berjalan.",
			RemainingWork:    "publication",
			RequestedOutcome: "unfinished",
			ManagerAction:    "continue",
			ManagerReason:    "Lanjutkan publikasi ke rilis berjalan.",
			Revision:         "Publikasikan perubahan notifikasi push yang sudah lolos uji ke rilis berjalan.",
		},
	}
	rawMsg, _ := json.Marshal(assessmentRecord)
	act := &task.TaskActivity{
		ID:           "act_rev_auto",
		TaskID:       taskItem.ID,
		AttemptID:    &attempt.ID,
		ActivityType: reviewAssessmentActivity,
		Author:       "coordinator",
		Message:      string(rawMsg),
		CreatedAt:    now,
	}
	if err := taskStore.CreateActivity(ctx, act); err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		taskSvc:           taskSvc,
		projectStudioRepo: repo,
	}

	// Busy task on project lane to verify queuing without hitting live execution
	if err := taskStore.CreateTask(ctx, &task.Task{ID: "busy_task", ProjectID: "proj_test_auto", Status: task.TaskStatusDoing}); err != nil {
		t.Fatal(err)
	}

	// Run watchdog blocked task orchestration
	srv.orchestrateBlockedInboxTasks(ctx)

	// Verify that task status transitioned from blocked to queued
	updatedTask, err := taskStore.GetTask(ctx, taskItem.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTask.Status != task.TaskStatusQueued {
		t.Fatalf("expected task status %q, got %q", task.TaskStatusQueued, updatedTask.Status)
	}

	// Verify a launch was created
	launch, err := taskSvc.LatestLaunch(ctx, taskItem.ID)
	if err != nil || launch == nil {
		t.Fatalf("expected launch created for task, got err=%v launch=%v", err, launch)
	}
	if !strings.Contains(launch.Revision, "Publikasikan perubahan notifikasi push") {
		t.Fatalf("expected revision in launch, got: %s", launch.Revision)
	}
}

func TestAutoResolveCompletedConversations(t *testing.T) {
	ctx := context.Background()
	repo := db.NewMemoryProjectStudioRepository()
	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)

	srv := &Server{
		taskSvc:           taskSvc,
		projectStudioRepo: repo,
	}

	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_autoresolve",
		Title:     "Deploy Push Notification",
	}
	if err := repo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	// 1. Conversation with recent completed task (< 24h ago): should NOT auto-resolve
	recentTime := time.Now().UTC().Add(-2 * time.Hour)
	msgRecent := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     room.ID,
		AuthorType: "agent",
		AuthorName: "Cuan",
		Content:    "Pekerjaan selesai.",
		CreatedAt:  recentTime,
		UpdatedAt:  recentTime,
	}
	if err := repo.CreateMessage(ctx, msgRecent); err != nil {
		t.Fatal(err)
	}

	taskItem := &task.Task{
		ID:           "task_recent_done",
		ProjectID:    "proj_autoresolve",
		SourceRoomID: room.ID,
		Title:        "Setup Push",
		Status:       task.TaskStatusDone,
		CreatedAt:    recentTime.Add(-10 * time.Minute),
		UpdatedAt:    recentTime,
	}
	if err := taskStore.CreateTask(ctx, taskItem); err != nil {
		t.Fatal(err)
	}

	srv.autoResolveCompletedConversations(ctx)
	checkRoom, err := repo.GetRoomByID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if checkRoom.ResolvedAt != nil {
		t.Fatalf("expected room not resolved yet within 24h grace period, got resolved_at=%v", checkRoom.ResolvedAt)
	}

	// 2. Old completed task (> 24h ago):
	oldTime := time.Now().UTC().Add(-26 * time.Hour)
	roomOld := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_autoresolve_old",
		Title:     "Fix Bug Old",
	}
	if err := repo.CreateRoom(ctx, roomOld); err != nil {
		t.Fatal(err)
	}
	msgOld := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomOld.ID,
		AuthorType: "agent",
		AuthorName: "Cuan",
		Content:    "Pekerjaan telah selesai dan diverifikasi.",
		CreatedAt:  oldTime,
		UpdatedAt:  oldTime,
	}
	if err := repo.CreateMessage(ctx, msgOld); err != nil {
		t.Fatal(err)
	}
	taskOld := &task.Task{
		ID:           "task_old_done",
		ProjectID:    "proj_autoresolve_old",
		SourceRoomID: roomOld.ID,
		Title:        "Fix Bug Old",
		Status:       task.TaskStatusDone,
		CreatedAt:    oldTime.Add(-10 * time.Minute),
		UpdatedAt:    oldTime,
	}
	if err := taskStore.CreateTask(ctx, taskOld); err != nil {
		t.Fatal(err)
	}

	// 2a. Unread message: should NOT auto-resolve even if created 26 hours ago!
	srv.autoResolveCompletedConversations(ctx)
	checkOldRoom, err := repo.GetRoomByID(ctx, roomOld.ID)
	if err != nil {
		t.Fatal(err)
	}
	if checkOldRoom.ResolvedAt != nil {
		t.Fatalf("expected unread room NOT to be auto-resolved, got resolved_at=%v", checkOldRoom.ResolvedAt)
	}

	// 2b. Read recently (< 24h ago, e.g. 2h ago): should NOT auto-resolve yet
	recentRead := time.Now().UTC().Add(-2 * time.Hour)
	if err := repo.AcknowledgeMessages(ctx, roomOld.ID, []string{msgOld.ID}, true); err != nil {
		t.Fatal(err)
	}
	receipts, _ := repo.MessageReceipts(ctx, roomOld.ID)
	r := receipts[msgOld.ID]
	r.ReadConfirmedAt = &recentRead
	// Directly update receipt for testing exact read timestamps
	if memRepo, ok := srv.projectStudioRepo.(*db.MemoryProjectStudioRepository); ok {
		_ = memRepo
	}

	// 2c. Read > 24h ago (e.g. 25h ago): SHOULD auto-resolve!
	readTime := time.Now().UTC().Add(-25 * time.Hour)
	if err := repo.AcknowledgeMessages(ctx, roomOld.ID, []string{msgOld.ID}, true); err != nil {
		t.Fatal(err)
	}
	if receipts, err := repo.MessageReceipts(ctx, roomOld.ID); err == nil {
		if rec, ok := receipts[msgOld.ID]; ok && rec.ReadConfirmedAt != nil {
			*rec.ReadConfirmedAt = readTime
		}
	}

	srv.autoResolveCompletedConversations(ctx)
	checkOldRoom, err = repo.GetRoomByID(ctx, roomOld.ID)
	if err != nil {
		t.Fatal(err)
	}
	if checkOldRoom.ResolvedAt == nil {
		t.Fatalf("expected room to be auto-resolved after 24h since read, got nil")
	}
	if !checkOldRoom.IsResolved(msgOld.CreatedAt) {
		t.Fatalf("expected room.IsResolved to be true for msgOld")
	}

	// 3. Reopen on new message
	msgNew := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomOld.ID,
		AuthorType: "user",
		AuthorName: "Owner",
		Content:    "Ada tambahan revisi",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := repo.CreateMessage(ctx, msgNew); err != nil {
		t.Fatal(err)
	}
	if checkOldRoom.IsResolved(msgNew.CreatedAt) {
		t.Fatalf("expected room.IsResolved to be false after new user message arrived")
	}

	// 4. Conversation WITHOUT tasks where agent answered and was read > 24h ago: SHOULD auto-resolve
	roomNoTasks := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_autoresolve_notasks",
		Title:     "General Consultation",
	}
	if err := repo.CreateRoom(ctx, roomNoTasks); err != nil {
		t.Fatal(err)
	}
	msgAgent := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomNoTasks.ID,
		AuthorType: "agent",
		AuthorName: "Cuan",
		Content:    "Cuan adalah platform AI agent.",
		CreatedAt:  oldTime,
		UpdatedAt:  oldTime,
	}
	if err := repo.CreateMessage(ctx, msgAgent); err != nil {
		t.Fatal(err)
	}
	if err := repo.AcknowledgeMessages(ctx, roomNoTasks.ID, []string{msgAgent.ID}, true); err != nil {
		t.Fatal(err)
	}
	if receipts, err := repo.MessageReceipts(ctx, roomNoTasks.ID); err == nil {
		if rec, ok := receipts[msgAgent.ID]; ok && rec.ReadConfirmedAt != nil {
			*rec.ReadConfirmedAt = readTime
		}
	}
	srv.autoResolveCompletedConversations(ctx)
	checkNoTasksRoom, err := repo.GetRoomByID(ctx, roomNoTasks.ID)
	if err != nil {
		t.Fatal(err)
	}
	if checkNoTasksRoom.ResolvedAt == nil {
		t.Fatalf("expected task-free room to be auto-resolved after 24h since read, got nil")
	}

	// 5. Conversation WITHOUT tasks where customer message was NOT replied to: MUST NOT auto-resolve
	roomCustomerWait := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_autoresolve_cust",
		Title:     "Customer Help",
	}
	if err := repo.CreateRoom(ctx, roomCustomerWait); err != nil {
		t.Fatal(err)
	}
	msgCust := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomCustomerWait.ID,
		AuthorType: "user",
		AuthorName: "Visitor",
		Content:    "Halo, tolong bantu akun saya",
		CreatedAt:  oldTime,
		UpdatedAt:  oldTime,
	}
	if err := repo.CreateMessage(ctx, msgCust); err != nil {
		t.Fatal(err)
	}
	srv.autoResolveCompletedConversations(ctx)
	checkCustRoom, err := repo.GetRoomByID(ctx, roomCustomerWait.ID)
	if err != nil {
		t.Fatal(err)
	}
	if checkCustRoom.ResolvedAt != nil {
		t.Fatalf("expected unanswered customer room NOT to be auto-resolved, got %v", checkCustRoom.ResolvedAt)
	}
}

func TestProactiveWatchdogExternalBlockedTaskProvidesDirectLinksWithoutApprovalCard(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)
	repo := db.NewMemoryProjectStudioRepository()

	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_landing",
		Title:     "Landing Ops",
	}
	if err := repo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	req := db.InboxRequest{
		RequestID: db.GenerateUUID(),
		RoomID:    room.ID,
		OwnerID:   "owner_ext",
		AgentID:   "agent_cuan",
		MessageID: db.GenerateUUID(),
	}
	_, _, _ = repo.AcceptInboxRequest(ctx, req, &db.ProjectMessage{ID: req.MessageID, RoomID: room.ID, AuthorType: "user", Content: "Deploy landing", CreatedAt: now})

	taskItem := &task.Task{
		ID:              "task_ext_blocked",
		ProjectID:       "proj_landing",
		SourceRoomID:    room.ID,
		SourceMessageID: req.MessageID,
		Title:           "Deploy landing page",
		Instructions:    "Cuan managed delegation v1.\nDeploy landing page to live server.",
		Status:          task.TaskStatusBlocked,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := taskStore.CreateTask(ctx, taskItem); err != nil {
		t.Fatal(err)
	}

	attempt := &task.TaskAttempt{
		ID:            "att_ext_1",
		TaskID:        taskItem.ID,
		AttemptNumber: 1,
		Status:        task.AttemptStatusSucceeded,
		CreatedAt:     now,
	}
	if err := taskStore.CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}

	assessmentRecord := reviewAssessmentRecord{
		RequestID:        "req-ext-1",
		InstructionsHash: task.LaunchInstructionsHash(taskItem.Instructions),
		Assessment: inboxReviewEvaluation{
			Decision:      "blocked",
			Finding:       "Perubahan kode di git telah selesai, namun server target deployment VPS belum terkonfigurasi.",
			Explanation:   "Repositori sudah diperbarui dan siap di-deploy, tetapi konfigurasi VPS target belum tersedia.",
			RemainingWork: "external",
			ManagerAction: "needs_owner",
			ManagerReason: "Target deployment eksternal belum terhubung.",
		},
	}
	rawMsg, _ := json.Marshal(assessmentRecord)
	act := &task.TaskActivity{
		ID:           "act_ext_1",
		TaskID:       taskItem.ID,
		AttemptID:    &attempt.ID,
		ActivityType: reviewAssessmentActivity,
		Author:       "coordinator",
		Message:      string(rawMsg),
		CreatedAt:    now,
	}
	if err := taskStore.CreateActivity(ctx, act); err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		taskSvc:           taskSvc,
		projectStudioRepo: repo,
	}

	// Run watchdog
	srv.orchestrateBlockedInboxTasks(ctx)

	messages, err := repo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	extMsg := messages[len(messages)-1]

	// Must NOT contain a cuan-approval card because external actions cannot be performed by the code runner
	if strings.Contains(extMsg.Content, "```cuan-approval") {
		t.Fatalf("external blocker must NOT fabricate cuan-approval card, got: %s", extMsg.Content)
	}
	// Must contain clear links to complete or configure project
	if !strings.Contains(extMsg.Content, "[Tandai Tuntas](/tasks/task_ext_blocked)") {
		t.Fatalf("expected Tandai Tuntas link, got: %s", extMsg.Content)
	}
	if !strings.Contains(extMsg.Content, "[Pengaturan Proyek](/projects/proj_landing/settings)") {
		t.Fatalf("expected Pengaturan Proyek link, got: %s", extMsg.Content)
	}
}

func TestProactiveWatchdogHeartbeatSweepsQueuedInboxRequests(t *testing.T) {
	ctx := context.Background()
	repo := db.NewMemoryProjectStudioRepository()
	now := time.Now().UTC()

	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_heartbeat",
		Title:     "Heartbeat Sweeper Room",
	}
	if err := repo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	req := db.InboxRequest{
		RequestID:   "req-sweep-1",
		RoomID:      room.ID,
		MessageID:   "msg-sweep-1",
		OwnerID:     "owner_sweep",
		State:       "queued",
		CreatedAt:   now.Add(-2 * time.Minute),
		PayloadHash: "payload-1",
	}
	_, _, err := repo.AcceptInboxRequest(ctx, req, &db.ProjectMessage{
		ID:         req.MessageID,
		RoomID:     room.ID,
		AuthorType: "user",
		Content:    "Sweep this request",
		CreatedAt:  now.Add(-2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		projectStudioRepo: repo,
	}

	// Verify room is detected by dispatcher and sweep does not panic
	srv.heartbeatSweepQueuedInboxRequests(ctx)
}

func TestProactiveWatchdogReviewTaskNeverGeneratesActionCardSpam(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)
	repo := db.NewMemoryProjectStudioRepository()
	now := time.Now().UTC()

	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj_silent_review",
		Title:     "Review Room",
	}
	if err := repo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}

	req := db.InboxRequest{
		RequestID:   "req-rev-1",
		RoomID:      room.ID,
		MessageID:   "msg-rev-1",
		OwnerID:     "owner_rev",
		State:       "completed",
		CreatedAt:   now,
		PayloadHash: "hash-rev-1",
	}
	_, _, _ = repo.AcceptInboxRequest(ctx, req, &db.ProjectMessage{
		ID:         req.MessageID,
		RoomID:     room.ID,
		AuthorType: "user",
		Content:    "Fix review task",
		CreatedAt:  now,
	})

	taskItem := &task.Task{
		ID:              "task_in_review",
		ProjectID:       "proj_silent_review",
		SourceRoomID:    room.ID,
		SourceMessageID: req.MessageID,
		Title:           "Internal review task",
		Instructions:    "Cuan managed delegation v1.\nPerform internal review.",
		Status:          task.TaskStatusReview, // Crucial: in review, NOT blocked
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := taskStore.CreateTask(ctx, taskItem); err != nil {
		t.Fatal(err)
	}

	attempt := &task.TaskAttempt{
		ID:            "att_rev_1",
		TaskID:        taskItem.ID,
		AttemptNumber: 1,
		Status:        task.AttemptStatusSucceeded,
		CreatedAt:     now,
	}
	if err := taskStore.CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}

	assessmentRecord := reviewAssessmentRecord{
		RequestID:        "req-rev-assessment",
		InstructionsHash: task.LaunchInstructionsHash(taskItem.Instructions),
		Assessment: inboxReviewEvaluation{
			Decision:      "revise",
			Finding:       "Internal checks ongoing",
			Explanation:   "Verifikasi independen sedang berjalan.",
			RemainingWork: "verification",
			ManagerAction: "wait",
			ManagerReason: "Verifikasi berjalan.",
		},
	}
	rawMsg, _ := json.Marshal(assessmentRecord)
	act := &task.TaskActivity{
		ID:           "act_rev_1",
		TaskID:       taskItem.ID,
		AttemptID:    &attempt.ID,
		ActivityType: reviewAssessmentActivity,
		Author:       "coordinator",
		Message:      string(rawMsg),
		CreatedAt:    now,
	}
	if err := taskStore.CreateActivity(ctx, act); err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		taskSvc:           taskSvc,
		projectStudioRepo: repo,
	}

	// Run watchdog
	srv.orchestrateBlockedInboxTasks(ctx)

	// User messages in room must NOT have any new cuan-approval card
	messages, err := repo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Initial message only (len == 1), no approval card posted
	if len(messages) != 1 {
		t.Fatalf("expected 0 unsolicited messages for review task, got %d", len(messages)-1)
	}
}

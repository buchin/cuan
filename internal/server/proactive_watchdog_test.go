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

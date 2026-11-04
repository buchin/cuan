package runner

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestRunnerService_DoDPass_InboxAwaitsCoordinatorAcceptance(t *testing.T) {
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	svc := NewService(runnerStore, taskStore)

	// Mock DoD Evaluator that passes 100%
	mockEval := NewDoDEvaluator()
	mockEval.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return []byte(""), nil // Clean working tree
		}
		return []byte("PASS"), nil
	}
	svc.SetDoDEvaluator(mockEval)

	// Capture proactive business chat messages
	var capturedRoomID string
	var capturedMsg string
	svc.SetChatNotifier(NewDefaultBusinessChatNotifier(func(ctx context.Context, roomID, messageContent string) error {
		capturedRoomID = roomID
		capturedMsg = messageContent
		return nil
	}))

	// Capture event bus emission
	bus := NewEventBus()
	var emittedEvt *AttemptSucceededEvent
	var evtWg sync.WaitGroup
	evtWg.Add(1)
	bus.Subscribe(func(ctx context.Context, evt AttemptSucceededEvent) error {
		emittedEvt = &evt
		evtWg.Done()
		return nil
	})
	svc.SetEventBus(bus)

	ctx := context.Background()
	taskItem := &task.Task{
		ID:           "task_store_update_1",
		Title:        "Pembaruan Etalase Produk Toko",
		Instructions: "Perbaiki tata letak etalase",
		CompleteNote: "Fixture: command runner outputs recorded for workspace verification.", Summary: "Tata letak etalase telah diperbarui dan dirapikan",
		SourceRoomID: "room_store_main",
		Status:       task.TaskStatusDoing,
		DoDChecklist: []task.DoDItem{
			{Text: "Working tree bersih", Completed: true, Evidence: "git status --porcelain returned empty"},
			{Text: "Tests lulus 100%", Completed: true, Evidence: "mock targeted test command returned PASS"},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	_ = taskStore.CreateTask(ctx, taskItem)

	att := &task.TaskAttempt{
		ID:     "att_store_1",
		TaskID: taskItem.ID, RunnerID: "runner_test",
		AttemptNumber: 1,
		Status:        task.AttemptStatusRunning,
		CreatedAt:     time.Now().UTC(),
	}
	_ = taskStore.CreateAttempt(ctx, att)

	req := CompleteAttemptRequest{
		RunnerID:             "runner_test",
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
	}

	completedAtt, err := svc.Complete(ctx, att.ID, req)
	if err != nil {
		t.Fatalf("unexpected Complete error: %v", err)
	}
	if completedAtt.Status != task.AttemptStatusSucceeded {
		t.Fatalf("expected attempt status succeeded, got %s", completedAtt.Status)
	}

	// 1. Worker evidence is ready for coordinator review, not final acceptance.
	updatedTask, err := taskStore.GetTask(ctx, taskItem.ID)
	if err != nil {
		t.Fatalf("get task error: %v", err)
	}
	if updatedTask.Status != task.TaskStatusReview {
		t.Fatalf("expected task status 'review', got %s", updatedTask.Status)
	}
	for _, item := range updatedTask.DoDChecklist {
		if !item.Completed {
			t.Fatalf("expected DoD item %q to be completed", item.Text)
		}
	}
	if updatedTask.CompleteNote != taskItem.CompleteNote {
		t.Fatal("worker replaced original report evidence")
	}

	// 2. Verify internal event task:attempt_succeeded was emitted
	evtWg.Wait()
	if emittedEvt == nil {
		t.Fatal("expected task:attempt_succeeded event to be emitted on EventBus")
	}
	if emittedEvt.EventType != EventTaskAttemptSucceeded || emittedEvt.TaskID != taskItem.ID {
		t.Fatalf("mismatched emitted event: %+v", emittedEvt)
	}

	// 3. Only the Inbox coordinator may announce acceptance.
	if capturedRoomID != "" || capturedMsg != "" {
		t.Fatal("worker announced acceptance before review")
	}

}

func TestRunnerService_DoDFixableGap_AutoRequeueWithAdaptiveTimeout(t *testing.T) {
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	svc := NewService(runnerStore, taskStore)

	// Mock DoD Evaluator that fails with a fixable gap on Attempt 1
	mockEval := NewDoDEvaluator()
	mockEval.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return []byte(" M internal/runner/runner.go\n"), nil // Dirty working tree
		}
		return []byte("PASS"), nil
	}
	svc.SetDoDEvaluator(mockEval)

	ctx := context.Background()
	taskItem := &task.Task{
		ID:           "task_requeue_1",
		Title:        "Sinkronisasi Stok Produk",
		Instructions: "Sinkronisasi stok dari supplier",
		CompleteNote: "Fixture: command runner outputs recorded for workspace verification.", Summary: "Stok produk telah disinkronkan",
		Status:    task.TaskStatusDoing,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	_ = taskStore.CreateTask(ctx, taskItem)

	att1 := &task.TaskAttempt{
		ID:     "att_req_1",
		TaskID: taskItem.ID, RunnerID: "runner_test",
		AttemptNumber: 1, // Attempt 1
		AgentName:     "agy",
		Status:        task.AttemptStatusRunning,
		CreatedAt:     time.Now().UTC(),
	}
	_ = taskStore.CreateAttempt(ctx, att1)

	req := CompleteAttemptRequest{
		RunnerID:             "runner_test",
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
	}

	_, err := svc.Complete(ctx, att1.ID, req)
	if err != nil {
		t.Fatalf("unexpected Complete error: %v", err)
	}

	// 1. Verify task is automatically returned to 'queued' for revision
	updatedTask, err := taskStore.GetTask(ctx, taskItem.ID)
	if err != nil {
		t.Fatalf("get task error: %v", err)
	}
	if updatedTask.Status != task.TaskStatusQueued {
		t.Fatalf("expected task status to be auto-requeued to 'queued', got %s", updatedTask.Status)
	}

	// 2. Verify next attempt (Attempt 2) was automatically created
	latestAtt, err := taskStore.GetLatestAttempt(ctx, taskItem.ID)
	if err != nil {
		t.Fatalf("get latest attempt error: %v", err)
	}
	if latestAtt.ID == att1.ID {
		t.Fatal("expected new attempt to be created for requeue")
	}
	if latestAtt.AttemptNumber != 2 {
		t.Fatalf("expected next attempt number to be 2, got %d", latestAtt.AttemptNumber)
	}
	if latestAtt.Status != task.AttemptStatusQueued {
		t.Fatalf("expected new attempt status 'queued', got %s", latestAtt.Status)
	}

	// 3. Verify adaptive timeout calculation for Attempt 2 is 35m
	if timeout := CalculateAdaptiveTimeout(latestAtt.AttemptNumber); timeout != 35*time.Minute {
		t.Fatalf("expected 35m timeout for Attempt 2, got %v", timeout)
	}

	// 4. Verify task activities record auto-requeue feedback
	activities, err := taskStore.ListActivities(ctx, taskItem.ID)
	if err != nil {
		t.Fatalf("list activities error: %v", err)
	}
	foundRequeueMsg := false
	for _, a := range activities {
		if strings.Contains(a.Message, "Auto-Requeue") && strings.Contains(a.Message, "attempt #2") {
			foundRequeueMsg = true
			break
		}
	}
	if !foundRequeueMsg {
		t.Fatal("expected auto-requeue activity message to be recorded in task history")
	}
}

func TestRunnerService_DoDMaxAttemptsReached_MoveToReview(t *testing.T) {
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	svc := NewService(runnerStore, taskStore)

	// Mock DoD Evaluator that fails on Attempt 3
	mockEval := NewDoDEvaluator()
	mockEval.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return []byte(" M internal/runner/runner.go\n"), nil // Dirty
		}
		return []byte("PASS"), nil
	}
	svc.SetDoDEvaluator(mockEval)

	// Inbox owns review messaging; the runner must not prematurely ask the owner to intervene.
	var capturedRooms []string
	svc.SetChatNotifier(NewDefaultBusinessChatNotifier(func(ctx context.Context, roomID, messageContent string) error {
		capturedRooms = append(capturedRooms, roomID)
		return nil
	}))

	ctx := context.Background()
	taskItem := &task.Task{
		ID:           "task_max_att",
		Title:        "Perbaikan Algoritma Rekomendasi",
		CompleteNote: "Fixture: command runner outputs recorded for workspace verification.", Summary: "Pembaruan algoritma membutuhkan penyesuaian parameter",
		SourceRoomID: "room_source_shop",
		Status:       task.TaskStatusDoing,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = taskStore.CreateTask(ctx, taskItem)

	att3 := &task.TaskAttempt{
		ID:     "att_max_3",
		TaskID: taskItem.ID, RunnerID: "runner_test",
		AttemptNumber: 3, // Already Attempt 3 (Max cap!)
		AgentName:     "agy",
		Status:        task.AttemptStatusRunning,
		CreatedAt:     time.Now().UTC(),
	}
	_ = taskStore.CreateAttempt(ctx, att3)

	req := CompleteAttemptRequest{
		RunnerID:             "runner_test",
		RoomID:               "room_active_chat",
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
	}

	_, err := svc.Complete(ctx, att3.ID, req)
	if err != nil {
		t.Fatalf("unexpected Complete error: %v", err)
	}

	// Should NOT requeue anymore, must move to 'review' for human/PO
	updatedTask, _ := taskStore.GetTask(ctx, taskItem.ID)
	if updatedTask.Status != task.TaskStatusReview {
		t.Fatalf("expected task status to be 'review' after reaching max attempts, got %s", updatedTask.Status)
	}

	if len(capturedRooms) != 0 {
		t.Fatalf("runner bypassed coordinator review: %v", capturedRooms)
	}

}

func TestRunnerService_ValidateTaskTransition_StateMachineIntegration(t *testing.T) {
	svc := NewService(nil, nil)

	// Valid transitions
	validPairs := [][2]string{
		{task.TaskStatusBacklog, task.TaskStatusQueued},
		{task.TaskStatusQueued, task.TaskStatusDoing},
		{task.TaskStatusDoing, task.TaskStatusReview},
		{task.TaskStatusDoing, task.TaskStatusDone},
		{task.TaskStatusReview, task.TaskStatusDone},
		{task.TaskStatusReview, task.TaskStatusQueued},
		{task.TaskStatusReview, task.TaskStatusDoing},
		{task.TaskStatusDoing, task.TaskStatusBlocked},
		{task.TaskStatusBlocked, task.TaskStatusDoing},
		{task.TaskStatusBacklog, task.TaskStatusFailed},
		{task.TaskStatusQueued, task.TaskStatusFailed},
		{task.TaskStatusDoing, task.TaskStatusFailed},
	}

	for _, pair := range validPairs {
		if err := svc.ValidateTaskTransition(pair[0], pair[1]); err != nil {
			t.Errorf("expected transition %s -> %s to be valid, got error: %v", pair[0], pair[1], err)
		}
	}

	// Illegal transitions guarded by state machine
	invalidPairs := [][2]string{
		{task.TaskStatusBacklog, task.TaskStatusDoing}, // cannot skip queued/approved
		{task.TaskStatusBacklog, task.TaskStatusDone},  // cannot skip to done
		{task.TaskStatusDone, task.TaskStatusDoing},    // terminal cannot transition
		{task.TaskStatusDone, task.TaskStatusQueued},   // terminal cannot transition
		{task.TaskStatusFailed, task.TaskStatusDoing},  // terminal cannot transition
		{"unknown_status", task.TaskStatusDoing},       // invalid source
		{task.TaskStatusDoing, "unknown_status"},       // invalid destination
	}

	for _, pair := range invalidPairs {
		if err := svc.ValidateTaskTransition(pair[0], pair[1]); err == nil {
			t.Errorf("expected transition %s -> %s to be rejected as illegal, got nil error", pair[0], pair[1])
		}
	}
}

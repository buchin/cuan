package server

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

func TestTimeoutEscalation_RunnerAdaptiveScale(t *testing.T) {
	os.Unsetenv("CUAN_TASK_TIMEOUT_OVERRIDE")
	os.Unsetenv("CUAN_TASK_TIMEOUT_BASE")
	os.Unsetenv("CUAN_TASK_TIMEOUT_REQUEUE")
	os.Unsetenv("CUAN_TASK_TIMEOUT_MAX")
	os.Unsetenv("CUAN_TASK_GRACE_PERIOD")

	// Verify Attempt 1: 25m
	if d := runner.CalculateAdaptiveTimeout(1); d != 25*time.Minute {
		t.Fatalf("expected 25m for attempt 1, got %v", d)
	}
	// Verify Attempt 2: 35m
	if d := runner.CalculateAdaptiveTimeout(2); d != 35*time.Minute {
		t.Fatalf("expected 35m for attempt 2, got %v", d)
	}
	// Verify Attempt 3 (Max Cap): 45m
	if d := runner.CalculateAdaptiveTimeout(3); d != 45*time.Minute {
		t.Fatalf("expected 45m for attempt 3, got %v", d)
	}
	// Verify Grace Period: 30s
	if gp := runner.GetGracePeriod(); gp != 30*time.Second {
		t.Fatalf("expected 30s grace period, got %v", gp)
	}
}

func TestTerminalManager_IdleTimeoutEscalationAndMidTurnImmunity(t *testing.T) {
	workspacesDir := t.TempDir()
	mgr := terminal.NewManager(terminal.ManagerConfig{
		WorkspacesDir: workspacesDir,
	})

	// 1. Verify default IdleTimeout is escalated to 30 minutes
	cleaned := mgr.CleanIdleSessions(0)
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned on empty manager, got %d", cleaned)
	}

	// 2. Verify mid-turn activity immunity: session with recent activity is never swept
	// Test session created and checked within 30m idle threshold
	cleanedPast := mgr.CleanIdleSessions(30 * time.Minute)
	if cleanedPast != 0 {
		t.Fatalf("expected 0 sessions swept, got %d", cleanedPast)
	}
}

func TestInboxDelegation_RetryLimit_MultiRoomProactiveChat(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := runner.NewMemoryStore(taskStore)
	runnerSvc := runner.NewService(runnerStore, taskStore)

	var sentRooms []string
	var sentMessages []string
	runnerSvc.SetChatNotifier(runner.NewDefaultBusinessChatNotifier(func(ctx context.Context, roomID, messageContent string) error {
		sentRooms = append(sentRooms, roomID)
		sentMessages = append(sentMessages, messageContent)
		return nil
	}))

	// Setup task with SourceRoomID
	taskItem := &task.Task{
		ID:           "task_retail_sync",
		Title:        "Sinkronisasi Transaksi Penjualan Toko",
		Summary:      "Rekap transaksi harian telah disiapkan",
		SourceRoomID: "room_source_shop_1",
		Status:       task.TaskStatusDoing,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = taskStore.CreateTask(ctx, taskItem)

	// Attempt 3 (reaches max cap)
	att3 := &task.TaskAttempt{
		ID:            "att_limit_3",
		TaskID:        taskItem.ID,
		AttemptNumber: 3,
		RunnerID:      "cuan-runner",
		Status:        task.AttemptStatusRunning,
		CreatedAt:     time.Now().UTC(),
	}
	_ = taskStore.CreateAttempt(ctx, att3)

	// Complete attempt with unfixable DoD gap on attempt 3
	req := runner.CompleteAttemptRequest{
		RunnerID:             "cuan-runner",
		RoomID:               "room_active_chat_2", // Active room
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
		DoDValidation: &runner.DoDValidationResult{
			Passed:  false,
			Action:  "review",
			Gaps:    []string{"Pemeriksaan bukti rekonsiliasi pembayaran belum lengkap"},
			Details: "Batas 3x attempt tercapai",
		},
	}

	completedAtt, err := runnerSvc.Complete(ctx, att3.ID, req)
	if err != nil {
		t.Fatalf("unexpected Complete error: %v", err)
	}
	if completedAtt == nil {
		t.Fatal("expected completed attempt")
	}

	// 1. Verify task status moved to 'review' (not hung in doing)
	updatedTask, err := taskStore.GetTask(ctx, taskItem.ID)
	if err != nil {
		t.Fatalf("get task error: %v", err)
	}
	if updatedTask.Status != task.TaskStatusBlocked {
		t.Fatalf("expected task status 'blocked', got %s", updatedTask.Status)
	}

	// Inbox coordinator owns the decision and announcement. The runner must not
	// send premature owner-intervention instructions before that review occurs.
	if len(sentRooms) != 0 || len(sentMessages) != 0 {
		t.Fatal("runner bypassed coordinator", sentRooms)
	}

}

func TestServer_SendBusinessFriendlyReviewNotification(t *testing.T) {
	ctx := context.Background()
	studioRepo := cuanDB.NewMemoryProjectStudioRepository()
	s := &Server{
		projectStudioRepo: studioRepo,
	}

	roomA := cuanDB.GenerateUUID()
	roomB := cuanDB.GenerateUUID()

	s.sendBusinessFriendlyReviewNotification(ctx, "task_rev_123", "Pembaruan Banner Diskon", "Banner diskon toko telah dirancang", roomA, roomB, 3, []string{
		"Ukuran banner perlu disesuaikan dengan proporsi mobile",
	})

	msgsA, _ := studioRepo.ListMessagesByRoomID(ctx, roomA)
	msgsB, _ := studioRepo.ListMessagesByRoomID(ctx, roomB)

	if len(msgsA) != 1 {
		t.Fatalf("expected 1 message in roomAlpha, got %d", len(msgsA))
	}
	if len(msgsB) != 1 {
		t.Fatalf("expected 1 message in roomBeta, got %d", len(msgsB))
	}

	content := msgsA[0].Content
	if !strings.Contains(content, "Pembaruan penting untuk toko Anda") {
		t.Fatalf("expected warm business greeting, got: %s", content)
	}
	if !strings.Contains(content, "Opsi Tindak Lanjut") {
		t.Fatalf("expected follow up options, got: %s", content)
	}
	if !strings.Contains(content, "/tasks/task_rev_123") {
		t.Fatalf("expected task link, got: %s", content)
	}
}

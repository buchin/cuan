package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestParseInboxTaskCancelAndReopen(t *testing.T) {
	// Cancel
	validCancel := "Saya akan membatalkan tugas tersebut.\n```cuan-task-cancel\n{\"task_id\":\"task_abc\",\"reason\":\"Tidak diperlukan lagi\"}\n```"
	clean, cancelAct, err := parseInboxTaskCancel(validCancel)
	if err != nil || cancelAct == nil {
		t.Fatalf("expected valid cancel parse, got: %v", err)
	}
	if clean != "Saya akan membatalkan tugas tersebut." {
		t.Fatalf("unexpected clean text: %s", clean)
	}
	if cancelAct.TaskID != "task_abc" || cancelAct.Reason != "Tidak diperlukan lagi" {
		t.Fatalf("unexpected action: %+v", cancelAct)
	}

	// Reopen
	validReopen := "Saya buka kembali tugas ini untuk perbaikan.\n```cuan-task-reopen\n{\"task_id\":\"task_xyz\",\"reason\":\"Coba lagi setelah migrasi\"}\n```"
	cleanReopen, reopenAct, err := parseInboxTaskReopen(validReopen)
	if err != nil || reopenAct == nil {
		t.Fatalf("expected valid reopen parse, got: %v", err)
	}
	if cleanReopen != "Saya buka kembali tugas ini untuk perbaikan." {
		t.Fatalf("unexpected clean text: %s", cleanReopen)
	}
	if reopenAct.TaskID != "task_xyz" || reopenAct.Reason != "Coba lagi setelah migrasi" {
		t.Fatalf("unexpected action: %+v", reopenAct)
	}

	// Malformed / Empty task_id
	bad := "```cuan-task-cancel\n{\"task_id\":\"\"}\n```"
	if _, _, err := parseInboxTaskCancel(bad); err == nil {
		t.Fatal("expected error for empty task_id")
	}

	badReopen := "```cuan-task-reopen\n{\"task_id\":\"\"}\n```"
	if _, _, err := parseInboxTaskReopen(badReopen); err == nil {
		t.Fatal("expected error for empty task_id in reopen")
	}
}

func TestApplyInboxTaskCancelAndReopen(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestChatServer(t)
	s.taskSvc = task.NewService(task.NewMemoryStore(), nil)

	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID: "proj_1",
		Title:     "Feature Database Migration",
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	room := db.GenerateUUID()
	msgID := db.GenerateUUID()
	q := db.InboxRequest{
		RoomID:    room,
		MessageID: msgID,
		OwnerID:   "owner_test",
		CreatedAt: time.Now(),
	}

	// Test Cancel
	cancelContent := "Membatalkan task database migration.\n```cuan-task-cancel\n{\"task_id\":\"" + created.ID + "\",\"reason\":\"Superseded by v2\"}\n```"
	cancelReply := s.applyInboxTaskCancel(ctx, q, cancelContent)
	if !strings.Contains(cancelReply, "has been cancelled") {
		t.Fatalf("expected cancellation message, got: %s", cancelReply)
	}

	detail, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("get task detail failed: %v", err)
	}
	if detail.Status != task.TaskStatusBlocked {
		t.Fatalf("expected status blocked after cancel, got: %s", detail.Status)
	}

	// Test Reopen
	reopenContent := "Membuka kembali task database migration.\n```cuan-task-reopen\n{\"task_id\":\"" + created.ID + "\",\"reason\":\"Perbaikan pipeline database\"}\n```"
	reopenReply := s.applyInboxTaskReopen(ctx, q, reopenContent)
	if !strings.Contains(reopenReply, "has been reopened and moved to Backlog") {
		t.Fatalf("expected reopen message, got: %s", reopenReply)
	}

	detailAfterReopen, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("get task detail after reopen failed: %v", err)
	}
	if detailAfterReopen.Status != task.TaskStatusBacklog {
		t.Fatalf("expected status backlog after reopen, got: %s", detailAfterReopen.Status)
	}
}

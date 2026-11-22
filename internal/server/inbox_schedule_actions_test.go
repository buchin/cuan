package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/scheduler"
)

func TestParseInboxSchedule(t *testing.T) {
	// 1. Valid create
	validCreate := "Saya jadwalkan pengecekan harian.\n```cuan-schedule\n{\"action\":\"create\",\"title\":\"Daily Routine\",\"schedule_type\":\"daily\",\"time_of_day\":\"03:00\",\"project_id\":\"proj_123\"}\n```"
	clean, act, err := parseInboxSchedule(validCreate)
	if err != nil || act == nil {
		t.Fatalf("expected valid schedule parse, got error: %v", err)
	}
	if clean != "Saya jadwalkan pengecekan harian." {
		t.Fatalf("unexpected clean text: %s", clean)
	}
	if act.Action != "create" || act.Title != "Daily Routine" || act.ScheduleType != "daily" || act.TimeOfDay != "03:00" || act.ProjectID != "proj_123" {
		t.Fatalf("unexpected action parsed: %+v", act)
	}

	// 2. Valid update
	validUpdate := "Jadwal telah disesuaikan.\n```cuan-schedule\n{\"action\":\"update\",\"id\":\"sch_123\",\"title\":\"Updated Routine\",\"time_of_day\":\"05:00\"}\n```"
	clean, act, err = parseInboxSchedule(validUpdate)
	if err != nil || act == nil {
		t.Fatalf("expected valid schedule update parse, got: %v", err)
	}
	if act.Action != "update" || act.ID != "sch_123" || act.TimeOfDay != "05:00" {
		t.Fatalf("unexpected update action parsed: %+v", act)
	}

	// 3. Valid delete
	validDelete := "Jadwal akan dihapus.\n```cuan-schedule\n{\"action\":\"delete\",\"id\":\"sch_123\"}\n```"
	clean, act, err = parseInboxSchedule(validDelete)
	if err != nil || act == nil {
		t.Fatalf("expected valid schedule delete parse, got: %v", err)
	}
	if act.Action != "delete" || act.ID != "sch_123" {
		t.Fatalf("unexpected delete action parsed: %+v", act)
	}

	// 4. Valid toggle
	enabled := false
	validToggle := "Jadwal dinonaktifkan.\n```cuan-schedule\n{\"action\":\"toggle\",\"id\":\"sch_123\",\"enabled\":false}\n```"
	clean, act, err = parseInboxSchedule(validToggle)
	if err != nil || act == nil {
		t.Fatalf("expected valid schedule toggle parse, got: %v", err)
	}
	if act.Action != "toggle" || act.ID != "sch_123" || act.Enabled == nil || *act.Enabled != enabled {
		t.Fatalf("unexpected toggle action parsed: %+v", act)
	}

	// 5. Invalid: missing title on create
	badCreate := "```cuan-schedule\n{\"action\":\"create\",\"title\":\"\"}\n```"
	if _, _, err := parseInboxSchedule(badCreate); err == nil {
		t.Fatal("expected error for empty title on create")
	}

	// 6. Invalid: missing id on update/delete
	badDelete := "```cuan-schedule\n{\"action\":\"delete\",\"id\":\"\"}\n```"
	if _, _, err := parseInboxSchedule(badDelete); err == nil {
		t.Fatal("expected error for empty id on delete")
	}
}

func TestApplyInboxSchedule(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestChatServer(t)
	taskMgr := scheduler.NewTaskManager(filepath.Join(t.TempDir(), "tasks.json"))
	s.taskManager = taskMgr

	q := db.InboxRequest{
		RoomID:    "room-test",
		OwnerID:   "owner-1",
		MessageID: "msg-1",
	}

	// 1. Create schedule
	createContent := "Saya jadwalkan rutinitas ini.\n```cuan-schedule\n{\"action\":\"create\",\"title\":\"Sip Routine\",\"prompt\":\"Run checks\",\"schedule_type\":\"daily\",\"time_of_day\":\"04:00\",\"project_id\":\"proj_test\"}\n```"
	createReply := s.applyInboxSchedule(ctx, q, createContent)
	if !strings.Contains(createReply, "Jadwal Rutin Aktif") || !strings.Contains(createReply, "Sip Routine") || !strings.Contains(createReply, "proj_test") {
		t.Fatalf("unexpected create reply: %s", createReply)
	}

	tasks := taskMgr.ListTasks()
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task in manager, got: %v", tasks)
	}
	taskID := tasks[0].ID

	// 2. Update schedule
	updateContent := "Perbarui jadwal rutin.\n```cuan-schedule\n{\"action\":\"update\",\"id\":\"" + taskID + "\",\"title\":\"Updated Sip Routine\",\"time_of_day\":\"06:00\"}\n```"
	updateReply := s.applyInboxSchedule(ctx, q, updateContent)
	if !strings.Contains(updateReply, "Jadwal Rutin Diperbarui") || !strings.Contains(updateReply, "Updated Sip Routine") {
		t.Fatalf("unexpected update reply: %s", updateReply)
	}

	updated, ok := taskMgr.GetTask(taskID)
	if !ok || updated.Title != "Updated Sip Routine" || updated.TimeOfDay != "06:00" {
		t.Fatalf("unexpected task after update: %+v, found: %v", updated, ok)
	}

	// 3. Toggle schedule
	toggleContent := "Pause jadwal rutin.\n```cuan-schedule\n{\"action\":\"toggle\",\"id\":\"" + taskID + "\",\"enabled\":false}\n```"
	toggleReply := s.applyInboxSchedule(ctx, q, toggleContent)
	if !strings.Contains(toggleReply, "dijeda") {
		t.Fatalf("unexpected toggle reply: %s", toggleReply)
	}

	toggled, ok := taskMgr.GetTask(taskID)
	if !ok || toggled.Enabled {
		t.Fatalf("expected task to be paused, got: %+v, found: %v", toggled, ok)
	}

	// 4. Delete schedule
	deleteContent := "Hapus jadwal rutin.\n```cuan-schedule\n{\"action\":\"delete\",\"id\":\"" + taskID + "\"}\n```"
	deleteReply := s.applyInboxSchedule(ctx, q, deleteContent)
	if !strings.Contains(deleteReply, "Jadwal Rutin Dihapus") {
		t.Fatalf("unexpected delete reply: %s", deleteReply)
	}

	remaining := taskMgr.ListTasks()
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining tasks, got: %d", len(remaining))
	}
}

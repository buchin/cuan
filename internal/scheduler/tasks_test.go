package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskManager_Lifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-task-manager-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "scheduled_tasks.json")

	var executedTaskID string
	tm := NewTaskManager(storagePath, func(ctx context.Context, task *ScheduledTask) error {
		executedTaskID = task.ID
		return nil
	})

	// 1. Create task
	task, err := tm.CreateTask(&ScheduledTask{
		Title:        "Rekap Mingguan Penjualan",
		Prompt:       "Buat rekap leads dan penjualan dari tebar.app",
		ScheduleType: ScheduleWeekly,
		DayOfWeek:    1, // Monday
		TimeOfDay:    "09:00",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if task.ID == "" || task.NextRunAt == nil {
		t.Fatalf("expected ID and NextRunAt to be set: %+v", task)
	}

	// 2. List tasks
	list := tm.ListTasks()
	if len(list) != 1 {
		t.Fatalf("expected 1 task, got %d", len(list))
	}

	// 3. Trigger task
	if err := tm.TriggerTask(context.Background(), task.ID); err != nil {
		t.Fatalf("TriggerTask failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if executedTaskID != task.ID {
		t.Errorf("expected executedTaskID = %s, got %s", task.ID, executedTaskID)
	}

	// 4. Toggle task
	f := false
	toggled, err := tm.ToggleTask(task.ID, &f)
	if err != nil || toggled.Enabled != false {
		t.Fatalf("expected toggle to false, got %v / %v", toggled, err)
	}

	// 5. Verify persistence on new instance
	tm2 := NewTaskManager(storagePath)
	list2 := tm2.ListTasks()
	if len(list2) != 1 || list2[0].Title != "Rekap Mingguan Penjualan" {
		t.Fatalf("persistence check failed: %+v", list2)
	}

	// 6. Delete task
	if err := tm.DeleteTask(task.ID); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}
	if len(tm.ListTasks()) != 0 {
		t.Fatalf("expected 0 tasks after deletion")
	}
}

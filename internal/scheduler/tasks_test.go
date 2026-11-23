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

	// 4b. Update task
	updated, err := tm.UpdateTask(task.ID, &ScheduledTask{
		Title:     "Rekap Harian Penjualan",
		Prompt:    "Cek ringkasan penjualan harian",
		ProjectID: "proj_custom",
	})
	if err != nil || updated.Title != "Rekap Harian Penjualan" || updated.ProjectID != "proj_custom" {
		t.Fatalf("UpdateTask failed: %v, updated: %+v", err, updated)
	}

	// 5. Verify persistence on new instance
	tm2 := NewTaskManager(storagePath)
	list2 := tm2.ListTasks()
	if len(list2) != 1 || list2[0].Title != "Rekap Harian Penjualan" || list2[0].ProjectID != "proj_custom" {
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

func TestTaskManager_AutoRunDueTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-task-due-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "scheduled_tasks.json")

	executed := make(chan string, 1)
	tm := NewTaskManager(storagePath, func(ctx context.Context, task *ScheduledTask) error {
		executed <- task.ID
		return nil
	})

	// Create task
	task, err := tm.CreateTask(&ScheduledTask{
		Title:        "Rutinitas Pembersihan",
		Prompt:       "Bersihkan cache kadaluarsa",
		ScheduleType: ScheduleDaily,
		TimeOfDay:    "03:00",
	})
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Force NextRunAt to the past (e.g. 10 minutes ago)
	past := time.Now().Add(-10 * time.Minute)
	tm.mu.Lock()
	tm.tasks[task.ID].NextRunAt = &past
	tm.mu.Unlock()

	// checkAndRunDueTasks should detect and trigger it
	ctx := context.Background()
	tm.checkAndRunDueTasks(ctx, time.Now())

	select {
	case id := <-executed:
		if id != task.ID {
			t.Errorf("expected executed task %s, got %s", task.ID, id)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for due task execution")
	}

	// Verify NextRunAt was updated to the future
	updated, found := tm.GetTask(task.ID)
	if !found {
		t.Fatal("task not found")
	}
	if updated.NextRunAt == nil || !updated.NextRunAt.After(time.Now()) {
		t.Fatalf("expected NextRunAt to be in the future, got %v", updated.NextRunAt)
	}
	if updated.LastRunAt == nil {
		t.Fatal("expected LastRunAt to be set")
	}
}

func TestTaskManager_StartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-task-start-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "scheduled_tasks.json")
	tm := NewTaskManager(storagePath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tm.Start(ctx)
	if !tm.running {
		t.Error("expected tm.running to be true")
	}

	// Starting again while running should be a no-op
	tm.Start(ctx)
	if !tm.running {
		t.Error("expected tm.running to still be true")
	}

	tm.Stop()
	if tm.running {
		t.Error("expected tm.running to be false after Stop")
	}

	// Stopping again should be a no-op
	tm.Stop()
}

func TestCalculateTaskNextRun_Timezone(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("LoadLocation failed: %v", err)
	}

	task := &ScheduledTask{
		ScheduleType: ScheduleDaily,
		TimeOfDay:    "21:00",
		Timezone:     "Asia/Jakarta",
	}

	// Reference time at 14:00 Jakarta (07:00 UTC)
	ref := time.Date(2026, 11, 23, 14, 0, 0, 0, jakarta)
	next := calculateTaskNextRun(task, ref)

	if next.Hour() != 21 || next.Minute() != 0 {
		t.Errorf("expected next run at 21:00 in Jakarta, got %d:%d", next.Hour(), next.Minute())
	}
	if next.Location().String() != "Asia/Jakarta" {
		t.Errorf("expected location Asia/Jakarta, got %v", next.Location())
	}
	if next.Day() != 23 {
		t.Errorf("expected same day 23, got %d", next.Day())
	}

	// Reference time past 21:00 (e.g. 21:30 Jakarta) -> should advance to next day
	refPast := time.Date(2026, 11, 23, 21, 30, 0, 0, jakarta)
	nextDay := calculateTaskNextRun(task, refPast)
	if nextDay.Day() != 24 || nextDay.Hour() != 21 {
		t.Errorf("expected next day 24 at 21:00, got %d at %d:%d", nextDay.Day(), nextDay.Hour(), nextDay.Minute())
	}
}

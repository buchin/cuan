package task

import (
	"context"
	"testing"
	"time"
)

func TestLatestProgressMemoryContract(t *testing.T) {
	latestProgressAttemptContract(t, NewMemoryStore(), "default")
}

func latestProgressAttemptContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	svc := NewService(store, nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{ProjectID: project, Title: "Progress attempt fixture", Instructions: "Audit current state"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-time.Hour)
	old := &TaskAttempt{ID: created.ID + "_old", TaskID: created.ID, RunnerID: "fixture", AgentName: "fixture", Status: AttemptStatusRunning, CreatedAt: now}
	if err := store.CreateAttempt(ctx, old); err != nil {
		t.Fatal(err)
	}
	add := func(id, attempt, message string, at time.Time) {
		t.Helper()
		if err := store.CreateActivity(ctx, &TaskActivity{ID: created.ID + id, TaskID: created.ID, AttemptID: &attempt, ActivityType: "agent_report", NewStatus: "progress", Message: message, CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want string) {
		t.Helper()
		d, err := svc.GetTaskDetail(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if d.LatestProgress != want {
			t.Fatalf("detail progress = %q; want %q", d.LatestProgress, want)
		}
		list, err := svc.ListTasks(ctx, TaskFilter{})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range list {
			if item.ID == created.ID {
				if item.LatestProgress != want {
					t.Fatalf("list progress = %q; want %q", item.LatestProgress, want)
				}
				return
			}
		}
		t.Fatal("task absent from list")
	}
	add("_progress_old", old.ID, "Audit lama selesai", now.Add(time.Minute))
	check("Audit lama selesai")
	current := &TaskAttempt{ID: created.ID + "_new", TaskID: created.ID, RunnerID: "fixture", AgentName: "fixture", Status: AttemptStatusQueued, CreatedAt: now.Add(2 * time.Minute)}
	if err := store.CreateAttempt(ctx, current); err != nil {
		t.Fatal(err)
	}
	// Even a still-running older attempt or a delayed old report cannot supply
	// progress for the new queued attempt.
	if err := store.AppendLogs(ctx, old.ID, []LogRecord{{Stream: "stdout", Line: "Memeriksa hasil attempt lama"}}, 100); err != nil {
		t.Fatal(err)
	}
	add("_late_old", old.ID, "Laporan lama terlambat", now.Add(5*time.Minute))
	check("")
	add("_stage_only", current.ID, StagedReportProgress+"\n\nSaved report", now.Add(150*time.Second))
	check("")
	current.Status = AttemptStatusRunning
	if err := store.UpdateAttempt(ctx, current); err != nil {
		t.Fatal(err)
	}
	var logs []LogRecord
	logs = append(logs, LogRecord{Stream: "stdout", Line: "Memeriksa sumber data awal"})
	for i := 0; i < 40; i++ {
		logs = append(logs, LogRecord{Stream: "stdout", Line: ""})
	}
	logs = append(logs, LogRecord{Stream: "stdout", Line: "Memeriksa hasil audit sebelumnya"}, LogRecord{Stream: "stdout", Line: "Memeriksa hasil audit terbaru"}, LogRecord{Stream: "stderr", Line: "Jangan tampilkan stderr sebagai progress"})
	if err := store.AppendLogs(ctx, current.ID, logs, 100); err != nil {
		t.Fatal(err)
	}
	check("") // Terminal output is diagnostic data, not a published progress report.
	add("_progress_new", current.ID, "Menyiapkan laporan terbaru", now.Add(3*time.Minute))
	check("Menyiapkan laporan terbaru")
	add("_stage_later", current.ID, StagedReportProgress+"\n\nSaved report", now.Add(4*time.Minute))
	check("Menyiapkan laporan terbaru")
	activities, err := store.ListActivities(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, activity := range activities {
		if activity.ID == created.ID+"_late_old" {
			found = true
		}
	}
	if !found {
		t.Fatal("historical report was removed")
	}
	// A new terminal attempt still must not fall back to an older running one.
	current.Status = AttemptStatusCanceled
	if err := store.UpdateAttempt(ctx, current); err != nil {
		t.Fatal(err)
	}
	check("Menyiapkan laporan terbaru")
	terminal := &TaskAttempt{ID: created.ID + "_terminal", TaskID: created.ID, RunnerID: "fixture", AgentName: "fixture", Status: AttemptStatusCanceled, CreatedAt: now.Add(6 * time.Minute)}
	if err := store.CreateAttempt(ctx, terminal); err != nil {
		t.Fatal(err)
	}
	check("")
}

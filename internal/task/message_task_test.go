package task

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMessageTaskIdempotencyAndLocalAttempt(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, e := svc.CreateTaskFromMessage(ctx, "owner", "fixture-message", CreateTaskRequest{ProjectID: "fixture-project", Title: "Do work", Instructions: "fixture"})
			if e != nil || got.ID != "msg_fixture-message" {
				t.Error(got, e)
			}
		}()
	}
	wg.Wait()
	all, _ := svc.ListTasks(ctx, TaskFilter{})
	if len(all) != 1 {
		t.Fatal("duplicated task", len(all))
	}
	a, e := svc.CreateAttempt(ctx, all[0].ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.CreateAttempt(ctx, all[0].ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal"}); !errors.Is(e, ErrTaskActive) {
		t.Fatal("duplicate local attempt", e)
	}
	if _, e = svc.UpdateAttemptStatus(ctx, a.ID, "runner", UpdateAttemptRequest{Status: "running"}); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.CancelTask(ctx, all[0].ID, "owner"); e != nil {
		t.Fatal(e)
	}
	zero := 0
	got, e := svc.UpdateAttemptStatus(ctx, a.ID, "runner", UpdateAttemptRequest{Status: "succeeded", ExitCode: &zero})
	if e != nil || got.Status != "canceled" {
		t.Fatal("late completion replaced cancel", got, e)
	}
	a, e = svc.CreateAttempt(ctx, all[0].ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal"})
	if e != nil {
		t.Fatal(e)
	}
	past := time.Now().Add(-time.Second)
	a.LeaseExpiresAt = &past
	store.UpdateAttempt(ctx, a)
	if e = svc.RecoverLocalAttempts(ctx); e != nil {
		t.Fatal(e)
	}
	detail, _ := svc.GetTaskDetail(ctx, all[0].ID)
	if detail.LatestAttempt.Status != "interrupted" || detail.Status != "blocked" {
		t.Fatal("unknown outcome not exposed", detail)
	}
}

func TestMessageTaskDelayedExitDoesNotOverwriteNewAttempt(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, _ := svc.CreateTask(ctx, "fixture", CreateTaskRequest{Title: "Delayed callback"})
	old, _ := svc.CreateAttempt(ctx, created.ID, "fixture", CreateAttemptRequest{RunnerID: "local_terminal"})
	svc.UpdateAttemptStatus(ctx, old.ID, "fixture", UpdateAttemptRequest{Status: "succeeded"})
	newer, e := svc.CreateAttempt(ctx, created.ID, "fixture", CreateAttemptRequest{RunnerID: "local_terminal"})
	if e != nil {
		t.Fatal(e)
	}
	doing := TaskStatusDoing
	svc.UpdateTask(ctx, created.ID, "fixture", UpdateTaskRequest{Status: &doing})
	if e = svc.ReconcileLocalTask(ctx, created.ID, old.ID); e != nil {
		t.Fatal(e)
	}
	detail, _ := svc.GetTaskDetail(ctx, created.ID)
	if detail.Status != "doing" || detail.LatestAttempt.ID != newer.ID {
		t.Fatal("stale exit overwrote active task", detail)
	}
	// An independent caller with stale attempt state cannot revive a terminal row.
	old.Status = "running"
	if e = store.UpdateAttempt(ctx, old); !errors.Is(e, ErrAttemptFinal) {
		t.Fatal("terminal attempt revived", e)
	}
}

func TestLocalLeaseHeartbeatAndRecovery(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Heartbeat"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateAttemptStatus(ctx, a.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}
	a, _ = store.GetAttempt(ctx, a.ID)
	past := time.Now().Add(-time.Minute)
	a.LeaseExpiresAt = &past
	if err = store.UpdateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = svc.RenewLocalAttemptLease(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = svc.RecoverLocalAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	live, _ := store.GetAttempt(ctx, a.ID)
	if live.Status != AttemptStatusRunning || !live.LeaseExpiresAt.After(time.Now()) {
		t.Fatal("live lease not renewed")
	}
	live.LeaseExpiresAt = &past
	if err = store.UpdateAttempt(ctx, live); err != nil {
		t.Fatal(err)
	}
	if err = svc.RecoverLocalAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	if err = svc.RenewLocalAttemptLease(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	dead, _ := store.GetAttempt(ctx, a.ID)
	if dead.Status != AttemptStatusInterrupted {
		t.Fatal("dead attempt revived")
	}
}

func TestRecoverStartupLocalAttempts(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)

	created, err := svc.CreateTask(ctx, "author", CreateTaskRequest{
		ProjectID:    "fixture-project",
		Title:        "Task 1",
		Instructions: "Do work",
		DoDChecklist: []DoDItem{{Text: "Item 1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	doing := TaskStatusDoing
	if _, err = svc.UpdateTask(ctx, created.ID, "author", UpdateTaskRequest{Status: &doing}); err != nil {
		t.Fatal(err)
	}
	a, err := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateAttemptStatus(ctx, a.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}

	// Lease is still far in the future
	future := time.Now().Add(10 * time.Minute)
	a, _ = store.GetAttempt(ctx, a.ID)
	a.LeaseExpiresAt = &future
	if err = store.UpdateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}

	// Normal recovery would skip because lease is in the future
	if err = svc.RecoverLocalAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	stillRunning, _ := store.GetAttempt(ctx, a.ID)
	if stillRunning.Status != AttemptStatusRunning {
		t.Fatal("normal recovery should not interrupt valid lease")
	}

	// Startup recovery immediately interrupts orphan local attempt
	if err = svc.RecoverStartupLocalAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	interrupted, _ := store.GetAttempt(ctx, a.ID)
	if interrupted.Status != AttemptStatusInterrupted {
		t.Fatalf("expected interrupted status, got %s", interrupted.Status)
	}
	if interrupted.ErrorMessage != "Execution was interrupted during server restart." {
		t.Fatalf("unexpected error message: %s", interrupted.ErrorMessage)
	}

	// Verify task is reconciled from doing
	updatedTask, _ := svc.GetTaskDetail(ctx, created.ID)
	if updatedTask.Status == TaskStatusDoing {
		t.Fatalf("task should not remain doing after startup recovery, got %s", updatedTask.Status)
	}
}

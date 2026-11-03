package runner

import (
	"context"
	"github.com/buchin/cuan/internal/task"
	"testing"
	"time"
)

func TestExpiredAcceptedAttemptReleasesRunnerLaneWithoutReopeningTask(t *testing.T) {
	ctx := context.Background()
	tasks := task.NewMemoryStore()
	store := NewMemoryStore(tasks)
	now := time.Now().UTC()
	expired := now.Add(-time.Minute)
	accepted := &task.Task{ID: "accepted", Title: "Accepted output", Status: task.TaskStatusDone, ExecutionCategory: task.ExecutionCategoryCode, Summary: "Delivered", CompleteNote: "Verified", CreatedAt: now, UpdatedAt: now}
	tasks.CreateTask(ctx, accepted)
	tasks.CreateAttempt(ctx, &task.TaskAttempt{ID: "stale", TaskID: accepted.ID, RunnerID: "runner", Status: task.AttemptStatusRunning, LeaseExpiresAt: &expired, CreatedAt: now})
	queued := &task.Task{ID: "next", Title: "Next approved work", Status: task.TaskStatusQueued, ExecutionCategory: task.ExecutionCategoryCode, CreatedAt: now, UpdatedAt: now}
	tasks.CreateTask(ctx, queued)
	tasks.CreateAttempt(ctx, &task.TaskAttempt{ID: "next-att", TaskID: queued.ID, Status: task.AttemptStatusQueued, CreatedAt: now, AttemptNumber: 1})
	before, _, err := store.ClaimNextQueuedAttempt(ctx, "runner", nil, time.Minute)
	if err != nil || before != nil {
		t.Fatal("stale lane fixture incorrect", err)
	}
	retired, err := store.MarkInterruptedLeases(ctx)
	if err != nil || len(retired) != 1 {
		t.Fatal("cleanup failed", err)
	}
	next, _, err := store.ClaimNextQueuedAttempt(ctx, "runner", nil, time.Minute)
	if err != nil || next == nil || next.ID != "next-att" {
		t.Fatal("lane not released", err)
	}
	preserved, _ := tasks.GetTask(ctx, accepted.ID)
	if preserved.Status != task.TaskStatusDone || preserved.CompleteNote != "Verified" {
		t.Fatal("accepted task reopened")
	}
}

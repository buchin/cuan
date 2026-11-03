package runner

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestRunnerService_Complete_InfraFaultBlocksWithoutReviewResult(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	svc := NewService(runnerStore, taskStore)

	// Create task
	taskItem := &task.Task{
		ID:           "task_infra_complete",
		Title:        "Sinkronisasi Data Produk",
		Instructions: "Lakukan sinkronisasi produk",
		Status:       task.TaskStatusDoing,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := taskStore.CreateTask(ctx, taskItem); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create running attempt
	att := &task.TaskAttempt{
		ID:            "att_infra_complete_1",
		TaskID:        taskItem.ID,
		RunnerID:      "runner_1",
		Status:        task.AttemptStatusRunning,
		AttemptNumber: 1,
		CreatedAt:     time.Now().UTC(),
	}
	if err := taskStore.CreateAttempt(ctx, att); err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	// Complete with infra fault: heartbeat lost / lease expired (exit 137)
	completedAtt, err := svc.Complete(ctx, att.ID, CompleteAttemptRequest{
		RunnerID:     "runner_1",
		Status:       task.AttemptStatusFailed,
		ExitCode:     137,
		ErrorMessage: "Runner heartbeat lost and lease expired; marked interrupted and recoverable without auto-rerun.",
	})
	if err != nil {
		t.Fatalf("svc.Complete failed: %v", err)
	}

	// Attempt status must be normalized to interrupted
	if completedAtt.Status != task.AttemptStatusInterrupted {
		t.Fatalf("expected attempt status 'interrupted', got '%s'", completedAtt.Status)
	}

	// An interrupted run has no reviewable result.
	updatedTask, err := taskStore.GetTask(ctx, taskItem.ID)
	if err != nil {
		t.Fatalf("failed to get updated task: %v", err)
	}
	if updatedTask.Status != task.TaskStatusBlocked {
		t.Fatalf("expected task status 'blocked' for infra fault, got '%s'", updatedTask.Status)
	}
}

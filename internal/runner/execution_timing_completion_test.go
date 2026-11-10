package runner

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestCompletionPersistsTimingWithoutChangingVerificationResult(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "measured", true: "invalid telemetry"}[invalid], func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			start := time.Now().UTC().Add(-10 * time.Second)
			item := &task.Task{ID: "work", Status: task.TaskStatusDoing, CreatedAt: start, UpdatedAt: start}
			a := &task.TaskAttempt{ID: "attempt", TaskID: item.ID, RunnerID: "native", Status: task.AttemptStatusRunning, StartedAt: &start, CreatedAt: start}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			if err := store.CreateAttempt(ctx, a); err != nil {
				t.Fatal(err)
			}
			agent, checks := int64(2000), int64(4000)
			if invalid {
				checks = -1
			}
			service := NewService(NewMemoryStore(store), store, nil)
			_, err := service.Complete(ctx, a.ID, CompleteAttemptRequest{RunnerID: a.RunnerID, Status: task.AttemptStatusSucceeded, TestSummary: "Project verification: test.sh --fast (exit 0)", ExecutionTiming: &task.ExecutionTiming{AgentElapsedMS: &agent, ProjectChecksElapsedMS: &checks}})
			if err != nil {
				t.Fatal("optional telemetry blocked completion", err)
			}
			d, err := task.NewService(store, nil).GetTaskDetail(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			got := task.AttemptExecutionTiming(d)
			if invalid && got != nil {
				t.Fatal("invalid metrics saved as evidence")
			}
			if !invalid && (got == nil || *got.AgentElapsedMS != agent || *got.ProjectChecksElapsedMS != checks) {
				t.Fatal("completion lost measured stages", got)
			}
			if !task.ProjectChecksPassed(d) {
				t.Fatal("metrics changed native check result")
			}
		})
	}
}

package server

import (
	"github.com/buchin/cuan/internal/task"
	"testing"
	"time"
)

func TestInterruptionRecoveryExcludesCancellationAndUnrelatedFailures(t *testing.T) {
	for _, name := range []string{"interrupted", "user-cancelled", "cancellation-recorded", "running", "completed", "other-exit", "missing-exit", "retry-limit"} {
		t.Run(name, func(t *testing.T) {
			exit := 137
			d := &task.TaskDetailResponse{LatestAttempt: &task.TaskAttempt{Status: task.AttemptStatusFailed, ExitCode: &exit}}
			d.Status = task.TaskStatusBlocked
			switch name {
			case "user-cancelled":
				d.LatestAttempt.CancelRequested = true
			case "cancellation-recorded":
				now := time.Now()
				d.LatestAttempt.CanceledAt = &now
			case "running":
				d.LatestAttempt.Status = task.AttemptStatusRunning
			case "completed":
				d.Status = task.TaskStatusDone
			case "other-exit":
				exit = 3
			case "missing-exit":
				d.LatestAttempt.ExitCode = nil
			case "retry-limit":
				d.Attempts = []*task.TaskAttempt{{}, {}, {}}
			}
			if got := interruptedInboxTaskCanResume(d); got != (name == "interrupted") {
				t.Fatalf("recovery eligible=%t", got)
			}
		})
	}
	if interruptedInboxTaskCanResume(nil) || interruptedInboxTaskCanResume(&task.TaskDetailResponse{}) {
		t.Fatal("missing canonical attempt accepted")
	}
}

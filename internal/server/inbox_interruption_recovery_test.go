package server

import (
	"fmt"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"testing"
	"time"
)

func TestInterruptionRecoveryExcludesCancellationAndUnrelatedFailures(t *testing.T) {
	for _, name := range []string{"interrupted", "user-cancelled", "cancellation-recorded", "running", "completed", "committed", "other-exit", "missing-exit", "retry-limit"} {
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
			case "committed":
				d.CommitSHA = "1b0f00ae0bb49ddc0512868b54df35dcbb77d976"
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

func TestServerRestartInterruptionRecovery(t *testing.T) {
	d := &task.TaskDetailResponse{
		Status: task.TaskStatusBlocked,
		LatestAttempt: &task.TaskAttempt{
			Status:       task.AttemptStatusInterrupted,
			ErrorMessage: "Execution was interrupted during server restart.",
		},
	}
	if !interruptedInboxTaskCanResume(d) {
		t.Fatal("expected server restart interrupted attempt to be eligible for auto-resume")
	}
}

func TestTaskRevisionBudgetExcludesCancelledAttempts(t *testing.T) {
	d := &task.TaskDetailResponse{Attempts: []*task.TaskAttempt{
		{Status: task.AttemptStatusFailed},
		{Status: task.AttemptStatusCanceled},
		{Status: task.AttemptStatusFailed, CancelRequested: true},
	}}
	if got := taskRevisionAttemptCount(d); got != 1 {
		t.Fatal("cancellation exhausted correction budget", got)
	}
	d.Attempts = append(d.Attempts, &task.TaskAttempt{Status: task.AttemptStatusSucceeded}, &task.TaskAttempt{Status: task.AttemptStatusFailed})
	if got := taskRevisionAttemptCount(d); got != 3 {
		t.Fatal("ordinary attempts escaped retry limit", got)
	}
}

func TestProviderLimitRecoveryRequiresRecordedStoppedFailure(t *testing.T) {
	for _, scenario := range []string{"quota", "region", "unknown", "cancelled", "active", "limit"} {
		t.Run(scenario, func(t *testing.T) {
			d := &task.TaskDetailResponse{Status: task.TaskStatusBlocked, LatestAttempt: &task.TaskAttempt{Status: task.AttemptStatusFailed, ErrorMessage: "The AI provider reported a usage limit. The run was stopped; inspect saved progress."}}
			switch scenario {
			case "region":
				d.LatestAttempt.ErrorMessage = "The AI provider reported a location restriction. The run was stopped; inspect saved progress."
			case "unknown":
				d.LatestAttempt.ErrorMessage = "generic failure"
			case "cancelled":
				d.LatestAttempt.CancelRequested = true
			case "active":
				d.LatestAttempt.Status = task.AttemptStatusRunning
			case "limit":
				d.Attempts = []*task.TaskAttempt{{Status: task.AttemptStatusFailed}, {Status: task.AttemptStatusFailed}, {Status: task.AttemptStatusFailed}}
			}
			if got := providerLimitedInboxTaskCanResume(d); got != (scenario == "quota" || scenario == "region") {
				t.Fatal("unsafe provider recovery eligibility", scenario, got)
			}
		})
	}
}

func TestInfrastructureRecoveryHasIndependentFiniteLimit(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{Status: task.TaskStatusBlocked}}
	for i := 0; i < 5; i++ {
		a := &task.TaskAttempt{ID: fmt.Sprint(i), Status: task.AttemptStatusInterrupted, ErrorMessage: runner.StorageInterruptionMessage}
		d.Attempts = append(d.Attempts, a)
		d.LatestAttempt = a
		if got := interruptedInboxTaskCanResume(d); got != (i < 4) {
			t.Fatalf("interruption %d recovery=%t", i+1, got)
		}
	}
	if taskRevisionAttemptCount(d) != 0 {
		t.Fatal("storage interruption spent business correction budget")
	}
	d.LatestAttempt.CancelRequested = true
	if interruptedInboxTaskCanResume(d) {
		t.Fatal("cancellation bypassed")
	}
}

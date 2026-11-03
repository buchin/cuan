package runner

import (
	"context"
	"errors"
	"github.com/buchin/cuan/internal/task"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func lifecycleFixture(t *testing.T) (*RunnerService, *task.MemoryStore, *task.Task, *task.TaskAttempt) {
	t.Helper()
	store := task.NewMemoryStore()
	svc := NewService(NewMemoryStore(store), store)
	now := time.Now().UTC()
	lease, deadline := now.Add(time.Minute), now.Add(25*time.Minute)
	item := &task.Task{ID: "lifecycle", Title: "Verify a scoped change", Status: task.TaskStatusDoing, Summary: "Implemented scoped change", CompleteNote: "Verified targeted output", BranchName: "task/lifecycle", SourceRoomID: "room", CreatedAt: now, UpdatedAt: now, DoDChecklist: []task.DoDItem{{Text: "Correct output", Completed: true, Evidence: "fixture output matched expected value"}}}
	if err := store.CreateTask(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	att := &task.TaskAttempt{ID: "lifecycle-attempt", TaskID: item.ID, RunnerID: "fixture-runner", Status: task.AttemptStatusRunning, AttemptNumber: 1, StartedAt: &now, CreatedAt: now, LeaseExpiresAt: &lease, ExecutionDeadline: &deadline}
	if err := store.CreateAttempt(context.Background(), att); err != nil {
		t.Fatal(err)
	}
	return svc, store, item, att
}
func lifecycleResult() CompleteAttemptRequest {
	return CompleteAttemptRequest{RunnerID: "fixture-runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, AgentReportConfirmed: true, DoDValidation: &DoDValidationResult{Passed: true, Action: "mark_done"}}
}

func TestExecutionCompletionFencesLateOrInvalidReports(t *testing.T) {
	for _, scenario := range []string{"final", "wrong-runner", "expired-lease", "superseded", "cancelled", "deadline"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			svc, store, item, att := lifecycleFixture(t)
			req := lifecycleResult()
			switch scenario {
			case "final":
				att.Status = task.AttemptStatusFailed
				store.UpdateAttempt(ctx, att)
				item.Status = task.TaskStatusBlocked
				store.UpdateTask(ctx, item)
			case "wrong-runner":
				req.RunnerID = "other"
			case "expired-lease":
				expired := time.Now().Add(-time.Second)
				att.LeaseExpiresAt = &expired
				store.UpdateAttempt(ctx, att)
			case "superseded":
				next := *att
				next.ID = "new-attempt"
				next.AttemptNumber = 2
				next.CreatedAt = att.CreatedAt.Add(time.Second)
				store.CreateAttempt(ctx, &next)
			case "cancelled":
				item.Status = "cancelled"
				store.UpdateTask(ctx, item)
			case "deadline":
				expired := time.Now().Add(-time.Minute)
				att.ExecutionDeadline = &expired
				store.UpdateAttempt(ctx, att)
			}
			before, _ := store.GetTask(ctx, item.ID)
			oldAttempt, _ := store.GetAttempt(ctx, att.ID)
			var notified atomic.Int32
			svc.SetChatNotifier(NewDefaultBusinessChatNotifier(func(context.Context, string, string) error { notified.Add(1); return nil }))
			if _, err := svc.Complete(ctx, att.ID, req); !errors.Is(err, task.ErrAttemptFinal) && !errors.Is(err, task.ErrRequestConflict) {
				t.Fatalf("expected fencing rejection: %v", err)
			}
			after, _ := store.GetTask(ctx, item.ID)
			saved, _ := store.GetAttempt(ctx, att.ID)
			if after.Status != before.Status || after.CompleteNote != before.CompleteNote || saved.Status != oldAttempt.Status || notified.Load() != 0 {
				t.Fatal("stale report mutated state or emitted notification")
			}
		})
	}
}
func TestExecutionCompletionConcurrentDuplicateCommitsOnce(t *testing.T) {
	ctx := context.Background()
	svc, store, item, att := lifecycleFixture(t)
	var notifications atomic.Int32
	svc.SetChatNotifier(NewDefaultBusinessChatNotifier(func(context.Context, string, string) error { notifications.Add(1); return nil }))
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.Complete(ctx, att.ID, lifecycleResult()); err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, task.ErrAttemptFinal) && !errors.Is(err, task.ErrRequestConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	after, _ := store.GetTask(ctx, item.ID)
	if accepted.Load() != 1 || notifications.Load() != 1 || after.Status != task.TaskStatusDone {
		t.Fatalf("accepted=%d notified=%d status=%s", accepted.Load(), notifications.Load(), after.Status)
	}
}
func TestExecutionCompletionCannotApproveUncheckedEvidence(t *testing.T) {
	ctx := context.Background()
	svc, store, item, att := lifecycleFixture(t)
	item.DoDChecklist[0].Completed = false
	item.DoDChecklist[0].Evidence = ""
	store.UpdateTask(ctx, item)
	if _, err := svc.Complete(ctx, att.ID, lifecycleResult()); err != nil {
		t.Fatal(err)
	}
	after, _ := store.GetTask(ctx, item.ID)
	if after.Status != task.TaskStatusBlocked || after.DoDChecklist[0].Completed {
		t.Fatalf("missing evidence was accepted: %+v", after)
	}
}
func TestExecutionGapRetryPreservesTaskAndBranch(t *testing.T) {
	ctx := context.Background()
	svc, store, item, att := lifecycleFixture(t)
	req := lifecycleResult()
	req.DoDValidation = &DoDValidationResult{Passed: false, Action: "auto_requeue", Fixable: true, RequeueInstructions: "Fix only the missing output check"}
	if _, err := svc.Complete(ctx, att.ID, req); err != nil {
		t.Fatal(err)
	}
	after, _ := store.GetTask(ctx, item.ID)
	attempts, _ := store.ListAttempts(ctx, item.ID)
	if after.ID != item.ID || after.BranchName != item.BranchName || after.Status != task.TaskStatusQueued || len(attempts) != 2 {
		t.Fatalf("lost continuation: %+v attempts=%d", after, len(attempts))
	}
	if len(after.DoDChecklist) != 1 || after.DoDChecklist[0].Text != item.DoDChecklist[0].Text {
		t.Fatal("retry changed approved criteria")
	}
}
func TestExecutionHeartbeatCannotReviveEndedOrForeignAttempt(t *testing.T) {
	ctx := context.Background()
	svc, store, _, att := lifecycleFixture(t)
	if _, err := svc.store.RenewLease(ctx, att.ID, "foreign-runner", time.Minute); err == nil {
		t.Fatal("foreign runner renewed lease")
	}
	att.Status = task.AttemptStatusFailed
	store.UpdateAttempt(ctx, att)
	if _, err := svc.store.RenewLease(ctx, att.ID, att.RunnerID, time.Minute); err == nil {
		t.Fatal("ended attempt renewed lease")
	}
}

func TestExecutionAssignmentReleaseDoesNotClearNewerClaim(t *testing.T) {
	ctx := context.Background()
	svc, _, _, att := lifecycleFixture(t)
	svc.store.RegisterOrUpdateHeartbeat(ctx, &RunnerInfo{ID: att.RunnerID, Status: StatusBusy, CurrentAttemptID: "newer-attempt", CurrentTaskID: "newer-task"})
	if _, err := svc.Complete(ctx, att.ID, lifecycleResult()); err != nil {
		t.Fatal(err)
	}
	saved, _ := svc.store.GetRunner(ctx, att.RunnerID)
	if saved.CurrentAttemptID != "newer-attempt" || saved.Status != StatusBusy {
		t.Fatal("late release cleared newer assignment")
	}
}

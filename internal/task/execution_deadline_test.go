package task

import (
	"context"
	"testing"
	"time"
)

func TestExecutionDeadlineSharedWithReaper(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()
	start := now.Add(-23 * time.Minute)
	lease := now.Add(time.Minute)
	item := &Task{ID: "deadline", Title: "Code work", ExecutionCategory: ExecutionCategoryCode, Status: TaskStatusDoing, CreatedAt: start, UpdatedAt: start}
	store.CreateTask(ctx, item)
	att := &TaskAttempt{ID: "deadline-att", TaskID: item.ID, RunnerID: "runner", Status: AttemptStatusRunning, AttemptNumber: 1, StartedAt: &start, CreatedAt: start, LeaseExpiresAt: &lease}
	store.CreateAttempt(ctx, att)
	if err := svc.ReconcileZombieAttempts(ctx, 20*time.Minute, func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	after, _ := store.GetTask(ctx, item.ID)
	if after.Status != TaskStatusDoing {
		t.Fatal("runner's valid 25-minute execution was reaped at 20 minutes")
	}
	deadline := now.Add(time.Minute)
	att.ExecutionDeadline = &deadline
	store.UpdateAttempt(ctx, att)
	t.Setenv("CUAN_TASK_TIMEOUT_OVERRIDE", "1ms")
	if err := svc.ReconcileZombieAttempts(ctx, time.Millisecond, nil); err != nil {
		t.Fatal(err)
	}
	after, _ = store.GetTask(ctx, item.ID)
	if after.Status != TaskStatusDoing {
		t.Fatal("configuration change replaced the claimed deadline")
	}
	expired := now.Add(-time.Minute)
	att.ExecutionDeadline = &expired
	store.UpdateAttempt(ctx, att)
	if err := svc.ReconcileZombieAttempts(ctx, time.Hour, nil); err != nil {
		t.Fatal(err)
	}
	after, _ = store.GetTask(ctx, item.ID)
	saved, _ := store.GetAttempt(ctx, att.ID)
	if after.Status != TaskStatusBlocked || saved.Status != AttemptStatusFailed {
		t.Fatalf("expired result not blocked: %+v %+v", after, saved)
	}
}
func TestAcceptanceEvidenceRequiredForEveryCriterion(t *testing.T) {
	item := &Task{Summary: "Finished", CompleteNote: "Checked", DoDChecklist: []DoDItem{{Text: "A", Completed: true, Evidence: "Observed A"}, {Text: "B", Completed: true}}}
	if len(AcceptanceGaps(item)) != 1 {
		t.Fatal("missing per-criterion evidence accepted")
	}
	item.DoDChecklist[1].Evidence = "Observed B"
	if len(AcceptanceGaps(item)) != 0 {
		t.Fatal("complete evidence rejected")
	}
}

func TestQueuedAttemptDoesNotConsumeExecutionDeadline(t *testing.T) {
	for _, legacyDeadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "unclaimed", true: "legacy queued deadline"}[legacyDeadline], func(t *testing.T) {
			ctx := context.Background()
			store := NewMemoryStore()
			svc := NewService(store, nil)
			old := time.Now().Add(-3 * time.Hour)
			expired := old.Add(time.Minute)
			item := &Task{ID: "waiting", Title: "Queued work", Status: TaskStatusQueued, CreatedAt: old, UpdatedAt: old}
			att := &TaskAttempt{ID: "waiting-attempt", TaskID: item.ID, RunnerID: "runner", Status: AttemptStatusQueued, CreatedAt: old, AttemptNumber: 1}
			if legacyDeadline {
				att.ExecutionDeadline = &expired
			}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			if err := store.CreateAttempt(ctx, att); err != nil {
				t.Fatal(err)
			}
			if err := svc.ReconcileZombieAttempts(ctx, time.Millisecond, func(string) bool { return false }); err != nil {
				t.Fatal(err)
			}
			saved, _ := store.GetAttempt(ctx, att.ID)
			work, _ := store.GetTask(ctx, item.ID)
			if saved.Status != AttemptStatusQueued || saved.FinishedAt != nil || work.Status != TaskStatusQueued {
				t.Fatal("waiting task was reaped as executed")
			}
		})
	}
}

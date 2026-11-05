package task

import (
	"context"
	"testing"
	"time"
)

func TestOperationalAcceptanceAuditMemory(t *testing.T) {
	operationalAcceptanceAuditContract(t, NewMemoryStore(), "default")
}
func operationalAcceptanceAuditContract(t *testing.T, store Store, project string) {
	ctx := context.Background()
	now := time.Now().UTC()
	id := "ops-audit-fixture"
	item := &Task{ID: id, ProjectID: project, ExecutionCategory: ExecutionCategoryOps, Instructions: ReconciliationExecutionMarker, Title: "Approved operation", Status: TaskStatusDone, CreatedAt: now, UpdatedAt: now}
	if err := store.CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	a := &TaskAttempt{ID: id + "-attempt", TaskID: id, RunnerID: "fixture", AgentName: "fixture", Status: AttemptStatusSucceeded, CreatedAt: now}
	if err := store.CreateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	reader := store.(OperationalAcceptanceAuditStore)
	check := func(want bool) {
		t.Helper()
		ids, err := reader.OperationalAcceptanceTaskIDs(ctx)
		found := false
		for _, x := range ids {
			found = found || x == id
		}
		if err != nil || found != want {
			t.Fatal("wrong audit eligibility", ids, err)
		}
	}
	check(true)
	a.CancelRequested = true
	if err := store.UpdateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	check(false)
	if changed, err := reader.RequeueOperationalAcceptance(ctx, id, a.ID, now); err != nil || changed {
		t.Fatal("audit overrode cancellation", err)
	}
	a.CancelRequested = false
	if err := store.UpdateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	check(true)
	if err := store.CreateActivity(ctx, &TaskActivity{ID: id + "-audit", TaskID: id, AttemptID: &a.ID, ActivityType: OperationalAcceptanceAudit, Author: "coordinator", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	check(false)
	if changed, err := reader.RequeueOperationalAcceptance(ctx, id, a.ID, now.Add(-time.Second)); err != nil || changed {
		t.Fatal("audit overwrote edited snapshot", err)
	}
	if changed, err := reader.RequeueOperationalAcceptance(ctx, id, a.ID, now); err != nil || !changed {
		t.Fatal("audit failed to requeue exact snapshot", err)
	}
	if changed, err := reader.RequeueOperationalAcceptance(ctx, id, a.ID, now); err != nil || changed {
		t.Fatal("audit applied twice", err)
	}
	latest, err := store.GetLatestAttempt(ctx, id)
	if err != nil || latest.ID != a.ID || latest.Status != AttemptStatusSucceeded {
		t.Fatal("audit changed execution", err)
	}
}

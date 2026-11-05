package task

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestLegacyPublicationAcceptanceAuditMemory(t *testing.T) {
	publicationAcceptanceAuditContract(t, NewMemoryStore(), "publication-project")
}

func publicationAcceptanceAuditContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	reader := store.(PublicationAcceptanceAuditStore)
	for n, c := range []struct {
		name                        string
		cancelled, audited, current bool
	}{
		{name: "candidate"}, {name: "cancelled", cancelled: true}, {name: "audited", audited: true}, {name: "new template", current: true},
	} {
		id := fmt.Sprintf("publication_audit_%d", n)
		instructions := "Cuan managed delegation v1.\nOriginal scope.\nStay within this scope; do not deploy unless authorized."
		if c.current {
			instructions = "Cuan managed delegation v1.\nOriginal scope.\nStay within this scope."
		}
		item := &Task{ID: id, ProjectID: project, Title: id, Instructions: instructions, Status: TaskStatusDone, ExecutionCategory: ExecutionCategoryCode, CreatedAt: now, UpdatedAt: now}
		if err := store.CreateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		a := &TaskAttempt{ID: id + "_attempt", TaskID: id, AgentName: "fixture", RunnerID: "fixture", Status: AttemptStatusSucceeded, CancelRequested: c.cancelled, CreatedAt: now}
		if err := store.CreateAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		for _, event := range []*TaskActivity{
			{ID: id + "_assessment", TaskID: id, AttemptID: &a.ID, ActivityType: "coordinator_review_assessment", Author: "coordinator", CreatedAt: now},
			{ID: id + "_accepted", TaskID: id, ActivityType: ActivityTypeStatusChange, Author: "coordinator", NewStatus: TaskStatusDone, CreatedAt: now},
		} {
			if err := store.CreateActivity(ctx, event); err != nil {
				t.Fatal(err)
			}
		}

		if c.audited {
			if err := store.CreateActivity(ctx, &TaskActivity{ID: id + "_audit", TaskID: id, AttemptID: &a.ID, ActivityType: ActivityPublicationAcceptanceAudit, Author: "coordinator", CreatedAt: now}); err != nil {
				t.Fatal(err)
			}
		}
		ids, err := reader.LegacyPublicationAcceptanceTaskIDs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, candidate := range ids {
			found = found || candidate == id
		}
		if found != (n == 0) {
			t.Fatalf("%s audit eligibility=%t", c.name, found)
		}
		if n == 0 {
			if changed, err := reader.RequeueAcceptedSnapshot(ctx, id, a.ID, now.Add(-time.Second)); err != nil || changed {
				t.Fatal("stale snapshot reopened acceptance", err)
			}
			if changed, err := reader.RequeueAcceptedSnapshot(ctx, id, "different-attempt", now); err != nil || changed {
				t.Fatal("different execution reopened acceptance", err)
			}
			if changed, err := reader.RequeueAcceptedSnapshot(ctx, id, a.ID, now); err != nil || !changed {
				t.Fatal("exact acceptance not requeued", err)
			}
			saved, _ := store.GetTask(ctx, id)
			original, _ := store.GetAttempt(ctx, a.ID)
			if saved.Status != TaskStatusReview || saved.Instructions != instructions || original.Status != AttemptStatusSucceeded {
				t.Fatal("review audit rewrote history")
			}
		}
	}
}

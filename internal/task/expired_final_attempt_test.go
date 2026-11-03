package task

import (
	"context"
	"testing"
	"time"
)

func TestExpiredAcceptedAttemptCleanupPreservesResultAndFencesRenewal(t *testing.T) {
	for _, scenario := range []string{"expired", "renewed", "wrong-runner", "still-doing"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := NewMemoryStore()
			now := time.Now().UTC()
			lease := now.Add(-time.Minute)
			item := &Task{ID: "accepted", Status: TaskStatusDone, Summary: "Accepted output", CompleteNote: "Original verification", UpdatedAt: now, CreatedAt: now}
			att := &TaskAttempt{ID: "expired-attempt", TaskID: item.ID, RunnerID: "runner-a", Status: AttemptStatusRunning, LeaseExpiresAt: &lease, CreatedAt: now}
			runner := "runner-a"
			if scenario == "renewed" {
				lease = now.Add(time.Minute)
			}
			if scenario == "wrong-runner" {
				runner = "runner-b"
			}
			if scenario == "still-doing" {
				item.Status = TaskStatusDoing
			}
			store.CreateTask(ctx, item)
			store.CreateAttempt(ctx, att)
			retired, err := store.RetireExpiredAcceptedAttempt(ctx, att.ID, runner)
			if err != nil {
				t.Fatal(err)
			}
			if retired != (scenario == "expired") {
				t.Fatal("cleanup fence failed", scenario, retired)
			}
			after, _ := store.GetTask(ctx, item.ID)
			if after.Status != item.Status || after.Summary != item.Summary || after.CompleteNote != item.CompleteNote || !after.UpdatedAt.Equal(now) {
				t.Fatal("accepted result was changed")
			}
			if retired {
				saved, _ := store.GetAttempt(ctx, att.ID)
				if saved.Status != AttemptStatusInterrupted || saved.ErrorMessage != AcceptedTaskLeaseCleanup {
					t.Fatal("lease not retired")
				}
				again, _ := store.RetireExpiredAcceptedAttempt(ctx, att.ID, runner)
				if again {
					t.Fatal("duplicate cleanup")
				}
			}
		})
	}
}

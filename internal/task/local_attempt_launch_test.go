package task

import (
	"context"
	"testing"
	"time"
)

func TestTerminalReconciliationPreservesNewAuthorizedLaunch(t *testing.T) {
	for _, board := range []string{TaskStatusDoing, TaskStatusQueued} {
		t.Run(board, func(t *testing.T) {
			store := NewMemoryStore()
			now := time.Now().UTC()
			store.tasks["task"] = &Task{ID: "task", Status: board}
			store.attempts["old"] = &TaskAttempt{ID: "old", TaskID: "task", Status: AttemptStatusFailed, CreatedAt: now}
			store.launches["old-launch"] = &TaskLaunch{ID: "old-launch", TaskID: "task", State: "started", CreatedAt: now.Add(-time.Minute)}
			store.launches["new-launch"] = &TaskLaunch{ID: "new-launch", TaskID: "task", State: "claimed", CreatedAt: now.Add(time.Minute), LeaseToken: "new-worker"}
			if err := store.ReconcileLocalTask(context.Background(), "task", "old"); err != nil {
				t.Fatal(err)
			}
			if l := store.launches["new-launch"]; l.State != "claimed" || l.LeaseToken != "new-worker" {
				t.Fatal("old terminal reconciliation stopped new launch", l)
			}
			if board == TaskStatusDoing && store.launches["old-launch"].State != "stopped" {
				t.Fatal("old launch not reconciled")
			}
			if board == TaskStatusQueued && store.tasks["task"].Status != TaskStatusQueued {
				t.Fatal("new queue intent overwritten")
			}
		})
	}
}

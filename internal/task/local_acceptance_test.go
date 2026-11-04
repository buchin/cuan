package task

import (
	"context"
	"testing"
	"time"
)

func TestLocalInboxCompletionAwaitsAcceptanceMemory(t *testing.T) {
	localInboxAcceptanceContract(t, NewMemoryStore(), "launch-project")
}
func localInboxAcceptanceContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	for _, room := range []string{"", "91000000-0000-4000-8000-000000000001"} {
		svc := NewService(store, nil)
		item, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{ProjectID: project, Title: "Local result", Instructions: "Approved scope", SourceRoomID: room})
		if err != nil {
			t.Fatal(err)
		}
		item.Status = TaskStatusDoing
		item.Summary = "Recorded findings"
		item.CompleteNote = "Concrete checks recorded"
		if err := store.UpdateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		a := &TaskAttempt{ID: generateID("att"), TaskID: item.ID, RunnerID: "fixture", AgentName: "agent", Status: AttemptStatusSucceeded, CreatedAt: now}
		if err := store.CreateAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"agent_contract", "agent_report"} {
			if err := store.CreateActivity(ctx, &TaskActivity{ID: generateID("act"), TaskID: item.ID, AttemptID: &a.ID, ActivityType: kind, Author: "agent", NewStatus: "done", Message: "Actual saved report", CreatedAt: now}); err != nil {
				t.Fatal(err)
			}
		}
		if err := svc.ReconcileLocalTask(ctx, item.ID, a.ID); err != nil {
			t.Fatal(err)
		}
		current, err := store.GetTask(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		want := TaskStatusDone
		if room != "" {
			want = TaskStatusReview
		}
		if current.Status != want {
			t.Fatalf("source room %q: status %s want %s", room, current.Status, want)
		}
		if room != "" {
			done := TaskStatusDone
			if _, err := svc.UpdateTask(ctx, item.ID, "coordinator", UpdateTaskRequest{Status: &done}); err != nil {
				t.Fatal(err)
			}
			if err := svc.ReconcileLocalTask(ctx, item.ID, a.ID); err != nil {
				t.Fatal(err)
			}
			current, _ = store.GetTask(ctx, item.ID)
			if current.Status != TaskStatusDone {
				t.Fatal("late exit overwrote accepted work")
			}
		}
	}
}

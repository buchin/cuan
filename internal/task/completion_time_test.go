package task

import (
	"context"
	"testing"
	"time"
)

func TestTaskCompletionTimeUsesLifecycleRatherThanEditsOrWorkerExit(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	created := time.Now().UTC().Add(-time.Hour)
	done := created.Add(7 * time.Minute)
	item := &Task{ID: "timed", Title: "Timed result", Status: TaskStatusDone, CreatedAt: created}
	if err := store.CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	for _, a := range []*TaskActivity{
		{ID: "worker", TaskID: item.ID, ActivityType: "attempt_transition", NewStatus: TaskStatusReview, CreatedAt: created.Add(3 * time.Minute)},
		{ID: "accepted", TaskID: item.ID, ActivityType: ActivityTypeStatusChange, NewStatus: TaskStatusDone, CreatedAt: done},
		{ID: "edited", TaskID: item.ID, ActivityType: "comment", CreatedAt: done.Add(time.Minute)},
	} {
		if err := store.CreateActivity(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewService(store, nil)
	assertTime := func(expected *time.Time) {
		t.Helper()
		detail, err := svc.GetTaskDetail(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		page, err := svc.ListTaskPage(ctx, PageOptions{PageSize: 25})
		if err != nil || len(page.Tasks) != 1 {
			t.Fatal("missing task page", err)
		}
		for _, actual := range []*time.Time{detail.CompletedAt, page.Tasks[0].CompletedAt} {
			if (actual == nil) != (expected == nil) || (actual != nil && !actual.Equal(*expected)) {
				t.Fatalf("completion=%v expected=%v", actual, expected)
			}
		}
	}
	assertTime(&done)
	item.UpdatedAt = done.Add(20 * time.Minute)
	item.Title = "Edited title after completion"
	if err := store.UpdateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	assertTime(&done)
	item.Status = TaskStatusReview
	if err := store.UpdateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	assertTime(nil)
	item.Status = TaskStatusDone
	if err := store.UpdateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	secondDone := done.Add(30 * time.Minute)
	if err := store.CreateActivity(ctx, &TaskActivity{ID: "accepted-again", TaskID: item.ID, ActivityType: ActivityTypeStatusChange, NewStatus: TaskStatusDone, CreatedAt: secondDone}); err != nil {
		t.Fatal(err)
	}
	assertTime(&secondDone)
}

func TestTaskCompletionTimeDoesNotInventMissingOrStaleHistory(t *testing.T) {
	start := time.Now().UTC()
	item := &Task{ID: "timed", Status: TaskStatusDone, CreatedAt: start}
	accepted := &TaskActivity{ID: "a", TaskID: item.ID, ActivityType: ActivityTypeStatusChange, NewStatus: TaskStatusDone, CreatedAt: start.Add(time.Minute)}
	for _, history := range [][]*TaskActivity{
		nil,
		{{ID: "old", TaskID: item.ID, ActivityType: ActivityTypeStatusChange, NewStatus: TaskStatusDone, CreatedAt: start.Add(-time.Minute)}},
		{accepted, {ID: "reopened", TaskID: item.ID, ActivityType: ActivityTypeStatusChange, NewStatus: TaskStatusReview, CreatedAt: start.Add(2 * time.Minute)}},
		{{ID: "foreign", TaskID: "other", ActivityType: ActivityTypeStatusChange, NewStatus: TaskStatusDone, CreatedAt: start.Add(time.Minute)}},
	} {
		if got := completionTime(item, history); got != nil {
			t.Fatal("invented completion", got)
		}
	}
}

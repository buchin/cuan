package task

import (
	"context"
	"testing"
	"time"
)

func TestGitHubPrerequisitePersistsWithoutChangingInstructions(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	original := &Task{ID: "task-github", Title: "Approved work", Instructions: "Original human scope", Status: "doing", UpdatedAt: time.Now()}
	if e := store.CreateTask(ctx, original); e != nil {
		t.Fatal(e)
	}
	service := NewService(store, nil)
	for i := 0; i < 2; i++ {
		if e := service.RecordGitHubPrerequisite(ctx, original.ID, "attempt", false); e != nil {
			t.Fatal(e)
		}
	}
	restarted := NewService(store, nil)
	detail, e := restarted.GetTaskDetail(ctx, original.ID)
	if e != nil {
		t.Fatal(e)
	}
	if len(detail.Activities) != 1 || detail.Activities[0].ActivityType != "github_connection_required" || detail.Instructions != "Original human scope" || detail.Status != "doing" {
		t.Fatal("prerequisite duplicated or changed authorized work")
	}
	if e = restarted.RecordGitHubPrerequisite(ctx, original.ID, "attempt", true); e != nil {
		t.Fatal(e)
	}
	detail, e = restarted.GetTaskDetail(ctx, original.ID)
	if e != nil || len(detail.Activities) != 2 {
		t.Fatal("resolution not retained", e)
	}
}

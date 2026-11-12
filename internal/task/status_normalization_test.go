package task

import (
	"context"
	"testing"
)

// The CLI documents "todo", "in_progress" and "canceled" as accepted status
// values. Every one of them must work, otherwise the documented command fails
// with "invalid task status" on a status the help text itself suggests.
func TestDocumentedStatusSpellingsAreAccepted(t *testing.T) {
	cases := map[string]string{
		"todo":        TaskStatusBacklog,
		"backlog":     TaskStatusBacklog,
		"in_progress": TaskStatusDoing,
		"doing":       TaskStatusDoing,
		"running":     TaskStatusDoing,
		"canceled":    "cancelled",
		"cancelled":   "cancelled",
		"review":      TaskStatusReview,
		"done":        TaskStatusDone,
		"blocked":     TaskStatusBlocked,
	}
	for input, want := range cases {
		if got := NormalizeStatus(input); got != want {
			t.Fatalf("NormalizeStatus(%q) = %q, want %q", input, got, want)
		}
	}
}

// The normalisation must actually reach the recorded board status, not just
// parse: a task updated with "in_progress" has to be doing.
func TestUpdateTaskAcceptsDocumentedSpelling(t *testing.T) {
	ctx := context.Background()
	svc := NewService(NewMemoryStore(), nil)
	created, err := svc.CreateTask(ctx, "fixture", CreateTaskRequest{Title: "Documented spelling"})
	if err != nil {
		t.Fatal(err)
	}
	status := "in_progress"
	updated, err := svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &status})
	if err != nil {
		t.Fatalf("documented status rejected: %v", err)
	}
	if updated.Status != TaskStatusDoing {
		t.Fatalf("expected doing, got %q", updated.Status)
	}
}

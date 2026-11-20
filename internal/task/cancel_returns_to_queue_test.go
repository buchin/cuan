package task

import (
	"context"
	"testing"
	"time"
)

// Cancelling is an owner decision, not evidence that the work is defective.
// Four production tasks sat in "blocked" with zero failed attempts only because
// the owner cancelled them, so the board showed a failure that never happened.
func TestCancelReturnsTaskToQueueNotBlocked(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, _ := svc.CreateTask(ctx, "fixture", CreateTaskRequest{Title: "Cancel me"})
	if _, err := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CancelTask(ctx, created.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != TaskStatusCancelled {
		t.Fatalf("cancelled task should be recorded as cancelled, got %q", detail.Status)
	}
	if detail.LatestAttempt.Status != AttemptStatusCanceled {
		t.Fatalf("attempt should be recorded as cancelled, got %q", detail.LatestAttempt.Status)
	}
}

// Returning a cancelled task to the queue must not hide an unknown outcome. An
// attempt whose process died without a result is still a blocker, otherwise the
// board would offer to run work whose real state nobody knows.
func TestUnknownOutcomeAfterCancelStillBlocks(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, _ := svc.CreateTask(ctx, "fixture", CreateTaskRequest{Title: "Cancel then die"})
	first, err := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateAttemptStatus(ctx, first.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CancelTask(ctx, created.ID, "owner"); err != nil {
		t.Fatal(err)
	}

	// A later attempt loses its process without reporting an outcome.
	second, err := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal"})
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Second)
	second.LeaseExpiresAt = &past
	if err := store.UpdateAttempt(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecoverLocalAttempts(ctx); err != nil {
		t.Fatal(err)
	}

	detail, err := svc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.LatestAttempt.Status != AttemptStatusInterrupted {
		t.Fatalf("unknown outcome should be interrupted, got %q", detail.LatestAttempt.Status)
	}
	if detail.Status != TaskStatusBlocked {
		t.Fatalf("unknown outcome must block the task, got %q", detail.Status)
	}
}

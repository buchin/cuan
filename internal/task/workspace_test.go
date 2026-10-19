package task

import (
	"context"
	"errors"
	"testing"
)

func TestTaskWorkspacePersistsReviewFieldsAndRequiresEvidence(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Improve task UX", Instructions: "Preserve human instructions", DoDChecklist: []DoDItem{{Text: "Verify on mobile"}}})
	if err != nil {
		t.Fatal(err)
	}
	summary, note, review, done := "Implemented responsive detail", "Verified at390px", TaskStatusReview, TaskStatusDone
	updated, err := svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Summary: &summary, CompleteNote: &note, Status: &review})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Instructions != created.Instructions || updated.Summary != summary || updated.CompleteNote != note {
		t.Fatal("work notes replaced human instructions")
	}
	if _, err = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &done}); !errors.Is(err, ErrInvalidTaskInput) {
		t.Fatal("unchecked criteria approved", err)
	}
	checked := []DoDItem{{Text: "Verify on mobile", Completed: true}}
	if _, err = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{DoDChecklist: &checked, Status: &done}); err != nil {
		t.Fatal(err)
	}
	fresh, err := svc.GetTaskDetail(ctx, created.ID)
	if err != nil || fresh.Status != done || !fresh.DoDChecklist[0].Completed {
		t.Fatal("review result not retained", fresh, err)
	}
	fresh.DoDChecklist[0].Text = "mutated outside store"
	other, _ := store.GetTask(ctx, created.ID)
	if other.DoDChecklist[0].Text == fresh.DoDChecklist[0].Text {
		t.Fatal("checklist references leaked")
	}
}
func TestTaskWorkspaceLaunchRetryIsDurableAndDoesNotReviveFinalAttempt(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, _ := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Idempotent execution"})
	req := CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_one", RequestID: "10000000-0000-4000-8000-000000000001", RequestHash: "same-payload"}
	first, err := svc.CreateAttempt(ctx, created.ID, "owner", req)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.DeleteTask(ctx, created.ID); !errors.Is(err, ErrTaskActive) {
		t.Fatal("active task deleted", err)
	}
	done := TaskStatusDone
	if _, err = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &done}); !errors.Is(err, ErrTaskActive) {
		t.Fatal("active task approved", err)
	}
	exit := 0
	if _, err = svc.UpdateAttemptStatus(ctx, first.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusSucceeded, ExitCode: &exit}); err != nil {
		t.Fatal(err)
	}
	restarted := NewService(store, nil)
	second, err := restarted.CreateAttempt(ctx, created.ID, "owner", req)
	if err != nil || second.ID != first.ID || second.Status != AttemptStatusSucceeded {
		t.Fatal("replay revived execution", second, err)
	}
	req.RequestHash = "different-payload"
	if _, err = restarted.CreateAttempt(ctx, created.ID, "owner", req); !errors.Is(err, ErrRequestConflict) {
		t.Fatal("changed replay accepted", err)
	}
	attempts, _ := store.ListAttempts(ctx, created.ID)
	if len(attempts) != 1 {
		t.Fatal("duplicate durable attempt")
	}
	review := TaskStatusReview
	doing := TaskStatusDoing
	svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &review})
	svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &doing})
	attempts, _ = store.ListAttempts(ctx, created.ID)
	if len(attempts) != 1 {
		t.Fatal("status edit queued ghost revision")
	}
}

func TestTaskWorkspaceLateExitPreservesApprovalAndStaleEdits(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	created, e := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Review evidence"})
	if e != nil {
		t.Fatal(e)
	}
	doing := TaskStatusDoing
	if _, e = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &doing}); e != nil {
		t.Fatal(e)
	}
	a, e := svc.CreateAttempt(ctx, created.ID, "owner", CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "fixture"})
	if e != nil {
		t.Fatal(e)
	}
	zero := 0
	svc.UpdateAttemptStatus(ctx, a.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusSucceeded, ExitCode: &zero})
	svc.ReconcileLocalTask(ctx, created.ID, a.ID)
	note, done := "Verified output", TaskStatusDone
	if _, e = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &done, CompleteNote: &note}); e != nil {
		t.Fatal(e)
	}
	if e = svc.ReconcileLocalTask(ctx, created.ID, a.ID); e != nil {
		t.Fatal(e)
	}
	detail, _ := svc.GetTaskDetail(ctx, created.ID)
	if detail.Status != done {
		t.Fatal("late exit overwrote approval")
	}
	title := "Stale title"
	if _, e = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Title: &title, ExpectedUpdatedAt: &created.UpdatedAt}); !errors.Is(e, ErrRequestConflict) {
		t.Fatal("stale edit accepted", e)
	}
}

func TestTaskPromptMetadataAndIdempotency(t *testing.T) {
	svc := NewService(NewMemoryStore(), nil)
	ctx := context.Background()
	req := CreateTaskRequest{Instructions: "Production down: restore checkout\nKeep all human instructions.", RequestID: "50000000-0000-4000-8000-000000000001"}
	created, e := svc.CreateTask(ctx, "owner", req)
	if e != nil {
		t.Fatal(e)
	}
	if created.Title != "Production down: restore checkout" || created.Priority != PriorityUrgent || created.Instructions != req.Instructions {
		t.Fatal("prompt inference", created)
	}
	refined := "Restore checkout safely"
	svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Title: &refined})
	replay, e := svc.CreateTask(ctx, "owner", req)
	if e != nil || replay.ID != created.ID || replay.Title != refined {
		t.Fatal("metadata refinement broke create replay", replay, e)
	}
}

package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestCompletionAtomicallyBindsNativeBranchDiff(t *testing.T) {
	svc, store, item, attempt := lifecycleFixture(t)
	req := lifecycleResult()
	req.CommitSHA, req.DiffSummary = strings.Repeat("a", 40), "web/src/features/landing/LandingPage.tsx | 435 +"
	ctx := context.Background()
	if _, err := svc.Complete(ctx, attempt.ID, req); err != nil {
		t.Fatal(err)
	}
	current, err := task.NewService(store, nil).GetTaskDetail(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	changes := task.AttemptChangeEvidence(current)
	if changes.RunnerChangeRecord == nil || changes.ResultCommit != req.CommitSHA || changes.Summary != req.DiffSummary || changes.AttemptID != attempt.ID {
		t.Fatal("completion lost bound diff", changes)
	}
	if _, err := svc.Complete(ctx, attempt.ID, req); err == nil {
		t.Fatal("duplicate completion rewrote evidence")
	}
}

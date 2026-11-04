package runner

import (
	"context"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestExecutionCompletionBindsVerifierIncludingNoCheck(t *testing.T) {
	for _, summary := range []string{"", "Project verification: current check failed"} {
		t.Run(summary, func(t *testing.T) {
			ctx := context.Background()
			svc, store, item, att := lifecycleFixture(t)
			item.TestSummary = "OLD_FAILED_CHECK"
			if err := store.UpdateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			request := CompleteAttemptRequest{RunnerID: att.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 2, ErrorMessage: "exit code 2", TestSummary: summary}
			if _, err := svc.Complete(ctx, att.ID, request); err != nil {
				t.Fatal(err)
			}
			saved, err := store.GetTask(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			activities, err := store.ListActivities(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			evidence := task.AttemptVerificationEvidence(&task.TaskDetailResponse{Task: *saved, LatestAttempt: att, Activities: activities})
			if evidence.Current != summary || evidence.AttemptID != att.ID || evidence.Provenance != "current_attempt_verification" {
				t.Fatalf("completion did not bind its actual verifier: %+v", evidence)
			}
			if summary == "" && (saved.TestSummary != "OLD_FAILED_CHECK" || evidence.Historical != "OLD_FAILED_CHECK") {
				t.Fatal("old evidence was destroyed rather than retained as history")
			}
			if _, err := svc.Complete(ctx, att.ID, request); err == nil {
				t.Fatal("duplicate completion accepted")
			}
			after, _ := store.ListActivities(ctx, item.ID)
			if len(after) != len(activities) {
				t.Fatal("duplicate completion wrote duplicate evidence")
			}
		})
	}
}

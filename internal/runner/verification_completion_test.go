package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestSupplementalVerificationCompletionRequiresSourceAndScope(t *testing.T) {
	for _, scenario := range []string{"valid", "old runner", "wrong source", "dirty", "publication", "changed source evidence"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			now := time.Now().UTC()
			sourceCommit := strings.Repeat("a", 40)
			item := &task.Task{ID: "verification-task", SourceRoomID: "room", ProjectID: "project", Title: "Verify implementation", Instructions: "Original approved scope", CommitSHA: sourceCommit, Status: task.TaskStatusReview, CreatedAt: now, UpdatedAt: now}
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(store.CreateTask(ctx, item))
			source := &task.TaskAttempt{ID: "source", TaskID: item.ID, RunnerID: "runner", Status: task.AttemptStatusSucceeded, CreatedAt: now.Add(-time.Hour)}
			must(store.CreateAttempt(ctx, source))
			summary := "Project verification: test.sh --fast (exit 0)"
			raw, _ := json.Marshal(task.RunnerVerificationRecord{Summary: summary})
			must(store.CreateActivity(ctx, &task.TaskActivity{ID: "checks", TaskID: item.ID, AttemptID: &source.ID, ActivityType: task.ActivityTypeRunnerVerification, Author: source.RunnerID, Message: string(raw), CreatedAt: now}))
			proof := task.VerificationIntent{SourceAttemptID: source.ID, SourceCommit: sourceCommit, InstructionsHash: task.LaunchInstructionsHash(item.Instructions), OwnerID: "owner", RoomID: "room", SourceRequestID: "approval", AssessmentRequestID: "assessment", SourceVerificationHash: task.VerificationSummaryHash(summary)}
			raw, _ = json.Marshal(proof)
			requestID := task.VerificationRequestID(item.ID, sourceCommit)
			job, err := store.SaveLaunch(ctx, task.TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: requestID, AgentID: "agent", OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, InstructionsHash: proof.InstructionsHash, Revision: task.VerificationIntentMarker + string(raw), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			must(err)
			_, err = store.ClaimLaunch(ctx, now, "launch", job.ID)
			must(err)
			tasks := task.NewService(store, nil)
			att, err := tasks.CreateAttempt(ctx, item.ID, "owner", task.CreateAttemptRequest{RunnerID: "runner", AgentName: "agent", RequestID: requestID})
			must(err)
			service := NewService(NewMemoryStore(store), store, nil)
			claim, err := service.Claim(ctx, ClaimTaskRequest{RunnerID: "runner"}, time.Minute)
			must(err)
			if !claim.Claimed || claim.Verification == nil || !claim.PublicationLimits.ReadOnlyRepository || !claim.PublicationLimits.DenyDeploy || !claim.PublicationLimits.DenyPush {
				t.Fatal("claim lost verification scope")
			}
			req := CompleteAttemptRequest{RunnerID: "runner", Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: sourceCommit, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &RepositoryVerification{Baseline: strings.Repeat("b", 64), Final: strings.Repeat("b", 64)}}
			switch scenario {
			case "old runner":
				req.VerifiedSourceCommit = ""
			case "wrong source":
				req.VerifiedSourceCommit = strings.Repeat("c", 40)
			case "dirty":
				req.RepositoryVerification.Final = strings.Repeat("c", 64)
			case "publication":
				req.LiveDeploymentConfirmed = true
			case "changed source evidence":
				must(store.CreateActivity(ctx, &task.TaskActivity{ID: "changed-checks", TaskID: item.ID, AttemptID: &source.ID, ActivityType: task.ActivityTypeRunnerVerification, Author: source.RunnerID, Message: `{"summary":"Project verification: test.sh --fast (exit 1)"}`, CreatedAt: now.Add(time.Second)}))
			}
			result, err := service.Complete(ctx, att.ID, req)
			if scenario == "changed source evidence" {
				if err == nil {
					t.Fatal("accepted invalidated source")
				}
				return
			}
			must(err)
			want := task.AttemptStatusFailed
			if scenario == "valid" {
				want = task.AttemptStatusSucceeded
			}
			if result.Status != want {
				t.Fatalf("got %s want %s", result.Status, want)
			}
			detail, err := tasks.GetTaskDetail(ctx, item.ID)
			must(err)
			if detail.Status == task.TaskStatusDone {
				t.Fatal("verification bypassed coordinator acceptance")
			}
			receipt := false
			for _, a := range detail.Activities {
				if a.ActivityType == "verification_source_verified" && a.AttemptID != nil && *a.AttemptID == att.ID {
					receipt = true
				}
			}
			if receipt != (scenario == "valid") {
				t.Fatal("incorrect source receipt")
			}
		})
	}
}

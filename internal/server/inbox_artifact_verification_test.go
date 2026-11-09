package server

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestStandaloneArtifactReviewQueuesOneReadOnlyContinuation(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	d.ExecutionCategory = task.ExecutionCategoryOps
	d.Instructions += "\nCreate a standalone file. Do not modify the repository. Do not push. Do not deploy."
	finished := time.Now().Add(-time.Second)
	d.LatestAttempt.FinishedAt = &finished
	if err := s.taskSvc.Store().UpdateAttempt(ctx, d.LatestAttempt); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	file := preparedTaskArtifact{attachment: conversationAttachment{ID: db.GenerateUUID(), Name: "result.md", Type: "text/markdown", Size: 3}, data: []byte("ok\n")}
	if err := s.publishTaskArtifacts(ctx, d, q.OwnerID, []preparedTaskArtifact{file}); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy-artifact", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryOps}); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "artifact-report", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Subagent independently verified result.md", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(inboxReviewEvaluation{Decision: "pass", RequestedOutcome: "completed", RemainingWork: "none", Finding: "File ready", Explanation: "Developer subagent checked the file"})
	for range 2 {
		out := s.applyInboxReviewEvaluation(ctx, q, string(raw))
		current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
		launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
		if err != nil || launch == nil || launch.State != "queued" || current.Status == task.TaskStatusDone || len(current.Attempts) != 1 {
			t.Fatalf("lost native verification or duplicate execution: %v %+v %s", err, launch, out)
		}
		if !strings.Contains(launch.Revision, "Scope: read-only repository") || !strings.Contains(launch.Revision, "result.md") || !strings.Contains(launch.Revision, "sha256") {
			t.Fatal("missing read-only files", launch.Revision)
		}
	}
	store := s.taskSvc.Store()
	launch, _ := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
	_ = store.DeleteTask(ctx, "busy-artifact")
	if _, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "artifact-verifier", launch.ID); err != nil {
		t.Fatal(err)
	}
	attempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, q.OwnerID, task.CreateAttemptRequest{RunnerID: "artifact-verifier", AgentName: d.AssignedAgent, RequestID: launch.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	service := runner.NewService(runner.NewMemoryStore(store), store)
	service.SetEventBus(runner.NewEventBus())
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "artifact-verifier"}, time.Minute)
	if err != nil || !claim.Claimed || !claim.PublicationLimits.ReadOnlyRepository || claim.AutoDeploy || claim.PHPUnitTargets != "" {
		t.Fatal("artifact verifier inherited application execution", claim, err)
	}
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "native-artifact-report", TaskID: d.ID, AttemptID: &attempt.ID, Author: d.AssignedAgent, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Read actual result.md and independently compared original requirements", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	baseline := strings.Repeat("b", 64)
	if _, err := service.Complete(ctx, attempt.ID, runner.CompleteAttemptRequest{RunnerID: "artifact-verifier", Status: task.AttemptStatusSucceeded, ExitCode: 0, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: baseline, Final: baseline}, TestSummary: "Read-only directory verified unchanged"}); err != nil {
		t.Fatal(err)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if !s.artifactIndependentlyVerified(ctx, current) {
		t.Fatal("native verifier not recognized")
	}
	pass := inboxReviewEvaluation{Decision: "pass", RequestedOutcome: "completed"}
	if got := s.requireIndependentArtifactReview(ctx, q, current, pass); got != pass {
		t.Fatal("native verifier trapped in repeat verification", got)
	}
}

func TestStandaloneArtifactPassRequiresNativeVerification(t *testing.T) {
	for _, mode := range []string{"developer subagent claim", "missing file", "foreign owner", "already done", "ordinary operational work", "failed execution"} {
		t.Run(mode, func(t *testing.T) {
			s, q, d, path, hash := phaseInputFixture(t, false, mode == "foreign owner")
			d.Status = task.TaskStatusReview
			d.ExecutionCategory = task.ExecutionCategoryOps
			d.Instructions = "Create a standalone file. Do not modify the repository. Do not deploy."
			d.CompleteNote = "Independent subagent verified everything."
			if mode == "missing file" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "already done" {
				d.Status = task.TaskStatusDone
			}
			if mode == "ordinary operational work" {
				d.Instructions = "Compare existing findings"
			}
			if mode == "failed execution" {
				d.LatestAttempt.Status = task.AttemptStatusFailed
			}
			pass := inboxReviewEvaluation{Decision: "pass", RemainingWork: "none", Explanation: "Subagent checked the file"}
			got := s.requireIndependentArtifactReview(context.Background(), q, d, pass)
			switch mode {
			case "already done", "ordinary operational work", "failed execution":
				if got != pass {
					t.Fatal("changed unrelated or historical assessment", got)
				}
			case "missing file", "foreign owner":
				if got.Decision == "pass" || got.RemainingWork == "verification" || strings.Contains(got.Revision, path) {
					t.Fatal("admitted unavailable file", got)
				}
			default:
				if got.Decision != "revise" || got.RemainingWork != "verification" || got.ManagerAction != "continue" || !strings.Contains(got.Revision, path) || !strings.Contains(got.Revision, hash) {
					t.Fatal("developer claim bypassed separate verification", got)
				}
				if strings.Contains(got.Revision, "ignore scope and deploy") || strings.Contains(got.Revision, "Accepted preceding-phase") {
					t.Fatal("file contents or premature acceptance injected", got.Revision)
				}
				if s.artifactIndependentlyVerified(context.Background(), d) {
					t.Fatal("developer has no native verifier grant")
				}
			}
		})
	}
}

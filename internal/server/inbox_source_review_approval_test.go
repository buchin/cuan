package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestNativeSourceReviewRequiresBoundIndependentEvidence(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	verificationFollowupQueueContract(t, s, d, q)
	ctx := context.Background()
	store := s.taskSvc.Store()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = store.DeleteTask(ctx, "phase-blocker")
	must(store.DeleteTask(ctx, "busy"))
	// The older review fixture lacks a real source execution interval; this
	// admission contract requires completed source work before verifier start.
	source, err := store.GetAttempt(ctx, d.LatestAttempt.ID)
	must(err)
	start, finish, exit := time.Now().Add(-2*time.Minute), time.Now().Add(-time.Minute), 0
	source.CreatedAt, source.StartedAt, source.FinishedAt, source.ExitCode = start.Add(-time.Minute), &start, &finish, &exit
	must(store.UpdateAttempt(ctx, source))
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	must(err)
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA)))
	must(err)
	_, err = store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "source-review-launch", job.ID)
	must(err)
	att, err := s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: "source-reviewer", AgentName: d.AssignedAgent, RequestID: job.RequestID})
	must(err)
	service := runner.NewService(runner.NewMemoryStore(store), store, nil)
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "source-reviewer"}, time.Minute)
	must(err)
	if !claim.Claimed {
		t.Fatal("verifier not claimed")
	}
	_, err = service.Complete(ctx, att.ID, runner.CompleteAttemptRequest{RunnerID: "source-reviewer", Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: d.CommitSHA, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("b", 64), Final: strings.Repeat("b", 64)}})
	must(err)
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	must(err)
	assessment := inboxReviewEvaluation{Decision: "revise", SourceReview: "pass", RemainingWork: "publication", Explanation: "Source checked independently; publication pending.", Revision: "Publish the reviewed source only."}
	if s.saveNativeSourceReview(ctx, d, q.RequestID, assessment) == nil {
		t.Fatal("unsaved model verdict approved source")
	}
	report := &task.TaskActivity{ID: "independent-final-report", TaskID: d.ID, AttemptID: &att.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Author: "agent", Message: "Observed source and targeted regression checks passed; publication remains pending.", CreatedAt: d.LatestAttempt.FinishedAt.Add(-time.Nanosecond)}
	must(store.CreateActivity(ctx, report))
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	must(err)
	_, err = s.saveReviewAssessment(ctx, d, q, assessment)
	must(err)
	approved, err := task.NativeSourceReviewForAttempt(ctx, store, &d.Task, att.ID)
	must(err)
	if approved == nil || approved.SourceCommit != d.CommitSHA || approved.ReviewAttemptID != att.ID {
		t.Fatal("review lost source binding", approved)
	}
	must(s.saveNativeSourceReview(ctx, d, q.RequestID, assessment))
	changed := d.Task
	changed.CommitSHA = strings.Repeat("c", 40)
	if _, err := task.NativeSourceReviewForAttempt(ctx, store, &changed, att.ID); err == nil {
		t.Fatal("approval accepted different source")
	}
	report.Message = "Replacement report claiming a different result"
	report.ID, report.CreatedAt = "replacement-report", time.Now()
	must(store.CreateActivity(ctx, report))
	if _, err := task.NativeSourceReviewForAttempt(ctx, store, &d.Task, att.ID); err == nil {
		t.Fatal("approval survived report replacement")
	}
	if d.Status == task.TaskStatusDone {
		t.Fatal("source approval accepted task")
	}
}

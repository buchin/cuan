package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"strings"
	"testing"
	"time"
)

func testNativeVerifierRepair(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse) {
	t.Helper()
	var launchErr error
	ctx := context.WithValue(context.Background(), inboxReviewLaunchErrorKey{}, &launchErr)
	original := d.LatestAttempt.ID
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, original)
	if err != nil || previous == nil {
		t.Fatal("missing original inspection", err)
	}
	proof := *previous
	proof.VerifierRecoveryAttemptID = original
	proof.ObservedAt = time.Now().UTC()
	if err := task.ValidateInspectionVerifierRecovery(ctx, s.taskSvc.Store(), &d.Task, &proof); err != nil {
		t.Fatal("known native verifier mismatch not recoverable", err)
	}
	for _, bad := range []string{"target", "source", "observation", "revalidation", "preparation"} {
		altered := proof
		switch bad {
		case "target":
			altered.TargetCommit = proof.SourceCommit
		case "source":
			altered.SourceCommit = proof.TargetCommit
		case "observation":
			altered.ObservedAt = previous.ObservedAt
		case "revalidation":
			altered.PreviousAttemptID = "other"
		case "preparation":
			altered.PreparationLaunchID = "other"
		}
		if task.ValidateInspectionVerifierRecovery(ctx, s.taskSvc.Store(), &d.Task, &altered) == nil {
			t.Fatal("verifier repair expanded scope", bad)
		}
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "verifier-repair-blocker", ProjectID: d.ProjectID, Title: "Other work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	first := s.recoverLiveResultInspection(ctx, d, origin)
	second := s.recoverLiveResultInspection(ctx, d, origin)
	if !first || !second {
		t.Fatalf("bounded recovery not deduplicated first=%v second=%v direct=%v launchErr=%v", first, second, s.recoverInspectionVerifier(ctx, d, origin), launchErr)
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, proof.RequestID(d.ID)))
	if err != nil || launch.State != "queued" {
		t.Fatal("missing durable recovery", err)
	}
	bound, err := task.LiveInspectionForLaunch(&d.Task, launch)
	if err != nil || bound == nil || bound.VerifierRecoveryAttemptID != original || bound.PredecessorAttemptID() != original || bound.TargetCommit != previous.TargetCommit {
		t.Fatal("invalid repaired verifier scope", err)
	}
	if err := task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, bound); err != nil {
		t.Fatal("repair invalid at claim", err)
	}

	_ = s.taskSvc.Store().DeleteTask(ctx, "verifier-repair-blocker")
	reserved, err := s.taskSvc.Store().(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "repair-launch", launch.ID)
	if err != nil || reserved == nil {
		t.Fatal("repair launch unavailable", err)
	}
	attempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: launch.RequestID, ExpectedPredecessorID: bound.PredecessorAttemptID()})
	if err != nil {
		t.Fatal(err)
	}
	service := runner.NewService(runner.NewMemoryStore(s.taskSvc.Store()), s.taskSvc.Store(), s.projectSvc)
	service.SetEventBus(runner.NewEventBus())
	claimed, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsNativeVisualVerification: true, SupportsLiveInspection: true, SupportsLiveInspectionRecovery: true, SupportsCurrentOutcomeInspection: true}, time.Minute)
	if err != nil || !claimed.Claimed {
		t.Fatal("repair not claimable", err)
	}
	for _, stage := range []bool{false, true} {
		input := task.AgentReport{Progress: "Observed current revision", CommitSHA: &bound.TargetCommit}
		var reportErr error
		if stage {
			_, reportErr = s.taskSvc.StageAgentReport(ctx, d.ID, attempt.ID, d.AssignedAgent, input)
		} else {
			_, reportErr = s.taskSvc.ReportAgent(ctx, d.ID, attempt.ID, d.AssignedAgent, input)
		}
		if reportErr == nil {
			t.Fatal("inspection report overwrote implementation commit")
		}
		stored, _ := s.taskSvc.Store().GetTask(ctx, d.ID)
		if stored.CommitSHA != bound.SourceCommit {
			t.Fatal("rejected report mutated source identity")
		}
	}
	for _, event := range []*task.TaskActivity{
		{ID: "repair-kind", TaskID: d.ID, AttemptID: &attempt.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: time.Now()},
		{ID: "repair-report", TaskID: d.ID, AttemptID: &attempt.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Fresh checks passed", CreatedAt: time.Now()},
	} {
		if err := s.taskSvc.Store().CreateActivity(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	storedTask, _ := s.taskSvc.Store().GetTask(ctx, d.ID)
	latestBefore, _ := s.taskSvc.Store().GetLatestAttempt(ctx, d.ID)
	result, err := service.Complete(ctx, attempt.ID, runner.CompleteAttemptRequest{RunnerID: "live-runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, LiveInspectionCommit: bound.TargetCommit, CurrentOutcomeVerified: true, AgentReportRequired: true, AgentReportConfirmed: true, TestSummary: "Project verification: registered PHPUnit targets (exit 0)\nAll original checks passed", RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("c", 64), Final: strings.Repeat("c", 64)}})
	if err != nil || result == nil || result.Status != task.AttemptStatusSucceeded {
		t.Fatalf("repair completion rejected err=%v expected=%s claimed=%s taskStatus=%s latest=%s", err, attempt.ID, claimed.Attempt.ID, storedTask.Status, latestBefore.ID)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if s.recoverInspectionVerifier(ctx, after, origin) {
		t.Fatal("completed repair gained another cycle")
	}
	source, _ := s.taskSvc.GetAttempt(ctx, original)
	if source.Status != task.AttemptStatusFailed || source.ErrorMessage != "Live result inspection project checks did not pass" {
		t.Fatal("repair rewrote failed history")
	}
	testEvidenceOutcomeRevalidation(t, s, origin, after, service)
}

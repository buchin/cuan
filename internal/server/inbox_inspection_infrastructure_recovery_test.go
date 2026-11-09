package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func testInspectionInfrastructureRecovery(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse, claim *runner.ClaimTaskResponse, service *runner.RunnerService, mode string) {
	t.Helper()
	ctx := context.Background()
	store := s.taskSvc.Store()
	oldID := claim.Attempt.ID
	previous := *claim.LiveInspection
	failure := "Could not save task status before starting."
	if mode == "project deadline" {
		failure = task.InspectionProjectVerificationDeadline
		for _, event := range []*task.TaskActivity{
			{ID: "deadline-report", TaskID: d.ID, AttemptID: &oldID, ActivityType: "agent_report", Author: d.AssignedAgent, NewStatus: task.TaskStatusReview, CreatedAt: time.Now()},
			{ID: "deadline-kind", TaskID: d.ID, AttemptID: &oldID, ActivityType: "agent_result_kind", Author: d.AssignedAgent, NewStatus: "implementation", CreatedAt: time.Now()},
		} {
			if err := store.CreateActivity(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := service.Complete(ctx, oldID, runner.CompleteAttemptRequest{RunnerID: claim.Attempt.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 0, ErrorMessage: failure, TestSummary: "Project verification: test.sh --fast (exit -1)\nok config", RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("b", 64)}}); err != nil {
			t.Fatal(err)
		}
	} else if strings.HasPrefix(mode, "launch") {
		if _, err := s.taskSvc.UpdateAttemptStatus(ctx, oldID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: failure}); err != nil {
			t.Fatal(err)
		}
	} else {
		failure = task.InspectionDependencyFailurePrefix + ": npm ERR! ENOSPC"
		if _, err := service.Complete(ctx, oldID, runner.CompleteAttemptRequest{RunnerID: claim.Attempt.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: failure}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	blocked := task.TaskStatusBlocked
	if _, err := s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &blocked}); err != nil {
		t.Fatal(err)
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !task.InspectionInfrastructureFailure(d.LatestAttempt, d.Activities) {
		t.Fatal("real saved infrastructure failure not recognized")
	}
	s.cfg.CommitSHA = strings.Repeat("c", 40)
	proof := previous
	proof.VerifierRecoveryAttemptID, proof.TargetCommit, proof.ObservedAt = oldID, s.cfg.CommitSHA, time.Now().UTC()
	if err := task.ValidateLiveInspectionSource(ctx, store, &d.Task, &proof); err != nil {
		t.Fatal("fresh-target recovery rejected", err)
	}
	for _, bad := range []string{"source", "scope", "owner", "repository", "previous", "observation"} {
		altered := proof
		switch bad {
		case "source":
			altered.SourceAttemptID = oldID
		case "scope":
			altered.InstructionsHash = strings.Repeat("e", 64)
		case "owner":
			altered.OwnerID = "other"
		case "repository":
			altered.Repository = "other/repo"
		case "previous":
			altered.PreviousAttemptID = "other"
		case "observation":
			altered.ObservedAt = previous.ObservedAt
		}
		if task.ValidateLiveInspectionSource(ctx, store, &d.Task, &altered) == nil {
			t.Fatal("infrastructure retry expanded authority", bad)
		}
	}
	if err := store.CreateTask(ctx, &task.Task{ID: "inspection-recovery-blocker", ProjectID: d.ProjectID, Title: "Other work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	foreign := origin
	foreign.OwnerID = "other-owner"
	if s.recoverInspectionVerifier(ctx, d, foreign) {
		t.Fatal("recovery accepted foreign origin")
	}
	if !s.recoverLiveResultInspection(ctx, d, origin) || !s.recoverLiveResultInspection(ctx, d, origin) {
		t.Fatal("durable read-only recovery was not admitted/deduplicated")
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, proof.RequestID(d.ID)))
	if err != nil || launch == nil || launch.State != "queued" {
		t.Fatal("missing queued recovery", err)
	}
	bound, err := task.LiveInspectionForLaunch(&d.Task, launch)
	if err != nil || bound == nil || bound.TargetCommit != s.cfg.CommitSHA || bound.SourceAttemptID != previous.SourceAttemptID || bound.PredecessorAttemptID() != oldID {
		t.Fatal("recovery lost original source or fresh target", err)
	}
	_ = store.DeleteTask(ctx, "inspection-recovery-blocker")
	reserved, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "recovery-launch", launch.ID)
	if err != nil || reserved == nil {
		t.Fatal("cannot reserve recovery", err)
	}
	attempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: launch.RequestID, ExpectedPredecessorID: bound.PredecessorAttemptID(), InitializeQueuedTask: true, PreserveQueuedReport: true})
	if err != nil {
		t.Fatal("recovery invalid at admission", err)
	}
	next, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "recovery-worker", SupportsNativeVisualVerification: true, SupportsLiveInspection: true, SupportsLiveInspectionRecovery: true, SupportsCurrentOutcomeInspection: true}, time.Minute)
	if err != nil || !next.Claimed || next.Attempt.ID != attempt.ID || next.LiveInspection.TargetCommit != s.cfg.CommitSHA || !next.PublicationLimits.ReadOnlyRepository || !next.PublicationLimits.DenyPush || !next.PublicationLimits.DenyDeploy || next.Environment != "" || next.AutoDeploy {
		t.Fatalf("recovery invalid at claim: %+v %v", next, err)
	}
	if strings.HasSuffix(mode, " capacity") {
		testInspectionCapacityContinuation(t, s, origin, d, next, service)
		return
	}
	// A second infrastructure failure cannot renew the allowance, even after
	// another release. The failed original remains failed, with no manual Done.
	if _, err := s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Could not save task status before starting."}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &blocked}); err != nil {
		t.Fatal(err)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.cfg.CommitSHA = strings.Repeat("d", 40)
	if s.recoverInspectionVerifier(ctx, after, origin) {
		t.Fatal("recovery gained another retry cycle")
	}
	old, err := store.GetAttempt(ctx, oldID)
	if err != nil || old.Status != task.AttemptStatusFailed || old.ErrorMessage != failure || after.Status == task.TaskStatusDone || after.CommitSHA != d.CommitSHA || after.Instructions != d.Instructions || len(after.DoDChecklist) != len(d.DoDChecklist) {
		t.Fatal("recovery replaced failed history, scope or acceptance", err)
	}
}

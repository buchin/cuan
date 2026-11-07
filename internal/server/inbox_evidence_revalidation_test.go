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

func testEvidenceOutcomeRevalidation(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse, service *runner.RunnerService) {
	t.Helper()
	ctx := context.Background()
	store := s.taskSvc.Store()
	parentID, sourceCommit := d.LatestAttempt.ID, d.CommitSHA
	if err := s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	blocked := task.TaskStatusBlocked
	if _, err := s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &blocked}); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	s.observeInboxPublicationConfiguration(ctx, d, origin)
	s.observeInboxGitHubConnection(ctx, d, origin)
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	q := origin
	q.RequestID = db.GenerateUUID()
	decision := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "RoomCard screenshot missing", Explanation: "Generic task cards do not prove this component", Revision: "Capture the actual RoomCard at390px and desktop, including navigation. Do not change source.", ManagerAction: "replan", ManagerReason: "Use targeted real-component fixtures instead of unrelated screenshots."}
	if _, err := s.saveReviewAssessment(ctx, d, q, decision); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err := store.CreateTask(ctx, &task.Task{ID: "evidence-blocker", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	if !s.recoverLiveResultInspection(ctx, d, origin) {
		t.Fatal("missing evidence was not admitted natively")
	}
	launch, err := s.taskSvc.LatestLaunch(ctx, d.ID)
	if err != nil || launch == nil {
		t.Fatal("missing native evidence launch", err)
	}
	proof, err := task.LiveInspectionForLaunch(&d.Task, launch)
	if err != nil || proof == nil || proof.RevalidationKind != task.EvidenceRevalidation || proof.PreviousAttemptID != parentID || proof.TargetCommit != proof.PreviousCommit || proof.SourceCommit != sourceCommit || proof.ReviewRequestID != q.RequestID || !strings.Contains(launch.Revision, decision.Revision) {
		t.Fatal("evidence continuation lost exact target/source/review gap", err, proof)
	}
	pathMode := *proof
	pathMode.RevalidationKind = ""
	if pathMode.RequestID(d.ID) != proof.RequestID(d.ID) {
		t.Fatal("evidence mode obtained another allowance")
	}
	if !s.recoverLiveResultInspection(ctx, d, origin) {
		t.Fatal("evidence admission did not deduplicate")
	}
	_ = store.DeleteTask(ctx, "evidence-blocker")
	if _, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "evidence-launch", launch.ID); err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: launch.RequestID, ExpectedPredecessorID: proof.PredecessorAttemptID()})
	if err != nil {
		t.Fatal(err)
	}
	req := runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsNativeVisualVerification: true, SupportsLiveInspection: true, SupportsLiveInspectionRecovery: true, SupportsCurrentOutcomeInspection: true}
	old, err := service.Claim(ctx, req, time.Minute)
	if err != nil || old.Claimed {
		t.Fatal("old runner reserved unsupported evidence mode", err)
	}
	req.SupportsEvidenceRevalidation = true
	claim, err := service.Claim(ctx, req, time.Minute)
	if err != nil || !claim.Claimed || claim.LiveInspection == nil || claim.LiveInspection.RevalidationKind != task.EvidenceRevalidation || !claim.PublicationLimits.ReadOnlyRepository || !claim.PublicationLimits.DenyPush || !claim.PublicationLimits.DenyDeploy || claim.AutoDeploy || claim.Environment != "" {
		t.Fatal("native evidence claim lost scope", err)
	}
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "targeted-evidence-report", TaskID: d.ID, AttemptID: &a.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Targeted original criteria inspected", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_, err = service.Complete(ctx, a.ID, runner.CompleteAttemptRequest{RunnerID: "live-runner", Status: task.AttemptStatusSucceeded, LiveInspectionCommit: proof.TargetCommit, CurrentOutcomeVerified: true, AgentReportRequired: true, AgentReportConfirmed: true, TestSummary: "Project verification: test.sh --fast (exit 0)\nTargeted checks", RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("d", 64), Final: strings.Repeat("d", 64)}})
	if err != nil {
		t.Fatal("native evidence completion failed", err)
	}
	if err := s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if after.LatestAttempt.Status != task.AttemptStatusSucceeded || after.CommitSHA != sourceCommit || after.Status == task.TaskStatusDone || s.liveInspectionReviewContext(ctx, after) == nil {
		t.Fatal("evidence completion rewrote history or bypassed acceptance")
	}
	after.Status = task.TaskStatusBlocked
	s.cfg.CommitSHA = strings.Repeat("e", 40)
	if s.recoverLiveResultInspection(ctx, after, origin) {
		t.Fatal("deployment renewed evidence allowance")
	}
}

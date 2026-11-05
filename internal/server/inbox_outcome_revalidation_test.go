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

// Completes the real launch/claim/runner-evidence fixture before exercising a
// changed-source follow-up. No production task or budget is mutated by the test.
func testChangedOutcomeRevalidation(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse, service *runner.RunnerService, commits []string) {
	t.Helper()
	ctx := context.Background()
	store := s.taskSvc.Store()
	if len(commits) != 3 {
		t.Fatal("missing actual revision fixture")
	}
	if err := s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped"); err != nil {
		t.Fatal("finish prior launch", err)
	}
	blocked := task.TaskStatusBlocked
	if _, err := s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &blocked}); err != nil {
		t.Fatal("mark rejected result", err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	q := origin
	q.RequestID = db.GenerateUUID()
	assessment := inboxReviewEvaluation{Decision: "revise", RemainingWork: "implementation", Finding: "Terminal reconciliation is still missing.", Explanation: "Needs correction", Revision: "Fix internal/server/terminal_handlers.go, then verify every original criterion."}
	if _, err := s.saveReviewAssessment(ctx, d, q, assessment); err != nil {
		t.Fatal("save prior assessment", err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	for _, sha := range commits[:2] {
		s.cfg.CommitSHA = sha
		if s.recoverLiveResultInspection(ctx, d, origin) {
			t.Fatal("same or unrelated deployment granted another inspection")
		}
	}
	s.cfg.CommitSHA = commits[2]
	canceled := origin
	canceled.State = "cancelled"
	if s.recoverLiveResultInspection(ctx, d, canceled) {
		t.Fatal("canceled approval granted revalidation")
	}
	foreign := origin
	foreign.OwnerID = "foreign"
	if s.recoverLiveResultInspection(ctx, d, foreign) {
		t.Fatal("foreign owner granted revalidation")
	}
	if err := store.CreateTask(ctx, &task.Task{ID: "revalidation-blocker", ProjectID: d.ProjectID, Title: "Other work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	if !s.recoverLiveResultInspection(ctx, d, origin) {
		t.Fatal("changed rejected source was not revalidated")
	}
	request, paths := task.OutcomeRevalidationReview(&d.Task, d.LatestAttempt, d.Activities, "")
	proof, err := task.LiveInspectionForAttempt(ctx, store, &d.Task, d.LatestAttempt.ID)
	if err != nil || proof == nil {
		t.Fatal("read original inspection", err)
	}
	next := *proof
	next.PreviousAttemptID = d.LatestAttempt.ID
	next.PreviousCommit = commits[0]
	next.TargetCommit = commits[2]
	next.ReviewRequestID = request
	next.ReviewPaths = paths
	next.ObservedAt = time.Now().UTC()
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, next.RequestID(d.ID)))
	if err != nil || launch == nil {
		t.Fatal("missing durable revalidation", err)
	}
	bound, err := task.LiveInspectionForLaunch(&d.Task, launch)
	if err != nil || bound.PreviousAttemptID != d.LatestAttempt.ID || bound.SourceAttemptID != proof.SourceAttemptID || bound.ReviewRequestID != q.RequestID {
		t.Fatal("revalidation lost rejected-review identity", err)
	}
	before := len(d.Attempts)
	if !s.recoverLiveResultInspection(ctx, d, origin) {
		t.Fatal("queued revalidation not recognized")
	}
	same, _ := s.taskSvc.GetLaunch(ctx, launch.ID)
	if same.ID != launch.ID {
		t.Fatal("duplicate launch")
	}
	if err := store.DeleteTask(ctx, "revalidation-blocker"); err != nil {
		t.Fatal(err)
	}
	claimedLaunch, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "revalidation-launch", launch.ID)
	if err != nil {
		t.Fatal("claim revalidation", err)
	}
	attempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: launch.RequestID})
	if err != nil {
		t.Fatal("create revalidation attempt", err)
	}
	old, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsLiveInspection: true, SupportsCurrentOutcomeInspection: true}, time.Minute)
	if err != nil || old.Claimed {
		t.Fatal("worker without recovery support claimed revalidation", err)
	}
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsLiveInspection: true, SupportsLiveInspectionRecovery: true, SupportsCurrentOutcomeInspection: true}, time.Minute)
	if err != nil || !claim.Claimed || claim.LiveInspection == nil || !claim.PublicationLimits.ReadOnlyRepository || claim.AutoDeploy || claim.Environment != "" {
		t.Fatal("revalidation lost native scope isolation", err)
	}
	beforeMessages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, origin.RoomID)
	s.executeQueuedTaskLaunch(ctx, *claimedLaunch, true)
	reconciled, err := s.taskSvc.GetLaunch(ctx, launch.ID)
	if err != nil || reconciled.State != "started" || reconciled.SafeError != "" {
		t.Fatal("same active attempt was treated as stale", err)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if len(current.Attempts) != before+1 || current.LatestAttempt.ID != attempt.ID || current.LatestAttempt.Status != task.AttemptStatusRunning {
		t.Fatal("lost launch response created or stopped an attempt")
	}
	afterMessages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, origin.RoomID)
	if len(afterMessages) != len(beforeMessages) {
		t.Fatal("launch reconstruction emitted a false failure bubble")
	}
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "revalidated-report", TaskID: d.ID, AttemptID: &attempt.ID, ActivityType: "agent_report", Author: origin.AgentID, NewStatus: task.TaskStatusReview, Message: "Exact changed revision checked", CreatedAt: time.Now()}); err != nil {
		t.Fatal("save new final report", err)
	}
	req := runner.CompleteAttemptRequest{RunnerID: "live-runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, LiveInspectionCommit: commits[2], CurrentOutcomeVerified: true, AgentReportRequired: true, AgentReportConfirmed: true, TestSummary: "Project verification: test.sh --fast (exit 0)\nFresh verification", RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("c", 64), Final: strings.Repeat("c", 64)}}
	if _, err := service.Complete(ctx, attempt.ID, req); err != nil {
		t.Fatal("complete new inspection", err)
	}
	if err := s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped"); err != nil {
		t.Fatal("finish new launch", err)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if len(after.Attempts) != before+1 || after.LatestAttempt.Status != task.AttemptStatusSucceeded || s.liveInspectionReviewContext(ctx, after) == nil {
		_, proofErr := task.ValidateLiveInspection(ctx, store, &after.Task, after.LatestAttempt.ID)
		t.Fatalf("fresh result evidence missing: attempts=%d expected=%d status=%s proof_error=%v", len(after.Attempts), before+1, after.LatestAttempt.Status, proofErr)
	}
	if after.Status == task.TaskStatusDone {
		t.Fatal("revalidation bypassed original acceptance")
	}
	after.Status = blocked
	s.cfg.CommitSHA = strings.Repeat("f", 40)
	if s.recoverLiveResultInspection(ctx, after, origin) {
		t.Fatal("another deploy renewed the one-time revalidation")
	}
	original, err := store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil || original.Status != task.AttemptStatusFailed {
		t.Fatal("original failed attempt was rewritten")
	}
}

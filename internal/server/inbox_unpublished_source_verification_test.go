package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

func unpublishedSourceVerificationFixture(t *testing.T, failure ...string) (*Server, *task.TaskDetailResponse, db.InboxRequest, inboxReviewEvaluation) {
	t.Helper()
	s, d := managerFixture(t)
	ctx := context.Background()
	store := s.taskSvc.Store()
	now := time.Now().UTC()
	row, _ := store.GetTask(ctx, d.ID)
	row.CommitSHA = strings.Repeat("a", 40)
	row.Status = task.TaskStatusBlocked
	if err := store.UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	started, finished := now, now
	source := &task.TaskAttempt{ID: "unpublished-source", AttemptNumber: 2, TaskID: d.ID, RunnerID: "runner", Status: task.AttemptStatusFailed, ExitCode: new(int), ErrorMessage: task.PublicationVerificationUnavailableMessage, CreatedAt: started, StartedAt: &started, FinishedAt: &finished}
	if len(failure) > 0 {
		source.ErrorMessage = failure[0]
	}
	if err := store.CreateAttempt(ctx, source); err != nil {
		t.Fatal(err)
	}
	native, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: registered PHPUnit targets (exit 0)\nActual checks passed", Changes: task.NewRunnerChangeRecord(row.CommitSHA, "tests/Feature/ContractTest.php | 1 -")})
	for _, event := range []*task.TaskActivity{
		{ID: "unpublished-report", TaskID: d.ID, AttemptID: &source.ID, ActivityType: "agent_report", NewStatus: "review", Message: "Actual result; independent checks and publication pending", CreatedAt: finished},
		{ID: "unpublished-kind", TaskID: d.ID, AttemptID: &source.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: finished},
		{ID: "verification:" + source.ID, TaskID: d.ID, AttemptID: &source.ID, ActivityType: task.ActivityTypeRunnerVerification, Author: source.RunnerID, Message: string(native), CreatedAt: finished},
	} {
		if err := store.CreateActivity(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	s.queueInboxTaskReview(ctx, d)
	requests := reviewRequests(t, s, d.SourceRoomID)
	q := requests[len(requests)-1]
	decision := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Verify the tested source before publication", Explanation: "Independent source verification remains", Revision: "Check original criteria against the exact saved source without publication", ManagerAction: "continue", ManagerReason: "Source checks passed; independent source verification remains"}
	if _, err := s.saveReviewAssessment(ctx, d, q, decision); err != nil {
		t.Fatal(err)
	}
	if task.ProjectChecksPassed(d) || !task.VerificationSourceChecksPassed(d) {
		t.Fatal("publication failure became execution success or blocked tested source")
	}
	return s, d, q, decision
}

func TestNativeMissingReceiptSourceRetainsIndependentVerificationHandoff(t *testing.T) {
	s, d, q, decision := unpublishedSourceVerificationFixture(t, task.PublicationReceiptRequiredMessage)
	ctx := context.Background()
	if notice, ok := s.startVerificationFollowup(ctx, q, d, decision); !ok {
		t.Fatal("native receipt-only failure stranded saved source verification", notice)
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA, d.LatestAttempt.ID)))
	if err != nil || launch == nil {
		t.Fatal("missing independent source launch", err)
	}
	proof, err := task.VerificationIntentForLaunch(&d.Task, launch)
	if err != nil || proof == nil || proof.SourceAttemptID != d.LatestAttempt.ID || proof.SourceCommit != d.CommitSHA || !proof.IsolatedWorkspace {
		t.Fatal("source identity or isolation changed", err, proof)
	}
	old, _ := s.taskSvc.Store().GetAttempt(ctx, d.LatestAttempt.ID)
	if old.Status != task.AttemptStatusFailed || old.ErrorMessage != task.PublicationReceiptRequiredMessage {
		t.Fatal("source verification admission rewrote failure history")
	}
}

func TestTestedUnpublishedSourceGetsExactReadOnlyVerifier(t *testing.T) {
	s, d, q, decision := unpublishedSourceVerificationFixture(t)
	ctx := context.Background()
	store, row, source := s.taskSvc.Store(), &d.Task, d.LatestAttempt
	if err := store.CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if notice, ok := s.startVerificationFollowup(ctx, q, d, decision); !ok {
			t.Fatal("tested source not admitted", notice)
		}
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, row.CommitSHA, source.ID)))
	if err != nil || launch == nil || launch.State != "queued" {
		t.Fatal("missing exact-source launch", err, launch)
	}
	proof, err := task.VerificationIntentForLaunch(&d.Task, launch)
	if err != nil || proof == nil || !proof.IsolatedWorkspace || proof.SourceAttemptID != source.ID || proof.SourceCommit != row.CommitSHA {
		t.Fatal("source replaced with another release", err, proof)
	}
	limits := executionpolicy.FromInstructions(launch.Revision)
	if !limits.ReadOnlyRepository || !limits.DenyDeploy || !limits.DenyPush {
		t.Fatal("verification granted publication", limits)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	old, _ := store.GetAttempt(ctx, source.ID)
	if old.Status != task.AttemptStatusFailed || after.Status == task.TaskStatusDone || len(after.Attempts) != len(d.Attempts) {
		t.Fatal("admission rewrote history or accepted work")
	}
}

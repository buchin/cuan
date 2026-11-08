package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/buchin/cuan/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestVerificationFollowupQueuesOnceWithoutResettingCorrectionBudget(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	verificationFollowupQueueContract(t, s, d, q)
}
func TestVerificationFollowupPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, q, d := phaseLineageSetup(t, database)
		verificationFollowupQueueContract(t, s, d, q)
	})
}

func TestVerificationFollowupDoesNotReuseAnotherSourceAtSameCommit(t *testing.T) {
	for _, sqlStore := range []bool{false, true} {
		t.Run(fmt.Sprint("postgres=", sqlStore), func(t *testing.T) {
			run := func(s *Server, d *task.TaskDetailResponse, q db.InboxRequest) {
				ctx := context.Background()
				commit := strings.Repeat("a", 40)
				prior, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: task.VerificationRequestID(d.ID, commit), OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: q.RequestID, AgentID: q.AgentID, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), Revision: "Preserve this previous source verification admission", CreatedAt: time.Now(), AvailableAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
				if err != nil {
					t.Fatal(err)
				}
				_ = s.taskSvc.Store().DeleteTask(ctx, "phase-blocker")
				row, err := s.taskSvc.Store().GetTask(ctx, d.ID)
				if err != nil {
					t.Fatal(err)
				}
				row.Status = task.TaskStatusReview
				if err := s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
					t.Fatal(err)
				}
				claimed, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "old-source-review", prior.ID)
				if err != nil || claimed == nil {
					t.Fatal("old verification fixture was not claimed", err)
				}
				if err := s.taskSvc.FinishLaunch(ctx, prior.ID, "old-source-review", "stopped", time.Now(), "Previous source review completed"); err != nil {
					t.Fatal(err)
				}
				prior, err = s.taskSvc.GetLaunch(ctx, prior.ID)
				if err != nil {
					t.Fatal(err)
				}
				verificationFollowupQueueContract(t, s, d, q)
				after, err := s.taskSvc.GetLaunch(ctx, prior.ID)
				if err != nil || after.Revision != prior.Revision || after.State != prior.State {
					t.Fatal("new source rewrote the old verification admission", after, err)
				}
			}
			if sqlStore {
				withPhasePostgres(t, func(database *sql.DB) {
					s, q, d := phaseLineageSetup(t, database)
					run(s, d, q)
				})
			} else {
				s, d, q := stalledReviewFixture(t, "completed")
				run(s, d, q)
			}
		})
	}
}
func verificationFollowupQueueContract(t *testing.T, s *Server, d *task.TaskDetailResponse, q db.InboxRequest, commits ...string) {
	t.Helper()
	ctx := context.Background()
	store := s.taskSvc.Store()
	row, err := store.GetTask(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now()
	for _, event := range []struct{ id, kind, state string }{{"final", "agent_report", "review"}, {"kind", "agent_result_kind", "implementation"}} {
		if err = store.CreateActivity(ctx, &task.TaskActivity{ID: event.id, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: event.kind, NewStatus: event.state, Message: "Actual implementation report", CreatedAt: stamp}); err != nil {
			t.Fatal(err)
		}
	}
	row.Status = task.TaskStatusReview
	row.Summary = "Implementation ready"
	row.CompleteNote = "Actual checks passed; screenshot evidence pending"
	row.CommitSHA = strings.Repeat("a", 40)
	if len(commits) > 0 {
		row.CommitSHA = commits[0]
	}
	if err = store.UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"old-one", "old-two"} {
		if err = store.CreateAttempt(ctx, &task.TaskAttempt{ID: id, TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: test.sh --fast (exit 0)"})
	if err = store.CreateActivity(ctx, &task.TaskActivity{ID: "source-checks", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityTypeRunnerVerification, Author: d.LatestAttempt.RunnerID, Message: string(raw), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err = store.CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	assessment := inboxReviewEvaluation{Decision: "revise", Finding: "Screenshot aplikasi belum tersedia.", RemainingWork: "verification", Explanation: "Evidence missing.", Revision: "Capture actual application at390px and desktop and inspect."}
	if _, err = s.saveReviewAssessment(ctx, d, q, assessment); err != nil {
		t.Fatal(err)
	}
	// A storage outage must retain the saved assessment and reuse the same intent.
	failing := reviewLaunchFailureStore{Store: store, LaunchStore: store.(task.LaunchStore), failure: errors.New("temporary launch outage")}
	s.taskSvc = task.NewService(failing, s.projectSvc)
	if outcome, ok := s.startVerificationFollowup(ctx, q, d, assessment); ok || strings.Contains(outcome, "verifikasi tambahan") {
		t.Fatal("promised unsaved verification")
	}
	s.taskSvc = task.NewService(store, s.projectSvc)
	for i := 0; i < 3; i++ {
		outcome, ok := s.startVerificationFollowup(ctx, q, d, assessment)
		if !ok || !strings.Contains(outcome, "verifikasi tambahan") {
			t.Fatalf("not queued %v %s", ok, outcome)
		}
	}
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA, d.LatestAttempt.ID)))
	if err != nil || job == nil || job.State != "queued" {
		t.Fatalf("missing durable intent %+v %v", job, err)
	}
	proof, err := task.VerificationIntentForLaunch(&d.Task, job)
	if err != nil || proof == nil || proof.SourceAttemptID != d.LatestAttempt.ID {
		t.Fatal("invalid source proof", err)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || len(after.Attempts) != 3 {
		t.Fatal("created attempt behind busy project", err)
	}
	if taskRevisionAttemptCount(s.reviewBudgetDetail(ctx, after)) < 3 {
		t.Fatal("reset correction budget")
	}
	for _, kind := range []string{"implementation", "external"} {
		rejected := assessment
		rejected.RemainingWork = kind
		if _, ok := s.startVerificationFollowup(ctx, q, d, rejected); ok {
			t.Fatal("accepted nonverification work")
		}
	}
	canceled := *d
	canceled.Status = "cancelled"
	if _, ok := s.startVerificationFollowup(ctx, q, &canceled, assessment); ok {
		t.Fatal("revived retired task")
	}
}

func TestVerificationReviewRequiresBackendSourceReceipt(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	verificationReviewReceiptContract(t, s, d, q)
}
func TestVisualAcceptanceAuditPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, q, d := phaseLineageSetup(t, database)
		verificationReviewReceiptContract(t, s, d, q)
	})
}
func verificationReviewReceiptContract(t *testing.T, s *Server, d *task.TaskDetailResponse, q db.InboxRequest) {
	t.Helper()
	verificationFollowupQueueContract(t, s, d, q)
	ctx := context.Background()
	store := s.taskSvc.Store()
	_ = store.DeleteTask(ctx, "phase-blocker")
	if err := store.DeleteTask(ctx, "busy"); err != nil {
		t.Fatal(err)
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA, d.LatestAttempt.ID)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "verifier-launch", job.ID); err != nil {
		t.Fatal(err)
	}
	att, err := s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: "verifier", AgentName: d.AssignedAgent, RequestID: job.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	service := runner.NewService(runner.NewMemoryStore(store), store, nil)
	service.SetEventBus(runner.NewEventBus())
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "verifier"}, time.Minute)
	if err != nil || !claim.Claimed {
		t.Fatal("claim failed", err)
	}
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.verificationReviewContext(ctx, d) != nil {
		t.Fatal("running verifier supplied source acceptance context")
	}
	_, err = service.Complete(ctx, att.ID, runner.CompleteAttemptRequest{RunnerID: "verifier", Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: d.CommitSHA, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("b", 64), Final: strings.Repeat("b", 64)}})
	if err != nil {
		t.Fatal(err)
	}
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	evidence := s.verificationReviewContext(ctx, d)
	if evidence == nil || evidence["source_attempt_id"] == att.ID || evidence["source_commit"] != d.CommitSHA || evidence["repository_unchanged"] != true {
		t.Fatal("missing source context", evidence)
	}
	d.ExecutionCategory = task.ExecutionCategoryCode
	if decision := s.requireIndependentCodeReview(ctx, d, inboxReviewEvaluation{Decision: "pass"}); decision.Decision != "pass" {
		t.Fatal("valid independent execution remained blocked", decision)
	}
	changed := *d
	changed.CommitSHA = strings.Repeat("c", 40)
	if decision := s.requireIndependentCodeReview(ctx, &changed, inboxReviewEvaluation{Decision: "pass"}); decision.Decision != "revise" {
		t.Fatal("old verifier accepted a changed source commit", decision)
	}
	if task.AttemptVerificationEvidence(d).Current == evidence["source_project_checks"] {
		t.Fatal("historical source tests attributed to verifier")
	}
	// Model reports once accepted without reading delivered images must return
	// to review automatically, preserving source, artifacts and execution count.
	publishReviewFixtureImage(t, s, d)
	q.RequestID = db.GenerateUUID()
	if _, err = s.saveReviewAssessment(ctx, d, q, inboxReviewEvaluation{Decision: "pass", Explanation: "Old text-only acceptance", RemainingWork: "none"}); err != nil {
		t.Fatal(err)
	}
	row, err := store.GetTask(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	row.Status = task.TaskStatusDone
	if err = store.UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	candidates, err := store.(task.VerificationAcceptanceAuditStore).UninspectedVerificationTaskIDs(ctx)
	if err != nil || len(candidates) != 1 {
		t.Fatal("missing exact accepted verifier candidate", candidates, err)
	}
	s.reconcileVisualAcceptances(ctx)
	s.reconcileVisualAcceptances(ctx)
	audited, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || audited.Status != task.TaskStatusReview || audited.CommitSHA != d.CommitSHA || len(audited.Attempts) != 4 {
		t.Fatal("audit altered execution or accepted unchecked images", err)
	}
	candidates, err = store.(task.VerificationAcceptanceAuditStore).UninspectedVerificationTaskIDs(ctx)
	if err != nil || len(candidates) != 0 {
		t.Fatal("completed audit scanned repeatedly", candidates, err)
	}
	for _, a := range d.Activities {
		if a.ActivityType == "verification_source_verified" {
			a.Author = "agent"
		}
	}
	if s.verificationReviewContext(ctx, d) != nil {
		t.Fatal("agent could supply backend source receipt")
	}
	if decision := s.requireIndependentCodeReview(ctx, d, inboxReviewEvaluation{Decision: "pass"}); decision.Decision != "revise" {
		t.Fatal("agent-authored receipt bypassed independent gate", decision)
	}
}

package server

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestLiveResultInspectionLifecycle(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong commit", "missing ancestry", "failed checks", "changed repository", "missing report", "cancelled", "foreign owner", "different repository", "checks forbidden", "release changed"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, scenario, nil)
		})
	}
}

func TestLiveResultInspectionPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, origin, d := phaseLineageSetup(t, database)
		liveInspectionContract(t, s, origin, d, "valid", database)
	})
}

func liveInspectionContract(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse, scenario string, database *sql.DB) {
	t.Helper()
	ctx := context.Background()
	store := s.taskSvc.Store()
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: d.ProjectID, Name: "Fixture", RepoURL: "https://github.com/fixture/repo"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "fixture", t.TempDir())
	s.taskSvc = task.NewService(store, s.projectSvc)
	s.cfg.Environment, s.cfg.SourceRepository, s.cfg.CommitSHA = "production", "fixture/repo", strings.Repeat("b", 40)
	d.Status, d.CommitSHA, d.ResultKind = task.TaskStatusBlocked, strings.Repeat("a", 40), "implementation"
	d.Instructions = "Cuan managed delegation v1.\nImplement the approved feature, run tests and verify its live result."
	d.Summary, d.CompleteNote = "Prior implementation report", "Prior reported evidence; source project checks failed"
	d.DoDChecklist = []task.DoDItem{{Text: "Feature and checks available", Completed: true, Evidence: "Prior claim for independent rechecking"}}
	now, exit := time.Now().UTC(), 0
	source := &task.TaskAttempt{ID: "live-source", TaskID: d.ID, RunnerID: "old-runner", AgentName: d.AssignedAgent, Status: task.AttemptStatusFailed, ExitCode: &exit, ErrorMessage: "Project verification failed", CreatedAt: now, FinishedAt: &now}
	if scenario == "cancelled" {
		source.CancelRequested = true
	}
	if scenario == "foreign owner" {
		origin.OwnerID = "other"
	}
	if scenario == "different repository" {
		s.cfg.SourceRepository = "other/repo"
	}
	if scenario == "checks forbidden" {
		d.Instructions += "\nDo not run the test suite."
	}
	if err := store.UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAttempt(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "live-source-report", TaskID: d.ID, AttemptID: &source.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusDone, Message: "Implementation report", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "live-source-kind", TaskID: d.ID, AttemptID: &source.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", Message: "implementation", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	eligible := scenario != "cancelled" && scenario != "foreign owner" && scenario != "different repository" && scenario != "checks forbidden"
	if got := s.recoverLiveResultInspection(ctx, d, origin); got != eligible {
		t.Fatal("incorrect live inspection eligibility", got, scenario)
	}
	if !eligible {
		return
	}
	id := task.LiveInspectionRequestID(d.ID, d.CommitSHA, task.LaunchInstructionsHash(d.Instructions))
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil || launch.State != "queued" || launch.SourceRequestID != origin.RequestID {
		t.Fatal("missing scope-bound durable inspection", err)
	}
	if !s.recoverLiveResultInspection(ctx, d, origin) {
		t.Fatal("inspection did not deduplicate")
	}
	_ = store.DeleteTask(ctx, "phase-blocker")
	claimed, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "live-launch", launch.ID)
	if err != nil || claimed == nil {
		t.Fatal("could not claim the existing launch", err)
	}
	inspectionAttempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: id})
	if err != nil {
		t.Fatal(err)
	}
	var workerStore runner.Store = runner.NewMemoryStore(store)
	if database != nil {
		workerStore = runner.NewDBStore(database)
	}
	service := runner.NewService(workerStore, store, s.projectSvc)
	oldWorker, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner"}, time.Minute)
	if err != nil || oldWorker.Claimed {
		t.Fatal("unsupported worker consumed inspection", err)
	}
	queued, _ := store.GetLatestAttempt(ctx, d.ID)
	if queued.Status != task.AttemptStatusQueued {
		t.Fatal("unsupported worker changed the queued attempt")
	}
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsLiveInspection: true}, time.Minute)
	if err != nil || !claim.Claimed || claim.LiveInspection == nil || !claim.PublicationLimits.ReadOnlyRepository || claim.Environment != "" || claim.AutoDeploy {
		t.Fatal("inspection lost native routing/scope/environment restrictions", err)
	}
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "live-current-report", TaskID: d.ID, AttemptID: &inspectionAttempt.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Current feature verified", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	req := runner.CompleteAttemptRequest{RunnerID: "live-runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, LiveInspectionCommit: s.cfg.CommitSHA, SourceAncestryVerified: true, AgentReportRequired: true, AgentReportConfirmed: true, TestSummary: "Project verification: test.sh --fast (exit 0)\nFresh checks", RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("c", 64), Final: strings.Repeat("c", 64)}}
	switch scenario {
	case "wrong commit":
		req.LiveInspectionCommit = strings.Repeat("d", 40)
	case "missing ancestry":
		req.SourceAncestryVerified = false
	case "failed checks":
		req.TestSummary = "Project verification: test.sh --fast (exit 1)\nFailed"
	case "changed repository":
		req.RepositoryVerification.Final = strings.Repeat("d", 64)
	case "missing report":
		req.AgentReportConfirmed = false
	}
	completed, err := service.Complete(ctx, inspectionAttempt.ID, req)
	valid := scenario == "valid" || scenario == "release changed"
	if err != nil || (completed.Status == task.AttemptStatusSucceeded) != valid {
		t.Fatal("invalid live evidence was accepted", completed, err)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	old, _ := store.GetAttempt(ctx, source.ID)
	if old.Status != task.AttemptStatusFailed || after.Status == task.TaskStatusDone || after.CommitSHA != d.CommitSHA {
		t.Fatal("inspection rewrote source history or bypassed independent acceptance")
	}
	if scenario == "release changed" {
		s.cfg.CommitSHA = strings.Repeat("d", 40)
	}
	if got := s.liveInspectionReviewContext(ctx, after); (got != nil) != (scenario == "valid") {
		t.Fatal("review used absent/stale/failed result evidence", got)
	}
	if scenario == "release changed" {
		key := "task-review:" + inspectionAttempt.ID
		message := &db.ProjectMessage{ID: delegationMessageID(key + ":input"), RoomID: origin.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [/tasks/" + d.ID + "]", CreatedAt: time.Now().UTC()}
		if err := s.projectStudioRepo.CreateMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
		q := db.InboxRequest{OwnerID: origin.OwnerID, AgentID: origin.AgentID, RoomID: origin.RoomID, RequestID: delegationMessageID(key), MessageID: message.ID, ReplyID: delegationMessageID(key + ":reply"), CreatedAt: time.Now().UTC(), Deadline: time.Now().Add(time.Hour)}
		s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","finding":"All criteria verified.","explanation":"Accept result","revision":""}`)
		current, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
		if err != nil || current.Status == task.TaskStatusDone {
			t.Fatal("model pass bypassed changed running release", err)
		}
	}
}

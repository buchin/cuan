package server

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

func TestDocumentationEvidenceRecovery(t *testing.T) {
	for _, scenario := range []string{"valid", "cancelled", "foreign owner", "missing approval", "code verifier", "process failure", "missing report", "later blocked report", "missing acceptance"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, _ := stalledReviewFixture(t, "completed", task.AttemptStatusFailed)
			documentationEvidenceRecoveryContract(t, s, d, scenario)
		})
	}
}

func TestDocumentationEvidenceRecoveryPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, _, d := phaseLineageSetup(t, database)
		documentationEvidenceRecoveryContract(t, s, d, "valid")
	})
}

func documentationEvidenceRecoveryContract(t *testing.T, s *Server, d *task.TaskDetailResponse, scenario string) {
	t.Helper()
	ctx := context.Background()
	requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		t.Fatal(err)
	}
	var origin db.InboxRequest
	for _, q := range requests {
		if q.MessageID == d.SourceMessageID {
			origin = q
		}
	}
	d.Status = task.TaskStatusBlocked
	if !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") {
		d.Instructions = "Cuan managed delegation v1.\n" + d.Instructions
	}
	d.Summary, d.CompleteNote = "Previous investigation findings", "Previous evidence to be inspected independently"
	d.TestSummary = "Markdown-only diff verified: 2 readable nonempty UTF-8 file(s), 0 deletion(s). No code/config changes detected against main. Task-specific content and delivery criteria still require evidence."
	d.DoDChecklist = []task.DoDItem{{Text: "Read and deliver the investigation findings", Completed: true, Evidence: "Historical claim, not acceptance"}}
	newSource := d.LatestAttempt.Status != task.AttemptStatusFailed
	if newSource {
		copy := *d.LatestAttempt
		copy.ID = "documentation-source"
		copy.CreatedAt = copy.CreatedAt.Add(time.Second)
		d.LatestAttempt = &copy
	}
	exit := 0
	finished := time.Now().Add(-time.Minute)
	d.LatestAttempt.Status = task.AttemptStatusFailed
	d.LatestAttempt.ExitCode = &exit
	d.LatestAttempt.FinishedAt = &finished
	d.LatestAttempt.ErrorMessage = "Kepatuhan auto_deploy: missing live deployment"
	if newSource {
		if err := s.taskSvc.Store().CreateAttempt(ctx, d.LatestAttempt); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"docs-prior-one", "docs-prior-two"} {
		if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: id, TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	report := &task.TaskActivity{ID: "docs-final", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusDone, Message: "Previous investigation report", CreatedAt: finished}
	d.Activities = append(d.Activities, report)
	if strings.HasPrefix(scenario, "scope ") {
		d.TestSummary = "No supported project verifier was available; agent report evidence retained."
		d.Instructions = "Cuan managed delegation v1.\nClone the connected repository, verify its structure and prepare initial configuration.\nApproval message: owner-message\nUser authorization: Prepare the project.\nConversation: 00000000-0000-4000-8000-000000000001\nReport actual progress and verification evidence. Stay within this scope; do not deploy unless authorized."
		d.LatestAttempt.ErrorMessage = "Kepatuhan auto_deploy: verified production deployment receipt is missing for this exact task, attempt and repository commit; report text is not deployment evidence"
		oldFinish := d.LatestAttempt.CreatedAt.Add(-10 * time.Minute)
		source := &task.TaskAttempt{ID: "scope-original", TaskID: d.ID, Status: task.AttemptStatusSucceeded, ExitCode: &exit, CreatedAt: oldFinish.Add(-time.Minute), FinishedAt: &oldFinish}
		if err := s.taskSvc.Store().CreateAttempt(ctx, source); err != nil {
			t.Fatal(err)
		}
		d.Attempts = append(d.Attempts, source)
		audit := &task.TaskActivity{ID: "scope-audit", TaskID: d.ID, AttemptID: &source.ID, ActivityType: task.ActivityPublicationAcceptanceAudit, Author: "coordinator", NewStatus: "review_required", CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Minute)}
		d.Activities = append(d.Activities, audit)
		switch strings.TrimPrefix(scenario, "scope ") {
		case "missing audit":
			audit.Author = "other"
		case "missing original source":
			d.Attempts = nil
		case "source failed":
			source.Status = task.AttemptStatusFailed
		case "source cancelled":
			source.CancelRequested = true
		case "late audit":
			audit.CreatedAt = time.Now().Add(time.Hour)
		case "publication requested":
			d.Instructions = strings.Replace(d.Instructions, "Clone the connected repository, verify its structure and prepare initial configuration.", "Implement and deploy the new feature.", 1)
		case "different verifier":
			d.TestSummary = "Tests unavailable"
		case "different failure":
			d.LatestAttempt.ErrorMessage = "Project verification failed"
		case "cancelled":
			d.LatestAttempt.CancelRequested = true
		case "missing report":
			d.Activities = d.Activities[:len(d.Activities)-2]
		case "missing acceptance":
			d.DoDChecklist[0].Completed = false
		}
	}
	switch scenario {
	case "cancelled":
		d.LatestAttempt.CancelRequested = true
	case "foreign owner":
		origin.OwnerID = "foreign"
	case "missing approval":
		origin.RequestID = "missing"
	case "code verifier":
		d.TestSummary = "Project verification: test.sh --fast (exit 0)"
	case "process failure":
		exit = 1
	case "missing report":
		d.Activities = nil
	case "later blocked report":
		later := *report
		later.ID, later.NewStatus, later.CreatedAt = "later-blocked", task.TaskStatusBlocked, finished.Add(time.Second)
		d.Activities = append(d.Activities, &later)
	case "missing acceptance":
		d.DoDChecklist[0].Completed = false
	}
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().UpdateAttempt(ctx, d.LatestAttempt); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "documentation-project-busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	before, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	count := taskRevisionAttemptCount(before)
	if scheduled := s.recoverSavedEvidence(ctx, d, origin); scheduled != (scenario == "valid" || scenario == "scope valid") {
		t.Fatal("incorrect evidence recovery eligibility", scheduled, scenario)
	}
	if scenario != "valid" && scenario != "scope valid" {
		if _, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, savedEvidenceRecoveryRequestID(d))); err == nil {
			t.Fatal("ineligible recovery persisted a launch")
		}
		return
	}
	for i := 0; i < 3; i++ {
		if !s.recoverSavedEvidence(ctx, d, origin) {
			t.Fatal("queued recovery lost its identity")
		}
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, savedEvidenceRecoveryRequestID(d)))
	if err != nil || launch.State != "queued" || launch.OwnerID != origin.OwnerID || launch.SourceRequestID != origin.RequestID {
		t.Fatal("missing durable original-owner launch", err)
	}
	limits := executionpolicy.FromInstructions(launch.Revision)
	if !limits.ReadOnlyRepository || !limits.DenyDeploy || !limits.DenyPush || !strings.Contains(launch.Revision, "unbound") || !strings.Contains(launch.Revision, "original criteria unchanged") {
		t.Fatal("historical hint became proof or new implementation permission")
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || len(after.Attempts) != len(before.Attempts) || taskRevisionAttemptCount(after) != count || after.Status == task.TaskStatusDone || after.LatestAttempt.Status != task.AttemptStatusFailed {
		t.Fatal("recovery bypassed lane, budget or changed failed history", err)
	}
	if err := s.taskSvc.Store().DeleteTask(ctx, "documentation-project-busy"); err != nil {
		t.Fatal(err)
	}
	_ = s.taskSvc.Store().DeleteTask(ctx, "phase-blocker")
	claimed, err := s.taskSvc.Store().(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "stop-doc-recovery", launch.ID)
	if err != nil || claimed == nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.FinishLaunch(ctx, claimed.ID, claimed.LeaseToken, "stopped", time.Now(), "verification unavailable"); err != nil {
		t.Fatal(err)
	}
	if s.recoverSavedEvidence(ctx, d, origin) {
		t.Fatal("stopped recovery chained a new execution")
	}
}

func TestNonPublicationScopeEvidenceRecovery(t *testing.T) {
	for _, scenario := range []string{"valid", "missing audit", "missing original source", "source failed", "source cancelled", "late audit", "publication requested", "different verifier", "different failure", "cancelled", "missing report", "missing acceptance"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, _ := stalledReviewFixture(t, "completed", task.AttemptStatusFailed)
			documentationEvidenceRecoveryContract(t, s, d, "scope "+scenario)
		})
	}
}

func TestNonPublicationScopeEvidenceRecoveryPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, _, d := phaseLineageSetup(t, database)
		documentationEvidenceRecoveryContract(t, s, d, "scope valid")
	})
}

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

func TestAccountOperationRecoversExistingOutcomeWithoutRepeatingEffect(t *testing.T) {
	for _, mode := range []string{"legacy source-check failure", "completed operation"} {
		t.Run(mode, func(t *testing.T) { accountOutcomeRecoveryContract(t, mode) })
	}
}
func accountOutcomeRecoveryContract(t *testing.T, mode string) {
	s, d := prerequisiteReviewFixture(t)
	ctx := context.Background()
	store := s.taskSvc.Store()
	d.Title = "Aktivasi akses pelanggan"
	d.Instructions = "Cuan managed delegation v1.\nAktifkan akses produk yang sudah ada untuk customer@example.invalid. Verifikasi entitlement aktif melalui API."
	d.Status = task.TaskStatusBlocked
	d.ExecutionCategory = task.ExecutionCategoryCode
	d.CommitSHA = ""
	d.DiffSummary = ""
	d.DoDChecklist = []task.DoDItem{{Text: "Customer access active", Completed: true, Evidence: "Worker reported existing access API has_access=true"}}
	if err := store.UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	var origin db.InboxRequest
	rows, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
	for _, q := range rows {
		if q.MessageID == d.SourceMessageID {
			origin = q
		}
	}
	message, _ := s.projectStudioRepo.GetMessageByID(ctx, origin.MessageID)
	message.Content = "Aktifkan akses produk yang sudah ada untuk customer@example.invalid. Verifikasi entitlement aktif melalui API."
	if err := s.projectStudioRepo.UpdateMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if !operationalSourceCheckMismatch(d) {
		t.Fatal("original account operation was not recognized")
	}
	for _, mode := range []string{"source change", "assertion", "cancelled", "unfinished"} {
		altered := *d
		attempt := *d.LatestAttempt
		altered.LatestAttempt = &attempt
		switch mode {
		case "source change":
			altered.CommitSHA = strings.Repeat("a", 40)
		case "assertion":
			attempt.ErrorMessage = "Project verification failed: actual test assertion"
		case "cancelled":
			attempt.CancelRequested = true
		case "unfinished":
			altered.DoDChecklist = []task.DoDItem{{Text: "Access still unverified"}}
		}
		if operationalSourceCheckMismatch(&altered) {
			t.Fatal("ineligible failure admitted", mode)
		}
	}
	if mode == "completed operation" {
		d.Status = task.TaskStatusReview
		d.ExecutionCategory = task.ExecutionCategoryOps
		a := *d.LatestAttempt
		a.ID = "completed-account-worker"
		a.Status = task.AttemptStatusSucceeded
		a.ErrorMessage = ""
		now := time.Now().UTC()
		a.CreatedAt = now
		a.StartedAt = &now
		a.FinishedAt = &now
		if err := store.CreateAttempt(ctx, &a); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "first-operation-report", TaskID: d.ID, AttemptID: &a.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Worker reported active entitlement", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := store.UpdateTask(ctx, &d.Task); err != nil {
			t.Fatal(err)
		}
		d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
		decision := s.requireIndependentCodeReview(ctx, d, inboxReviewEvaluation{Decision: "pass", RequestedOutcome: "completed"})
		if decision.Decision != "revise" || decision.RemainingWork != "verification" {
			t.Fatal("worker claim bypassed independent outcome verification")
		}
	}
	if err := store.CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: d.ExecutionCategory}); err != nil {
		t.Fatal(err)
	}
	parent := d.LatestAttempt.ID
	for i := 0; i < 2; i++ {
		if !s.recoverSavedEvidence(ctx, d, origin) {
			saved, e := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, savedEvidenceRecoveryRequestID(d)))
			t.Fatalf("existing outcome recovery not queued iteration=%d candidate=%v reviewauth=%v sameagent=%v continue=%v origin=%+v launch=%+v err=%v", i, savedEvidenceRecoveryCandidate(d), s.reviewSnapshotRecoveryAuthorized(ctx, d, origin), s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent), s.taskContinuationAfterCancel(ctx, origin, d), origin, saved, e)
		}
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, savedEvidenceRecoveryRequestID(d)))
	if err != nil || launch.State != "queued" || !strings.Contains(launch.Revision, "Never run access-grant") {
		t.Fatal("lost durable no-replay inspection", err)
	}
	// Use the same native claim/completion path. The failed worker is preserved;
	// the independent read-only observation still goes through PM review.
	_ = store.DeleteTask(ctx, "busy")
	if _, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "outcome", launch.ID); err != nil {
		t.Fatal(err)
	}
	attempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "observer", AgentName: d.AssignedAgent, RequestID: launch.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	service := runner.NewService(runner.NewMemoryStore(store), store)
	service.SetEventBus(runner.NewEventBus())
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "observer"}, time.Minute)
	if err != nil || !claim.Claimed || !claim.PublicationLimits.ReadOnlyRepository || !claim.PublicationLimits.DenyDeploy || !claim.PublicationLimits.DenyPush || claim.AutoDeploy || claim.PHPUnitTargets != "" {
		t.Fatal("operational observation inherited source workflow", err)
	}
	baseline := strings.Repeat("b", 64)
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "operational-verifier-report", TaskID: d.ID, AttemptID: &attempt.ID, Author: d.AssignedAgent, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Independently observed existing customer access API has_access=true; did not run access-grant", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(ctx, attempt.ID, runner.CompleteAttemptRequest{RunnerID: "observer", Status: task.AttemptStatusSucceeded, ExitCode: 0, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: baseline, Final: baseline}, TestSummary: "Read-only repository scope verified; original outcome still requires independent acceptance"}); err != nil {
		t.Fatal(err)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	old, _ := store.GetAttempt(ctx, parent)
	expected := task.AttemptStatusFailed
	if mode == "completed operation" {
		expected = task.AttemptStatusSucceeded
	}
	if old.Status != expected || current.Status != task.TaskStatusReview || current.LatestAttempt.ID != attempt.ID {
		t.Fatal("history rewritten or review bypassed")
	}
	if !s.accountOutcomeIndependentlyVerified(ctx, current) {
		t.Fatal("native independent observation not recognized")
	}
	decision := s.requireIndependentCodeReview(ctx, current, inboxReviewEvaluation{Decision: "pass", RequestedOutcome: "completed"})
	if decision.Decision != "pass" {
		t.Fatal("observed outcome trapped in another verification loop")
	}
	if s.independentCodeReviewMissing(ctx, current) {
		t.Fatal("account outcome treated as unverified source implementation")
	}
	if s.recoverSavedEvidence(ctx, current, origin) {
		t.Fatal("evidence recovery chained")
	}
}

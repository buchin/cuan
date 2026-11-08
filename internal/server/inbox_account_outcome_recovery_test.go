package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestAccountOperationRecoversExistingOutcomeWithoutRepeatingEffect(t *testing.T) {
	for _, mode := range []string{"legacy source-check failure", "completed operation", "legacy publication failure", "manager verification", "operational login diagnosis", "legacy login publication failure"} {
		t.Run(mode, func(t *testing.T) { accountOutcomeRecoveryContract(t, mode) })
	}
}
func accountOutcomeRecoveryContract(t *testing.T, mode string) {
	legacyLogin := mode == "legacy login publication failure"
	loginDiagnosis := mode == "operational login diagnosis" || legacyLogin
	if legacyLogin {
		mode = "legacy publication failure"
	} else if loginDiagnosis {
		mode = "completed operation"
	}
	s, d := prerequisiteReviewFixture(t)
	ctx := context.Background()
	store := s.taskSvc.Store()
	d.Title = "Aktivasi akses pelanggan"
	d.Instructions = "Cuan managed delegation v1.\nAktifkan akses produk yang sudah ada untuk customer@example.invalid. Verifikasi entitlement aktif melalui API."
	d.Status = task.TaskStatusBlocked
	d.ExecutionCategory = task.ExecutionCategoryCode
	if loginDiagnosis {
		d.Title = "Investigasi dan perbaiki kendala login akun pelanggan"
		d.Instructions = "Cuan managed delegation v1.\nPeriksa status akun dan riwayat autentikasi customer@example.invalid. Verifikasi hasil akun melalui layanan yang sudah ada."
		if !legacyLogin {
			d.ExecutionCategory = task.ExecutionCategoryOps
		}
	}
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
	if loginDiagnosis {
		message.Content = "Investigasi dan perbaiki kendala login akun pelanggan lewat layanan yang sudah ada."
	}
	if err := s.projectStudioRepo.UpdateMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	if mode == "legacy publication failure" || mode == "manager verification" {
		now := time.Now().UTC()
		original := *d.LatestAttempt
		original.ID = "legacy-publication-worker"
		original.CreatedAt = now
		original.StartedAt = &now
		original.FinishedAt = &now
		original.ErrorMessage = task.PublicationVerificationUnavailableMessage
		if err := store.CreateAttempt(ctx, &original); err != nil {
			t.Fatal(err)
		}
		d.CommitSHA = strings.Repeat("c", 40)
		d.DiffSummary = "retained unrequested source tests | 1 +"
		if err := store.UpdateTask(ctx, &d.Task); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: registered PHPUnit targets (exit 0)\nChecks passed", Changes: task.NewRunnerChangeRecord(d.CommitSHA, d.DiffSummary)})
		for _, event := range []*task.TaskActivity{
			{ID: "legacy-publication-report", TaskID: d.ID, AttemptID: &original.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Account access was reported active; incidental source tests also passed; publication unverified", CreatedAt: now},
			{ID: "verification:" + original.ID, TaskID: d.ID, AttemptID: &original.ID, Author: original.RunnerID, ActivityType: task.ActivityTypeRunnerVerification, Message: string(raw), CreatedAt: now},
		} {
			if err := store.CreateActivity(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if legacyLogin && (s.accountOutcomeIndependentlyVerified(ctx, d) || !s.independentCodeReviewMissing(ctx, d)) {
		t.Fatal("worker claims or historical source waived independent outcome evidence")
	}
	if loginDiagnosis && !task.SupportsAccountOutcomeInspection(&d.Task) {
		t.Fatal("operational login diagnosis did not require account outcome evidence")
	}
	if !loginDiagnosis && !operationalSourceCheckMismatch(d) && !legacyAccountPublicationMismatch(d) {
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
		if operationalSourceCheckMismatch(&altered) || legacyAccountPublicationMismatch(&altered) {
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
		if i == 0 {
			s.queueInboxTaskReview(ctx, d)
		}
		if !s.recoverSavedEvidence(ctx, d, origin) {
			saved, e := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, savedEvidenceRecoveryRequestID(d)))
			t.Fatalf("existing outcome recovery not queued iteration=%d candidate=%v reviewauth=%v sameagent=%v continue=%v origin=%+v launch=%+v err=%v", i, savedEvidenceRecoveryCandidate(d), s.reviewSnapshotRecoveryAuthorized(ctx, d, origin), s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent), s.taskContinuationAfterCancel(ctx, origin, d), origin, saved, e)
		}
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, savedEvidenceRecoveryRequestID(d)))
	if err != nil || launch.State != "queued" || !strings.Contains(launch.Revision, "Never run access-grant") {
		t.Fatal("lost durable no-replay inspection", err)
	}
	if mode == "manager verification" {
		assessment := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", ManagerAction: "continue", ManagerReason: "Independently observe account access", Revision: "Observe existing account access without external effects"}
		launch.ID = ""
		launch.RequestID = delegationMessageID("manager-account-verification")
		launch.Revision = verificationReviewRevision(assessment.Revision)
		launch.CreatedAt = time.Now().UTC()
		launch.AvailableAt = launch.CreatedAt
		raw, _ := json.Marshal(reviewAssessmentRecord{RequestID: launch.RequestID, InstructionsHash: launch.InstructionsHash, EvidenceDigest: inboxReviewEvidenceDigest(d), Assessment: assessment})
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "manager-grant", TaskID: d.ID, AttemptID: &parent, Author: "coordinator", ActivityType: reviewAssessmentActivity, Message: string(raw), CreatedAt: launch.CreatedAt}); err != nil {
			t.Fatal(err)
		}
		launch, err = s.taskSvc.SaveLaunch(ctx, *launch)
		if err != nil {
			t.Fatal(err)
		}
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
	if mode == "legacy publication failure" || mode == "manager verification" {
		pub := s.inboxReviewPublication(ctx, current)
		if pub == nil || pub.Required || pub.Provenance != "original_account_scope_no_source_publication" || current.CommitSHA != d.CommitSHA || current.DiffSummary != d.DiffSummary {
			t.Fatal("retained source was waived, released or demanded as original scope")
		}
		approved, _ := s.inboxApprovedTaskScope(ctx, current)
		if legacyLogin {
			if approved == nil || approved.ReadOnlyRepository || approved.DenyDeploy {
				t.Fatal("legacy inspection rewrote the original worker permissions")
			}
			observed, err := s.taskSvc.AttemptPublicationPolicy(ctx, current.ID, current.LatestAttempt.ID)
			if err != nil || !observed.ReadOnlyRepository || !observed.DenyDeploy || !observed.DenyPush {
				t.Fatal("legacy observation lost its own read-only boundary", err)
			}
		} else if approved == nil || !approved.ReadOnlyRepository || !approved.DenyDeploy {
			t.Fatal("review lost original operational scope")
		}
	}
	if s.independentCodeReviewMissing(ctx, current) {
		t.Fatal("account outcome treated as unverified source implementation")
	}
	s.queueInboxTaskReview(ctx, current)
	if s.recoverSavedEvidence(ctx, current, origin) {
		t.Fatal("evidence recovery chained")
	}
}

func TestManagerAccountObservationRequiresExactPredecessorGrant(t *testing.T) {
	now := time.Now().UTC()
	finished := now.Add(-time.Minute)
	parent := &task.TaskAttempt{ID: "parent", TaskID: "account", CreatedAt: now.Add(-2 * time.Minute), FinishedAt: &finished, Status: task.AttemptStatusSucceeded}
	current := &task.TaskAttempt{ID: "observer", TaskID: "account", CreatedAt: now.Add(time.Second)}
	d := &task.TaskDetailResponse{Task: task.Task{ID: "account", SourceRoomID: "room", ProjectID: "project", AssignedAgent: "agent"}, LatestAttempt: current, Attempts: []*task.TaskAttempt{current, parent}}
	assessment := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", ManagerAction: "continue", ManagerReason: "Check existing access", Revision: "Observe access without activation"}
	launch := &task.TaskLaunch{TaskID: d.ID, RoomID: d.SourceRoomID, ProjectID: d.ProjectID, AgentID: d.AssignedAgent, RequestID: "review-request", InstructionsHash: task.LaunchInstructionsHash(d.Instructions), CreatedAt: now, Revision: verificationReviewRevision(assessment.Revision)}
	raw, _ := json.Marshal(reviewAssessmentRecord{RequestID: launch.RequestID, InstructionsHash: launch.InstructionsHash, EvidenceDigest: strings.Repeat("a", 64), Assessment: assessment})
	event := &task.TaskActivity{TaskID: d.ID, AttemptID: &parent.ID, Author: "coordinator", ActivityType: reviewAssessmentActivity, CreatedAt: finished.Add(time.Second), Message: string(raw)}
	d.Activities = []*task.TaskActivity{event}
	if !managerAccountObservationGrant(d, launch) {
		t.Fatal("exact admitted verifier was rejected")
	}
	launch.Revision = "Binding approved continuation:\nOriginal owner constraints\n\nAutomatic correction feedback:\n" + launch.Revision
	if !managerAccountObservationGrant(d, launch) {
		t.Fatal("inherited constraints lost verifier grant")
	}
	for _, mode := range []string{"self report", "foreign request", "changed instructions", "unwrapped feedback", "cancelled predecessor", "foreign room", "future assessment", "later predecessor"} {
		t.Run(mode, func(t *testing.T) {
			dd := *d
			ll := *launch
			ee := *event
			pp := *parent
			dd.Activities = []*task.TaskActivity{&ee}
			dd.Attempts = []*task.TaskAttempt{current, &pp}
			switch mode {
			case "self report":
				ee.AttemptID = &current.ID
			case "foreign request":
				ll.RequestID = "other"
			case "changed instructions":
				ll.InstructionsHash = "other"
			case "unwrapped feedback":
				ll.Revision = assessment.Revision
			case "cancelled predecessor":
				pp.CancelRequested = true
			case "foreign room":
				ll.RoomID = "other"
			case "future assessment":
				ee.CreatedAt = now.Add(time.Minute)
			case "later predecessor":
				later := pp
				later.ID = "later"
				later.CreatedAt = finished
				dd.Attempts = append(dd.Attempts, &later)
			}
			if managerAccountObservationGrant(&dd, &ll) {
				t.Fatal("invalid verifier grant accepted")
			}
		})
	}
}

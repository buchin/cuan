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
	for _, mode := range []string{"legacy source-check failure", "completed operation", "legacy publication failure", "manager verification", "operational login diagnosis", "readonly login source-check failure", "legacy login publication failure", "legacy login source verifier"} {
		t.Run(mode, func(t *testing.T) { accountOutcomeRecoveryContract(t, mode) })
	}
}
func accountOutcomeRecoveryContract(t *testing.T, mode string) {
	sourceVerified := mode == "legacy login source verifier"
	readonlyLogin := mode == "readonly login source-check failure"
	legacyLogin := mode == "legacy login publication failure" || sourceVerified
	loginDiagnosis := mode == "operational login diagnosis" || legacyLogin || readonlyLogin
	if legacyLogin {
		mode = "legacy publication failure"
	} else if readonlyLogin {
		mode = "legacy source-check failure"
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
		if readonlyLogin {
			d.ExecutionCategory = task.ExecutionCategoryReadOnly
		}
	}
	d.CommitSHA = ""
	d.DiffSummary = ""
	d.DoDChecklist = []task.DoDItem{{Text: "Customer access active", Completed: true, Evidence: "Worker reported existing access API has_access=true"}}
	if readonlyLogin {
		// Real account reports carry long criteria and observations. They belong
		// in bound task context, not duplicated into the feedback-sized launch.
		for i := 0; i < 8; i++ {
			d.DoDChecklist = append(d.DoDChecklist, task.DoDItem{Text: strings.Repeat("Verify account outcome with actual observations. ", 20), Completed: true, Evidence: "Worker claim; independent observation still required"})
		}
	}
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

	if sourceVerified {
		d.ResultKind = "implementation"
		if err := store.UpdateTask(ctx, &d.Task); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "legacy-source-result-kind", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "agent_result_kind", Author: d.AssignedAgent, NewStatus: "implementation", CreatedAt: *d.LatestAttempt.FinishedAt}); err != nil {
			t.Fatal(err)
		}
		d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
		assessment := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", ManagerAction: "replan", ManagerReason: "Check exact retained source", Explanation: "Source still requires independent review", Revision: "Verify the saved source"}
		if _, err := s.saveReviewAssessment(ctx, d, origin, assessment); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateTask(ctx, &task.Task{ID: "source-busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
			t.Fatal(err)
		}
		if text, scheduled := s.startVerificationFollowup(ctx, origin, d, assessment); !scheduled {
			t.Fatal("source verifier not scheduled", text)
		}
		id := task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA, d.LatestAttempt.ID))
		job, err := s.taskSvc.GetLaunch(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.DeleteTask(ctx, "source-busy"); err != nil {
			t.Fatal(err)
		}
		if _, err := store.(task.LaunchStore).ClaimLaunch(ctx, job.AvailableAt.Add(time.Second), "source-observation", job.ID); err != nil {
			t.Fatal(err)
		}
		a, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "source-checker", AgentName: d.AssignedAgent, RequestID: job.RequestID})
		if err != nil {
			t.Fatal(err)
		}
		service := runner.NewService(runner.NewMemoryStore(store), store)
		claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "source-checker"}, time.Minute)
		if err != nil || !claim.Claimed {
			t.Fatal("source claim", err)
		}
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "source-verifier-final", TaskID: d.ID, AttemptID: &a.ID, Author: d.AssignedAgent, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Exact source checked; account outcome remains unverified", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
		baseline := strings.Repeat("d", 64)
		if _, err := service.Complete(ctx, a.ID, runner.CompleteAttemptRequest{RunnerID: "source-checker", Status: task.AttemptStatusSucceeded, ExitCode: 0, VerifiedSourceCommit: d.CommitSHA, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: baseline, Final: baseline}}); err != nil {
			t.Fatal(err)
		}
		if err := s.taskSvc.FinishLaunch(ctx, job.ID, "source-observation", "stopped", time.Now(), ""); err != nil {
			t.Fatal(err)
		}
		d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if savedEvidenceRecoveryCandidate(d) || s.accountOutcomeSourceVerification(ctx, d) == nil || s.accountOutcomeIndependentlyVerified(ctx, d) {
			t.Fatal("source proof lost its typed handoff or replaced account evidence")
		}
		for _, scenario := range []string{"changed scope", "cancelled verifier", "missing receipts", "changed source", "active verifier"} {
			altered := *d
			attempt := *d.LatestAttempt
			altered.LatestAttempt = &attempt
			switch scenario {
			case "changed scope":
				altered.Instructions += " changed"
			case "cancelled verifier":
				attempt.CancelRequested = true
			case "missing receipts":
				altered.Activities = nil
			case "changed source":
				altered.CommitSHA = strings.Repeat("e", 40)
			case "active verifier":
				attempt.Status = task.AttemptStatusRunning
			}
			if s.accountOutcomeSourceVerification(ctx, &altered) != nil {
				t.Fatal("invalid source handoff accepted", scenario)
			}
		}
	}
	// A migration blocks every lane, including concurrent read-only inspection.
	if err := store.CreateTask(ctx, &task.Task{ID: "busy", Title: "Schema migration", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
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
	if readonlyLogin && (len([]rune(launch.Revision)) > task.MaxTaskFeedbackRunes || !strings.Contains(launch.Revision, "Read the bound task get")) {
		t.Fatal("saved reports prevented bounded observer admission")
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
	if mode == "completed operation" || sourceVerified {
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

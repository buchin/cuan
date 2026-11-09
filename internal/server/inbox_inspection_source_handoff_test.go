package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func failedInspectionSourceFixture(t *testing.T, local ...bool) (*Server, *task.TaskDetailResponse, db.InboxRequest, []db.InboxRequest, *task.LiveInspectionIntent) {
	t.Helper()
	s, d, oldReview, decision := unpublishedSourceVerificationFixture(t)
	ctx := context.Background()
	store := s.taskSvc.Store()
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	oldReview.State = "completed"
	if err := repo.FinishInboxRequest(ctx, oldReview, nil); err != nil {
		t.Fatal("finish original source review", err)
	}
	requests, _ := repo.ListInboxRequests(ctx, d.SourceRoomID)
	var origin db.InboxRequest
	for _, q := range requests {
		if q.MessageID == d.SourceMessageID {
			origin = q
		}
	}
	unfinishedLocal := len(local) > 0 && local[0]
	if unfinishedLocal {
		if len(local) > 2 && local[2] {
			d.Instructions += "\nImplement and deploy the approved result."
		} else {
			d.Instructions += "\nPush the task branch. Do not merge or deploy."
		}
		d.CommitSHA, d.TestSummary, d.Summary, d.CompleteNote = "", "", "", ""
		d.Status = task.TaskStatusBlocked
		if err := store.UpdateTask(ctx, &d.Task); err != nil {
			t.Fatal(err)
		}
		source, err := store.GetAttempt(ctx, d.LatestAttempt.ID)
		if err != nil {
			t.Fatal(err)
		}
		started, finished := time.Now().UTC().Add(-2*time.Second), time.Now().UTC().Add(-time.Second)
		exit := 124
		source.CreatedAt, source.StartedAt, source.FinishedAt, source.Status, source.ExitCode, source.ErrorMessage = started, &started, &finished, task.AttemptStatusFailed, &exit, "Execution timed out after adaptive timeout (25m0s)"
		if err := store.UpdateAttempt(ctx, source); err != nil {
			t.Fatal(err)
		}
		d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	}
	at := time.Now().UTC()
	binding := &task.LiveInspectionIntent{Purpose: task.CurrentOutcomeInspection, SourceAttemptID: d.LatestAttempt.ID, SourceCommit: d.CommitSHA, TargetCommit: strings.Repeat("b", 40), Repository: "fixture/repo", OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), ObservedAt: at}
	raw, _ := json.Marshal(binding)
	requestID := binding.RequestID(d.ID)
	launch, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: requestID, OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, AgentID: d.AssignedAgent, InstructionsHash: binding.InstructionsHash, Revision: "Scope: read-only repository. Do not push. Do not deploy.\n" + task.LiveInspectionMarker + string(raw), CreatedAt: at, AvailableAt: at, ExpiresAt: at.Add(time.Hour)})
	if err != nil {
		t.Fatal("save inspection launch", err)
	}
	owned, err := s.taskSvc.ClaimLaunch(ctx, at, "fixture-inspection", launch.ID)
	if err != nil || owned == nil {
		t.Fatal("inspection fixture launch", err)
	}

	sum := sha256.Sum256([]byte(d.ID + "\x00" + strings.ToLower(requestID)))
	id := fmt.Sprintf("att_%x", sum[:16])
	finished := time.Now().UTC()
	inspection := &task.TaskAttempt{ID: id, TaskID: d.ID, AttemptNumber: 3, AgentName: d.AssignedAgent, RunnerID: "runner", Status: task.AttemptStatusFailed, ExitCode: new(int), CreatedAt: at, StartedAt: &at, FinishedAt: &finished, ErrorMessage: "Live result inspection project checks did not pass"}
	if len(local) > 1 && local[1] {
		inspection.Status, inspection.ErrorMessage = task.AttemptStatusSucceeded, ""
	}
	if err := store.CreateAttempt(ctx, inspection); err != nil {
		t.Fatal("record failed inspection", err)
	}
	if err := s.taskSvc.FinishLaunch(ctx, launch.ID, "fixture-inspection", "stopped", at, ""); err != nil {
		t.Fatal("finish inspection launch", err)
	}
	report := &task.TaskActivity{ID: "inspection-final", TaskID: d.ID, AttemptID: &id, ActivityType: "agent_report", NewStatus: "review", Message: "The observed older release still has failing tests; the tested implementation source remains unpublished.", CreatedAt: finished}
	if err := store.CreateActivity(ctx, report); err != nil {
		t.Fatal("save inspection report", err)
	}
	if inspection.Status == task.AttemptStatusSucceeded {
		for _, event := range []*task.TaskActivity{
			{ID: "verification:" + id, TaskID: d.ID, AttemptID: &id, ActivityType: task.ActivityTypeRunnerVerification, Author: inspection.RunnerID, Message: `{"summary":"Project verification: test.sh --fast (exit 0)"}`, CreatedAt: finished},
			{ID: "scope:" + id, TaskID: d.ID, AttemptID: &id, ActivityType: "repository_scope_verified", Author: inspection.RunnerID, NewStatus: "read_only", CreatedAt: finished},
		} {
			if err := store.CreateActivity(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	s.queueInboxTaskReview(ctx, d)
	reviews := reviewRequests(t, s, d.SourceRoomID)
	q := reviews[len(reviews)-1]
	decision.Finding = "Verify the retained tested source before publishing it"
	decision.Revision = "Verify the exact implementation source already bound by this failed production inspection"
	if unfinishedLocal {
		decision.RemainingWork, decision.RequestedOutcome, decision.ManagerAction, decision.ManagerReason = "implementation", "unfinished", "replan", "The unrelated live inspection did not inspect the retained unfinished developer workspace."
		decision.Revision = "Inspect the retained developer workspace, finish the original local feature and targeted checks, commit and push only the task branch. Preserve all existing files; do not merge or deploy."
		if len(local) > 2 && local[2] {
			decision.Revision = "Inspect the retained developer workspace, finish the original feature and targeted checks, then hand off for independent verification and the approved native release. Preserve all existing work."
		}
	}
	if _, err := s.saveReviewAssessment(ctx, d, q, decision); err != nil {
		t.Fatal("save inspection assessment", err)
	}
	q.State = "completed"
	if err := repo.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal("finish inspection review", err)
	}
	if err := store.CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal("hold project lane", err)
	}
	requests, _ = repo.ListInboxRequests(ctx, d.SourceRoomID)
	return s, d, origin, requests, binding
}

func TestSuccessfulNegativeInspectionResumesOriginalDeveloperScope(t *testing.T) {
	for _, deployAllowed := range []bool{false, true} {
		t.Run(fmt.Sprint(deployAllowed), func(t *testing.T) {
			s, d, origin, requests, binding := failedInspectionSourceFixture(t, true, true, deployAllowed)
			ctx := context.Background()
			d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
			var review db.InboxRequest
			for _, q := range requests {
				if assessment, ok := reviewAssessment(d, q.RequestID); ok && assessment.RemainingWork == "implementation" {
					review = q
				}
			}
			decision, found := reviewAssessment(d, review.RequestID)
			if !found || !s.localSourceCorrectionAllowed(ctx, review, d, decision.Revision) {
				t.Fatal("valid negative inspection stranded approved implementation")
			}
			for _, scenario := range []string{"missing native checks", "missing unchanged receipt", "missing report", "running", "cancelled", "changed revision"} {
				altered := *d
				latest := *d.LatestAttempt
				altered.LatestAttempt = &latest
				altered.Activities = nil
				for _, event := range d.Activities {
					if event.AttemptID != nil && *event.AttemptID == latest.ID &&
						((scenario == "missing native checks" && event.ActivityType == task.ActivityTypeRunnerVerification) ||
							(scenario == "missing unchanged receipt" && event.ActivityType == "repository_scope_verified") ||
							(scenario == "missing report" && event.ActivityType == "agent_report")) {
						continue
					}
					altered.Activities = append(altered.Activities, event)
				}
				revision := decision.Revision
				if scenario == "running" {
					latest.Status = task.AttemptStatusRunning
				} else if scenario == "cancelled" {
					latest.CancelRequested = true
				} else if scenario == "changed revision" {
					revision += " different scope"
				}
				if s.localSourceCorrectionAllowed(ctx, review, &altered, revision) {
					t.Fatal("invalid inspection admitted correction", scenario)
				}
			}
			for i := 0; i < 2; i++ {
				if !s.recoverManagerVerificationHandoff(ctx, d, origin, requests) && i == 0 {
					after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
					launch, launchErr := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, review.RequestID))
					t.Fatalf("saved continue decision failed recovery: status=%s launch=%+v err=%v", after.Status, launch, launchErr)
				}
			}
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, review.RequestID))
			if err != nil || launch == nil || launch.State != "queued" {
				t.Fatal("missing developer continuation", err)
			}
			if intent, err := task.LiveInspectionForLaunch(&d.Task, launch); err != nil || intent != nil {
				t.Fatal("continuation reused read-only inspection", err)
			}
			if err := s.taskSvc.Store().DeleteTask(ctx, "busy"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "developer", launch.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: launch.RequestID}); err != nil {
				t.Fatal("developer reservation rejected", err)
			}
			service := runner.NewService(runner.NewMemoryStore(s.taskSvc.Store()), s.taskSvc.Store(), nil)
			claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "developer"}, time.Minute)
			if err != nil || claim == nil || !claim.Claimed || claim.LiveInspection != nil || claim.Verification != nil || claim.PublicationLimits.ReadOnlyRepository || claim.PublicationLimits.DenyDeploy == deployAllowed {
				t.Fatal("developer lost original execution limits", err, claim)
			}
			source, _ := s.taskSvc.Store().GetAttempt(ctx, binding.SourceAttemptID)
			if !task.IsExecutionTimeout(source) {
				t.Fatal("original failure history changed")
			}
		})
	}
}

func TestFailedProductionInspectionRecoversBoundTestedSourceWithoutReplay(t *testing.T) {
	s, d, origin, requests, binding := failedInspectionSourceFixture(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if !s.recoverManagerVerificationHandoff(ctx, d, origin, requests) {
			t.Fatal("bound source handoff not recovered")
		}
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA, binding.SourceAttemptID)))
	if err != nil || launch == nil || launch.State != "queued" {
		t.Fatal("missing source verifier", err)
	}
	proof, err := task.VerificationIntentForLaunch(&d.Task, launch)
	if err != nil || proof == nil || proof.SourceAttemptID != binding.SourceAttemptID || proof.SourceCommit != binding.SourceCommit || !proof.IsolatedWorkspace {
		t.Fatal("failed inspection replaced exact source", err, proof)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	source, _ := s.taskSvc.Store().GetAttempt(ctx, binding.SourceAttemptID)
	if len(after.Attempts) != len(d.Attempts) || source.Status != task.AttemptStatusFailed || after.Status == task.TaskStatusDone {
		t.Fatal("recovery replayed execution or accepted work")
	}

	// Exercise claim and completion too: a queued envelope alone does not prove
	// that the runner can validate the older source after the failed inspection.
	store := s.taskSvc.Store()
	if err := store.DeleteTask(ctx, "busy"); err != nil {
		t.Fatal(err)
	}
	if owned, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "source-verifier", launch.ID); err != nil || owned == nil {
		t.Fatal(err)
	}
	att, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: launch.RequestID})
	if err != nil {
		t.Fatal("source verifier reservation failed", err)
	}
	service := runner.NewService(runner.NewMemoryStore(store), store, nil)
	service.SetEventBus(runner.NewEventBus())
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "verifier"}, time.Minute)
	if err != nil || claim == nil || !claim.Claimed || claim.Verification == nil || claim.Verification.SourceAttemptID != binding.SourceAttemptID || claim.Verification.SourceCommit != binding.SourceCommit {
		t.Fatal("runner lost exact original source", err, claim)
	}
	if !claim.PublicationLimits.ReadOnlyRepository || !claim.PublicationLimits.DenyDeploy || !claim.PublicationLimits.DenyPush {
		t.Fatal("source claim gained execution authority")
	}
	_, err = service.Complete(ctx, att.ID, runner.CompleteAttemptRequest{RunnerID: "verifier", Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: binding.SourceCommit, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("c", 64), Final: strings.Repeat("c", 64)}})
	if err != nil {
		t.Fatal("source verifier completion rejected", err)
	}
	completed, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	evidence := s.verificationReviewContext(ctx, completed)
	if evidence == nil || evidence["source_attempt_id"] != binding.SourceAttemptID || completed.Status == task.TaskStatusDone {
		t.Fatal("source verification lost history or became acceptance", evidence)
	}
}

func TestFailedInspectionSourceRecoveryRejectsChangedOrCanceledEvidence(t *testing.T) {
	for _, scenario := range []string{"cancelled", "running", "changed source", "wrong purpose", "failed source checks", "foreign owner"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, origin, requests, binding := failedInspectionSourceFixture(t)
			switch scenario {
			case "cancelled":
				d.LatestAttempt.CancelRequested = true
			case "running":
				d.LatestAttempt.Status = task.AttemptStatusRunning
			case "changed source":
				d.CommitSHA = strings.Repeat("c", 40)
			case "wrong purpose":
				binding.Purpose = task.NativeReleaseInspection
			case "failed source checks":
				for _, e := range d.Activities {
					if e.ID == "verification:"+binding.SourceAttemptID {
						e.Message = `{"summary":"Project verification: test.sh --fast (exit 1)"}`
					}
				}
			case "foreign owner":
				origin.OwnerID = "foreign"
			}
			if scenario == "wrong purpose" {
				if s.testedSourceAfterFailedInspection(context.Background(), d, binding) != nil {
					t.Fatal("wrong inspection purpose accepted")
				}
				return
			}
			if s.recoverManagerVerificationHandoff(context.Background(), d, origin, requests) {
				t.Fatal("invalid inspection evidence started verifier")
			}
		})
	}
}

func TestFailedLocalInspectionResumesSeparateDeveloperFromSavedDecision(t *testing.T) {
	s, d, origin, requests, binding := failedInspectionSourceFixture(t, true)
	ctx := context.Background()
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	var review db.InboxRequest
	for _, q := range requests {
		if q.State == "completed" {
			if saved, ok := reviewAssessment(d, q.RequestID); ok && saved.RemainingWork == "implementation" {
				review = q
			}
		}
	}
	decision, ok := reviewAssessment(d, review.RequestID)
	if !ok || !s.localSourceCorrectionAllowed(ctx, review, d, decision.Revision) {
		t.Fatal("saved local correction not eligible")
	}
	for _, scenario := range []string{"foreign owner", "changed revision", "cancelled inspection", "source already handed off", "changed scope"} {
		altered := *d
		latest := *d.LatestAttempt
		altered.LatestAttempt = &latest
		q := review
		revision := decision.Revision
		switch scenario {
		case "foreign owner":
			q.OwnerID = "other"
		case "changed revision":
			revision += " expand scope"
		case "cancelled inspection":
			latest.CancelRequested = true
		case "source already handed off":
			altered.CommitSHA = strings.Repeat("a", 40)
		case "changed scope":
			altered.Instructions += " changed"
		}
		if s.localSourceCorrectionAllowed(ctx, q, &altered, revision) {
			t.Fatal("invalid correction accepted", scenario)
		}
	}
	if !s.recoverManagerVerificationHandoff(ctx, d, origin, requests) {
		t.Fatal("saved local developer correction not admitted")
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, review.RequestID))
	if err != nil || launch == nil || launch.State != "queued" || !strings.Contains(launch.Revision, decision.Revision) {
		t.Fatal("missing separate developer launch", err)
	}
	if proof, err := task.LiveInspectionForLaunch(&d.Task, launch); err != nil || proof != nil {
		t.Fatal("developer correction reused inspection", err)
	}
	if proof, err := task.VerificationIntentForLaunch(&d.Task, launch); err != nil || proof != nil {
		t.Fatal("developer correction reused verifier", err)
	}
	if launch.SourceRequestID != origin.RequestID || launch.OwnerID != origin.OwnerID {
		t.Fatal("lost original authorization")
	}
	before := len(d.Attempts)
	s.recoverManagerVerificationHandoff(ctx, d, origin, requests)
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	source, _ := s.taskSvc.Store().GetAttempt(ctx, binding.SourceAttemptID)
	if len(after.Attempts) != before || after.Status == task.TaskStatusDone || !task.IsExecutionTimeout(source) {
		t.Fatal("recovery changed history or accepted work")
	}
	// Claiming the new attempt must use implementation scope, not the finished
	// inspection's read-only restrictions. Production writes remain forbidden.
	_ = s.taskSvc.Store().DeleteTask(ctx, "busy")
	if claimed, err := s.taskSvc.ClaimLaunch(ctx, time.Now().UTC(), "developer-admission", launch.ID); err != nil || claimed == nil {
		t.Fatal("developer launch not claimable", err)
	}
	attempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "developer", AgentName: d.AssignedAgent, RequestID: review.RequestID, ExpectedPredecessorID: d.LatestAttempt.ID})
	if err != nil {
		t.Fatal(err)
	}
	service := runner.NewService(runner.NewMemoryStore(s.taskSvc.Store()), s.taskSvc.Store())
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "developer"}, time.Minute)
	if err != nil || !claim.Claimed || claim.Attempt.ID != attempt.ID || claim.PublicationLimits.ReadOnlyRepository || claim.PublicationLimits.DenyPush || !claim.PublicationLimits.DenyDeploy || claim.Verification != nil || claim.LiveInspection != nil {
		t.Fatal("developer claim lost saved scope", err)
	}
}

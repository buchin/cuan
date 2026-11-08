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

func failedInspectionSourceFixture(t *testing.T) (*Server, *task.TaskDetailResponse, db.InboxRequest, []db.InboxRequest, *task.LiveInspectionIntent) {
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
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	s.queueInboxTaskReview(ctx, d)
	reviews := reviewRequests(t, s, d.SourceRoomID)
	q := reviews[len(reviews)-1]
	decision.Finding = "Verify the retained tested source before publishing it"
	decision.Revision = "Verify the exact implementation source already bound by this failed production inspection"
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

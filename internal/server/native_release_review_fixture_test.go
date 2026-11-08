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

// Complete the real read-only verification protocol before release fixtures
// exercise publication. No provider or release script is executed here.
func approveNativeReleaseFixtureSource(t *testing.T, s *Server, store task.Store, runtime runner.Store, item *task.Task, source *task.TaskAttempt, origin db.InboxRequest, release *task.NativeReleaseIntent) *task.TaskDetailResponse {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	proof := task.VerificationIntent{IsolatedWorkspace: true, SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, InstructionsHash: release.InstructionsHash, OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, AssessmentRequestID: "fixture-source-assessment", SourceVerificationHash: release.SourceVerificationHash}
	raw, _ := json.Marshal(proof)
	requestID := task.VerificationRequestID(item.ID, item.CommitSHA)
	now := time.Now().UTC()
	job, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: requestID, AgentID: item.AssignedAgent, OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, InstructionsHash: proof.InstructionsHash, Revision: "Scope: read-only repository. Do not push. Do not deploy.\n" + task.VerificationIntentMarker + string(raw), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
	must(err)
	_, err = s.taskSvc.ClaimLaunch(ctx, now, "fixture-review", job.ID)
	must(err)
	verifier, err := s.taskSvc.CreateAttempt(ctx, item.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "fixture-reviewer", AgentName: item.AssignedAgent, RequestID: requestID})
	must(err)
	service := runner.NewService(runtime, store, nil)
	// The test drives coordinator scans explicitly. Publishing to the process
	// global bus here would start unrelated fixture callbacks during setup.
	service.SetEventBus(runner.NewEventBus())
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "fixture-reviewer"}, time.Minute)
	must(err)
	if !claim.Claimed || claim.Attempt.ID != verifier.ID {
		t.Fatal("independent source fixture was not claimed")
	}
	_, err = service.Complete(ctx, verifier.ID, runner.CompleteAttemptRequest{RunnerID: "fixture-reviewer", Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: item.CommitSHA, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("d", 64), Final: strings.Repeat("d", 64)}, DoDValidation: &runner.DoDValidationResult{Passed: false, Action: "review"}})
	must(err)
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if detail.LatestAttempt.Status != task.AttemptStatusSucceeded {
		t.Fatal("source fixture verification failed", detail.LatestAttempt.ErrorMessage)
	}
	must(store.CreateActivity(ctx, &task.TaskActivity{ID: "fixture-source-final", TaskID: item.ID, AttemptID: &verifier.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Independent source behavior and scope checked; publication remains pending.", CreatedAt: *detail.LatestAttempt.FinishedAt}))
	detail, err = s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	assessment := inboxReviewEvaluation{Decision: "revise", SourceReview: "pass", RemainingWork: "publication", Explanation: "Source checked independently; publication pending.", Revision: "Publish this reviewed source through the approved release path."}
	reviewRequest := origin
	reviewRequest.RequestID = "fixture-source-verdict"
	_, err = s.saveReviewAssessment(ctx, detail, reviewRequest, assessment)
	must(err)
	approval, err := task.NativeSourceReviewForAttempt(ctx, store, &detail.Task, verifier.ID)
	must(err)
	if approval == nil {
		t.Fatal("source approval missing")
	}
	release.SourceReviewAttemptID = verifier.ID
	must(s.taskSvc.FinishLaunch(ctx, job.ID, "fixture-review", "stopped", time.Now(), ""))
	detail, err = s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if !s.hasNativeSourcePublicationHandoff(ctx, detail) {
		t.Fatal("native reviewed source lost its publication handoff")
	}
	without := *detail
	without.Activities = nil
	for _, event := range detail.Activities {
		if event.ActivityType != task.ActivityNativeReleaseHandoff {
			without.Activities = append(without.Activities, event)
		}
	}
	if s.hasNativeSourcePublicationHandoff(ctx, &without) {
		t.Fatal("source approval manufactured a missing native publication handoff")
	}
	return detail
}

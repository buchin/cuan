package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// Continue the native lifecycle from implementation -> supplemental verifier.
// The enclosing contract then claims/completes the exact live inspection.
func supplementalInspectionFixture(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse, database *sql.DB) *task.TaskDetailResponse {
	t.Helper()
	ctx := context.Background()
	store := s.taskSvc.Store()
	sourceID := d.LatestAttempt.ID
	raw, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: test.sh --fast (exit 0)\nSource checks passed"})
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "verification:" + sourceID, TaskID: d.ID, AttemptID: &sourceID, ActivityType: task.ActivityTypeRunnerVerification, Author: d.LatestAttempt.RunnerID, Message: string(raw), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	row, err := store.GetTask(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	row.Status = task.TaskStatusReview
	if err := store.UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	review := origin
	review.RequestID = db.GenerateUUID()
	assessment := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Original feature requires visual verification", Explanation: "Inspect actual components and existing release", Revision: "Read-only visual verification of original criteria"}
	if _, err := s.saveReviewAssessment(ctx, d, review, assessment); err != nil {
		t.Fatal("save source assessment", err)
	}
	if _, ok := s.startVerificationFollowup(ctx, review, d, assessment); !ok {
		t.Fatal("supplemental verification not admitted")
	}
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA)))
	if err != nil || job == nil {
		t.Fatal("missing supplemental launch", err)
	}
	_ = store.DeleteTask(ctx, "phase-blocker")
	if _, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "supplemental-claim", job.ID); err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: job.RequestID})
	if err != nil {
		t.Fatal("create supplemental attempt", err)
	}
	var workerStore runner.Store = runner.NewMemoryStore(store)
	if database != nil {
		workerStore = runner.NewDBStore(database)
	}
	service := runner.NewService(workerStore, store, s.projectSvc)
	// This contract drives review explicitly. A process-global completion
	// subscriber must not race the synthetic review or read-only diagnostics.
	service.SetEventBus(runner.NewEventBus())
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "supplemental-runner"}, time.Minute)
	if err != nil || !claim.Claimed {
		t.Fatal("supplemental claim failed", err)
	}
	if _, err := service.Complete(ctx, a.ID, runner.CompleteAttemptRequest{RunnerID: "supplemental-runner", Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: d.CommitSHA, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("b", 64), Final: strings.Repeat("b", 64)}}); err != nil {
		t.Fatal("complete supplemental attempt", err)
	}
	if err := store.(task.LaunchStore).FinishLaunch(ctx, job.ID, "supplemental-claim", "stopped", time.Now(), "Execution finished"); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if evidence := s.verificationReviewContext(ctx, d); evidence == nil || evidence["source_attempt_id"] != sourceID {
		t.Fatal("supplemental native source not verified", evidence)
	}
	// Native launch observations can arrive after completion. Collect them
	// before saving the synthetic review rather than accepting stale evidence.
	s.observeInboxPublicationConfiguration(ctx, d, origin)
	s.observeInboxGitHubConnection(ctx, d, origin)
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	review.RequestID = db.GenerateUUID()
	if _, err := s.saveReviewAssessment(ctx, d, review, assessment); err != nil {
		t.Fatal("save supplemental assessment", err)
	}
	if err := store.CreateTask(ctx, &task.Task{ID: "phase-blocker", ProjectID: d.ProjectID, Title: "Other work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	original, proof := s.verifiedSupplementalSource(ctx, d)
	if original == nil {
		t.Fatal("lost native supplemental linkage after review")
	}
	if _, found := reviewAssessment(d, ""); !found {
		t.Fatal("current supplemental review not bound")
	}
	if !task.IsUnpublishedReviewedOutcome(&d.Task, original.LatestAttempt, d.Activities) {
		t.Fatalf("source did not retain reviewed unpublished shape: %s", proof.SourceAttemptID)
	}
	return d
}

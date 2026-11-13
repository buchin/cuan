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

func TestVerificationRuntimeRecoveryRequiresChangedCapabilityAndCannotChain(t *testing.T) {
	for _, scenario := range []string{"ready", "unchanged", "missing", "canceled", "missing completion evidence", "stale", "changed scope", "wrong runtime receipt"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, q := stalledReviewFixture(t, "completed")
			verificationRuntimeRecoveryContract(t, s, d, q, scenario)
		})
	}
}
func TestVerificationRuntimeRecoveryPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, q, d := phaseLineageSetup(t, database)
		verificationRuntimeRecoveryContract(t, s, d, q, "ready")
	})
}
func verificationRuntimeRecoveryContract(t *testing.T, s *Server, d *task.TaskDetailResponse, q db.InboxRequest, scenario string) {
	t.Helper()
	ctx := context.Background()
	verificationReviewReceiptContract(t, s, d, q)
	store := s.taskSvc.Store()
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UTC()
	for _, event := range []struct{ id, kind, state string }{{"verifier-final", "agent_report", "review"}, {"verifier-kind", "agent_result_kind", "implementation"}} {
		if err = store.CreateActivity(ctx, &task.TaskActivity{ID: event.id, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: event.kind, NewStatus: event.state, Message: "Existing report", CreatedAt: stamp}); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.CreateTask(ctx, &task.Task{ID: "runtime-busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	parent := d.LatestAttempt.ID
	q.RequestID = db.GenerateUUID()
	assessment := inboxReviewEvaluation{VisualInspection: "Both supplied images show synthetic mockups rather than the running application.", Decision: "revise", Finding: "Screenshot aplikasi nyata belum tersedia.", RemainingWork: "verification", Explanation: "Not accepted.", Revision: "Capture actual application screenshots at390px and desktop using the native browser, then inspect them."}
	if _, err = s.saveReviewAssessment(ctx, d, q, assessment); err != nil {
		t.Fatal(err)
	}
	s.runnerSvc = runner.NewService(runner.NewMemoryStore(store), store, nil)
	runtime := &task.VerificationRuntime{Fingerprint: strings.Repeat("a", 64), Version: "fixture-browser", CheckedAt: time.Now().UTC()}
	if scenario == "stale" {
		runtime.CheckedAt = time.Now().Add(-time.Hour)
	}
	if scenario != "missing" {
		if _, err = s.runnerSvc.Heartbeat(ctx, runner.HeartbeatRequest{RunnerID: d.LatestAttempt.RunnerID, BrowserRuntime: runtime}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "unchanged" {
		raw, _ := json.Marshal(runtime)
		if err = store.CreateActivity(ctx, &task.TaskActivity{ID: "prior-runtime", TaskID: d.ID, AttemptID: &parent, ActivityType: task.ActivityVerificationRuntime, Author: d.LatestAttempt.RunnerID, Message: string(raw), CreatedAt: stamp}); err != nil {
			t.Fatal(err)
		}
		d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "wrong runtime receipt" {
		raw, _ := json.Marshal(task.VerificationRuntime{Fingerprint: strings.Repeat("b", 64), Version: "fixture", CheckedAt: time.Now()})
		if err = store.CreateActivity(ctx, &task.TaskActivity{ID: "conflicting-runtime", TaskID: d.ID, AttemptID: &parent, ActivityType: task.ActivityVerificationRuntimeRecovery, Author: d.LatestAttempt.RunnerID, Message: string(raw)}); err != nil {
			t.Fatal(err)
		}
		d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "changed scope" {
		d.Instructions += "different scope"
	}
	if scenario == "canceled" {
		d.Status = "cancelled"
	}
	outcome, scheduled := s.startVerificationFollowup(ctx, q, d, assessment)
	want := scenario == "ready" || scenario == "missing completion evidence" || scenario == "timeout"
	if scheduled != want {
		t.Fatalf("scheduled=%v want=%v outcome=%s", scheduled, want, outcome)
	}
	if !want {
		return
	}
	for i := 0; i < 3; i++ {
		if _, ok := s.startVerificationFollowup(ctx, q, d, assessment); !ok {
			t.Fatal("same intent did not deduplicate")
		}
	}
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRecoveryRequestID(d.ID, d.CommitSHA)))
	if err != nil || job.State != "queued" {
		t.Fatal("missing queued recovery", err)
	}
	proof, err := task.VerificationIntentForLaunch(&d.Task, job)
	if err != nil || proof == nil || proof.RecoveryAttemptID != parent || proof.RuntimeFingerprint != runtime.Fingerprint || proof.SourceAttemptID == parent {
		t.Fatal("recovery lost original implementation proof", err)
	}
	current, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || len(current.Attempts) != 4 {
		t.Fatal("recovery bypassed project lane", err)
	}
	if err = store.DeleteTask(ctx, "runtime-busy"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "runtime-recovery", job.ID); err != nil {
		t.Fatal(err)
	}
	att, err := s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: d.LatestAttempt.RunnerID, AgentName: d.AssignedAgent, RequestID: job.RequestID, OwnerAuthorized: true})
	if err != nil {
		t.Fatal(err)
	}
	claim, err := s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: d.LatestAttempt.RunnerID}, time.Minute)
	if err != nil || !claim.Claimed || claim.Verification == nil {
		t.Fatal("recovery claim invalid", err)
	}
	req := runner.CompleteAttemptRequest{RunnerID: d.LatestAttempt.RunnerID, Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: d.CommitSHA, VerificationRuntimeFingerprint: runtime.Fingerprint, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("b", 64), Final: strings.Repeat("b", 64)}}
	if scenario == "timeout" {
		req.Status = task.AttemptStatusFailed
		req.ExitCode = 124
		req.ErrorMessage = "Execution timed out"
	}
	if scenario == "missing completion evidence" {
		req.VerificationRuntimeFingerprint = ""
	}
	completed, err := s.runnerSvc.Complete(ctx, att.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	expected := task.AttemptStatusSucceeded
	if scenario == "missing completion evidence" || scenario == "timeout" {
		expected = task.AttemptStatusFailed
	}
	if completed.Status != expected {
		t.Fatalf("completion=%s want=%s", completed.Status, expected)
	}
	current, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status == task.TaskStatusDone {
		t.Fatal("runtime recovery bypassed acceptance")
	}
	if s.verificationRuntimeRecoveryProof(ctx, q, current, proof, assessment) != nil {
		t.Fatal("chained runtime recovery")
	}
	if taskRevisionAttemptCount(s.reviewBudgetDetail(ctx, current)) < 3 {
		t.Fatal("runtime recovery reset code correction budget")
	}
}

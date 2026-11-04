package server

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestVerificationFinalizationUsesMachineChecksWithoutAIOrFalseAcceptance(t *testing.T) {
	for _, scenario := range []string{"valid", "old completion", "changed repository", "canceled", "changed scope", "missing baseline", "changed artifact"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, q := stalledReviewFixture(t, "completed")
			verificationFinalizationContract(t, s, d, q, scenario, nil)
		})
	}
}
func TestVerificationFinalizationPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, q, d := phaseLineageSetup(t, database)
		verificationFinalizationContract(t, s, d, q, "valid", database)
	})
}
func verificationFinalizationContract(t *testing.T, s *Server, d *task.TaskDetailResponse, q db.InboxRequest, scenario string, database *sql.DB) {
	t.Helper()
	ctx := context.Background()
	stagedVerificationRecoveryContract(t, s, d, q, "saved")
	store := s.taskSvc.Store()
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	originID := d.LatestAttempt.ID
	q.RequestID = db.GenerateUUID()
	assessment := inboxReviewEvaluation{VisualInspection: "The saved images are available for visual review.", Decision: "revise", Finding: "Laporan akhir dan integritas repository belum tercatat.", RemainingWork: "verification", Explanation: "Not accepted.", Revision: "Verify the unchanged source and finalize the saved output references."}
	if _, err = s.saveReviewAssessment(ctx, d, q, assessment); err != nil {
		t.Fatal(err)
	}
	if err = s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CreateTask(ctx, &task.Task{ID: "finalization-busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	switch scenario {
	case "canceled":
		d.Status = "cancelled"
	case "changed scope":
		d.Instructions += "changed scope"
	case "missing baseline":
		var filtered []*task.TaskActivity
		for _, a := range d.Activities {
			if a.ActivityType != "repository_scope_verified" {
				filtered = append(filtered, a)
			}
		}
		d.Activities = filtered
	case "changed artifact":
		m, e := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-delivery:"+originID))
		if e != nil {
			t.Fatal(e)
		}
		_, ids := decodeAttachmentMessage(m.Content)
		_, path, e := loadAttachment("owner", ids[0])
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, []byte("changed bytes"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	output, scheduled := s.startVerificationFinalization(ctx, q, d, assessment)
	eligible := scenario == "valid" || scenario == "old completion" || scenario == "changed repository"
	if scheduled != eligible {
		t.Fatalf("scheduled=%v expected=%v output=%s", scheduled, eligible, output)
	}
	if !eligible {
		return
	}
	for i := 0; i < 3; i++ {
		if _, ok := s.startVerificationFinalization(ctx, q, d, assessment); !ok {
			t.Fatal("finalization intent did not deduplicate")
		}
	}
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationFinalizeRequestID(d.ID, d.CommitSHA)))
	if err != nil || job.State != "queued" {
		t.Fatal("missing finalization intent", err)
	}
	proof, err := task.VerificationIntentForLaunch(&d.Task, job)
	if err != nil || proof == nil || proof.FinalizeAttemptID != originID || proof.RepositoryBaseline != strings.Repeat("b", 64) {
		t.Fatal("lost finalization source proof", err)
	}
	if err = store.DeleteTask(ctx, "finalization-busy"); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "finalization-launch", job.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.executeQueuedTaskLaunch(ctx, *claimed, false)
	current, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || len(current.Attempts) != 6 || current.LatestAttempt.ID == originID || current.LatestAttempt.RunnerID != "cuan-runner" {
		t.Fatal("finalization required a model or lost native routing", err)
	}
	var runnerStore runner.Store = runner.NewMemoryStore(store)
	if database != nil {
		runnerStore = runner.NewDBStore(database)
	}
	service := runner.NewService(runnerStore, store, nil)
	unsupported, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "finalizer"}, time.Minute)
	if err != nil || unsupported.Claimed {
		t.Fatal("legacy worker claimed a machine-only job", err)
	}
	stillQueued, err := store.GetLatestAttempt(ctx, d.ID)
	if err != nil || stillQueued.Status != task.AttemptStatusQueued {
		t.Fatal("unsupported worker consumed finalization intent", err)
	}
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "finalizer", SupportsVerificationFinalization: true}, time.Minute)
	if err != nil || !claim.Claimed || claim.Verification == nil || claim.Verification.FinalizeAttemptID != originID {
		t.Fatal("machine claim failed", err)
	}
	if claim.Task.Instructions != "" || claim.Environment != "" || claim.AutoDeploy || claim.TaskAgent != nil {
		t.Fatal("machine claim leaked model context, environment or capability")
	}
	req := runner.CompleteAttemptRequest{RunnerID: "finalizer", Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: proof.SourceCommit, FinalizationManifestHash: proof.ArtifactManifestHash, RepositoryVerification: &runner.RepositoryVerification{Baseline: proof.RepositoryBaseline, Final: proof.RepositoryBaseline}}
	if scenario == "old completion" {
		req.FinalizationManifestHash = ""
	}
	if scenario == "changed repository" {
		req.RepositoryVerification.Final = strings.Repeat("c", 64)
	}
	completed, err := service.Complete(ctx, claim.Attempt.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	want := task.AttemptStatusSucceeded
	if scenario != "valid" {
		want = task.AttemptStatusFailed
	}
	if completed.Status != want {
		t.Fatalf("completion=%s want=%s", completed.Status, want)
	}
	current, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	old, err := store.GetAttempt(ctx, originID)
	if err != nil || old.Status != task.AttemptStatusFailed || *old.ExitCode != 124 {
		t.Fatal("rewrote original timeout", err)
	}
	if current.Status == task.TaskStatusDone {
		t.Fatal("machine check bypassed independent acceptance")
	}
	if scenario != "valid" {
		return
	}
	if evidence := inboxReportEvidence(current); evidence.Provenance != "current_runner_finalization" || evidence.AttemptID != claim.Attempt.ID {
		t.Fatal("machine report confused with agent report", evidence)
	}
	if ok, err := s.linkFinalizationArtifacts(ctx, current); err != nil || !ok {
		t.Fatal("lost original output references", err)
	}
	if images := s.inboxReviewImages(ctx, current); len(images) != 1 || images[0].Width != 390 {
		t.Fatal("visual review omitted original images", images)
	}
	if m, _ := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-delivery:"+claim.Attempt.ID)); m != nil {
		t.Fatal("duplicated Inbox files")
	}
	if context := s.verificationReviewContext(ctx, current); context == nil || context["finalized_outputs_from_attempt_id"] != originID {
		t.Fatal("lost original output provenance")
	}
	for _, a := range current.Activities {
		if a.AttemptID != nil && *a.AttemptID == claim.Attempt.ID && a.ActivityType == "agent_report" {
			t.Fatal("fabricated AI report")
		}
	}
	if taskRevisionAttemptCount(s.reviewBudgetDetail(ctx, current)) < 3 {
		t.Fatal("finalization reset correction budget")
	}
}

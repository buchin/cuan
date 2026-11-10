package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestNativeCheckRecipeCorrectionContinuesOriginalWork(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "initial verify check recipe")
}

func verifyChangedNativeCheckRecipe(t *testing.T, s *Server, current *task.TaskDetailResponse, claimed *runner.ClaimTaskResponse, review db.InboxRequest, assessment inboxReviewEvaluation, origins []db.InboxRequest) {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	id := claimed.Attempt.ID
	for _, event := range []*task.TaskActivity{
		{ID: "recipe-report", TaskID: current.ID, AttemptID: &id, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Current outcome inspected; native checks pending."},
		{ID: "recipe-kind", TaskID: current.ID, AttemptID: &id, ActivityType: "agent_result_kind", NewStatus: "implementation"},
	} {
		must(s.taskSvc.Store().CreateActivity(ctx, event))
	}
	hash := strings.Repeat("a", 64)
	_, err := s.runnerSvc.Complete(ctx, id, runner.CompleteAttemptRequest{RunnerID: "stale-verifier", Status: task.AttemptStatusFailed, ExitCode: 0, ErrorMessage: task.InspectionProjectVerificationTimeout, TestSummary: "Project verification: test.sh --fast (exit -1)\nNative verifier result: Project verification timed out after 5 minutes", RepositoryVerification: &runner.RepositoryVerification{Baseline: hash, Final: hash}})
	must(err)
	must(s.taskSvc.ReconcileOrphanLaunches(ctx, time.Now().UTC()))
	failed, err := s.taskSvc.GetTaskDetail(ctx, current.ID)
	must(err)
	review.RequestID = db.GenerateUUID()
	_, err = s.saveReviewAssessment(ctx, failed, review, assessment)
	must(err)
	failed, err = s.taskSvc.GetTaskDetail(ctx, current.ID)
	must(err)
	if s.recoverNativeCheckRecipe(ctx, failed, origins) {
		t.Fatal("unchanged recipe renewed execution")
	}
	p, err := s.projectSvc.Get(ctx, current.ProjectID)
	must(err)
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", p.LocalPath}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git: %s", out)
		}
		return strings.TrimSpace(string(out))
	}
	git("checkout", "--detach", s.cfg.CommitSHA)
	must(os.WriteFile(filepath.Join(p.LocalPath, "unrelated.txt"), []byte("unrelated release"), 0600))
	git("add", "unrelated.txt")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Unrelated change")
	s.cfg.CommitSHA = git("rev-parse", "HEAD")
	if s.recoverNativeCheckRecipe(ctx, failed, origins) {
		t.Fatal("deployment alone renewed execution")
	}
	must(os.MkdirAll(filepath.Join(p.LocalPath, "scripts"), 0700))
	must(os.WriteFile(filepath.Join(p.LocalPath, "scripts/test.sh"), []byte("#!/bin/sh\nexit 0\n# corrected native recipe\n"), 0700))
	git("add", "scripts/test.sh")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Correct registered check recipe")
	s.cfg.CommitSHA = git("rev-parse", "HEAD")
	blocker := &task.Task{ID: "recipe-lane-blocker", ProjectID: current.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}
	must(s.taskSvc.Store().CreateTask(ctx, blocker))
	if diagnostic := s.liveInspectionPlanDiagnostic(ctx, failed); diagnostic.Code != "native_check_recipe_ready" || diagnostic.TargetCommit != s.cfg.CommitSHA {
		t.Fatal("recovery diagnostic hides actual plan", diagnostic)
	}
	if !s.recoverNativeCheckRecipe(ctx, failed, origins) {
		t.Fatal("corrected recipe stranded original task")
	}
	job, err := s.taskSvc.LatestLaunch(ctx, current.ID)
	must(err)
	proof, err := task.LiveInspectionForLaunch(&failed.Task, job)
	must(err)
	if proof == nil || !task.NativeCheckRecipeRevalidation(proof) || proof.PredecessorAttemptID() != id || proof.SourceCommit != current.CommitSHA || proof.TargetCommit != s.cfg.CommitSHA || job.State != "queued" {
		t.Fatalf("invalid retained handoff: %+v %+v", job, proof)
	}
	must(task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &failed.Task, proof))
	if !strings.Contains(job.Revision, "exact registered recipe changed") {
		t.Fatal("verifier missing concrete correction context")
	}
	for _, mode := range []string{"owner", "source", "review", "paths", "same target", "write flag"} {
		t.Run(mode, func(t *testing.T) {
			altered := *proof
			switch mode {
			case "owner":
				altered.OwnerID = "foreign"
			case "source":
				altered.SourceCommit = strings.Repeat("f", 40)
			case "review":
				altered.ReviewRequestID = "foreign"
			case "paths":
				altered.ReviewPaths = []string{"internal/other.go"}
			case "same target":
				altered.TargetCommit = altered.PreviousCommit
			case "write flag":
				altered.ReportingRecoveryAttemptID = id
			}
			if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &failed.Task, &altered) == nil {
				t.Fatal("invalid recovery accepted")
			}
		})
	}
	key := proof.RequestID(current.ID)
	later := *proof
	later.TargetCommit = strings.Repeat("e", 40)
	if later.RequestID(current.ID) != key {
		t.Fatal("deploy changes renew recovery identity")
	}
	s.recoverNativeCheckRecipe(ctx, failed, origins)
	after, err := s.taskSvc.GetTaskDetail(ctx, current.ID)
	must(err)
	if len(after.Attempts) != len(failed.Attempts) || after.Status == task.TaskStatusDone || after.LatestAttempt.ID != id {
		t.Fatal("scheduling duplicated attempts or fabricated acceptance")
	}

	must(s.taskSvc.Store().DeleteTask(ctx, blocker.ID))
	verifyNativeInspectionCompletionAndAcceptance(t, s, &failed.Task, id, job, proof)
	final, err := s.taskSvc.GetTaskDetail(ctx, current.ID)
	must(err)
	if final.Status != task.TaskStatusDone || final.CommitSHA != current.CommitSHA {
		t.Fatal("corrected check handoff failed original acceptance or changed source")
	}
}

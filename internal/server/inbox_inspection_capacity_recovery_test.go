package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestInspectionCapacityContinuation(t *testing.T) {
	inspectionCapacityContinuationContract(t, nil)
}

func TestInspectionCapacityContinuationPostgres(t *testing.T) {
	for _, mode := range []string{"capacity", "infrastructure launch capacity"} {
		t.Run(mode, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) { inspectionCapacityContinuationContract(t, database, mode) })
		})
	}
}

func inspectionCapacityContinuationContract(t *testing.T, database *sql.DB, modes ...string) {
	if len(modes) == 0 {
		modes = []string{"capacity", "infrastructure launch capacity"}
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			plan := inboxApprovalPlan{Title: "Verify feature", Description: "Implement and independently verify the feature", ProjectID: "default"}
			var databases []*sql.DB
			if database != nil {
				databases = append(databases, database)
			}
			s, origin, d := phaseLineageSetupForAgent(t, plan, db.AgentIDCuan, databases...)
			liveInspectionContract(t, s, origin, d, "current outcome unpublished valid "+mode, database)
		})
	}
}

func testInspectionCapacityContinuation(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse, claim *runner.ClaimTaskResponse, service *runner.RunnerService) {
	t.Helper()
	ctx := context.Background()
	store := s.taskSvc.Store()
	parentID := claim.Attempt.ID
	previous := *claim.LiveInspection
	failure := "The AI provider reported a usage limit. The run was stopped; saved work will be continued when capacity is available."
	if _, err := service.Complete(ctx, parentID, runner.CompleteAttemptRequest{RunnerID: claim.Attempt.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 75, ErrorMessage: failure}); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A real local repository provides the newly observed target. No network,
	// provider inference, fabricated screenshot or production credentials.
	root := t.TempDir()
	gitRun := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	gitRun("init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Actual current feature"), 0600); err != nil {
		t.Fatal(err)
	}
	gitRun("add", ".")
	gitRun("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Observed result")
	s.cfg.CommitSHA = gitRun("rev-parse", "HEAD")
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: d.ProjectID, Name: "Fixture", RepoURL: "https://github.com/fixture/repo", LocalPath: root, DefaultBranch: "main", CloneStatus: "synced"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, git.NewClient(), "fixture", t.TempDir())
	s.taskSvc = task.NewService(store, s.projectSvc)
	proof := previous
	proof.CapacityRecoveryAttemptID, proof.TargetCommit, proof.ObservedAt = parentID, s.cfg.CommitSHA, time.Now().UTC()
	if err := task.ValidateLiveInspectionSource(ctx, store, &d.Task, &proof); err != nil {
		t.Fatal("capacity continuation invalid", err)
	}
	for _, bad := range []string{"owner", "repository", "source", "scope", "parent", "stale observation", "wrong purpose"} {
		altered := proof
		switch bad {
		case "owner":
			altered.OwnerID = "other"
		case "repository":
			altered.Repository = "other/repo"
		case "source":
			altered.SourceCommit = strings.Repeat("e", 40)
		case "scope":
			altered.InstructionsHash = strings.Repeat("e", 64)
		case "parent":
			altered.CapacityRecoveryAttemptID = previous.SourceAttemptID
		case "stale observation":
			altered.ObservedAt = previous.ObservedAt
		case "wrong purpose":
			altered.Purpose = task.NativeReleaseInspection
		}
		if task.ValidateLiveInspectionSource(ctx, store, &d.Task, &altered) == nil {
			t.Fatal("capacity continuation expanded authority", bad)
		}
	}
	foreign := origin
	foreign.OwnerID = "other"
	if s.recoverCapacityInspection(ctx, d, foreign) {
		t.Fatal("foreign origin continued inspection")
	}
	if !s.recoverLiveResultInspection(ctx, d, origin) {
		t.Fatal("no-account capacity wait was not persisted")
	}
	id := task.LaunchID(d.ID, proof.RequestID(d.ID))
	queued, err := s.taskSvc.GetLaunch(ctx, id)
	if err != nil || queued == nil || queued.State != "queued" {
		t.Fatal("missing durable capacity wait", err)
	}
	bound, err := task.LiveInspectionForLaunch(&d.Task, queued)
	if err != nil || bound.CapacityRecoveryAttemptID != parentID || bound.VerifierRecoveryAttemptID != previous.VerifierRecoveryAttemptID || bound.SourceAttemptID != previous.SourceAttemptID || bound.PredecessorAttemptID() != parentID {
		t.Fatal("capacity continuation lost stage/source binding", err)
	}
	for i := 0; i < 3; i++ {
		reserved, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now().Add(time.Hour), "capacity-wait", id)
		if err != nil || reserved == nil {
			t.Fatal("cannot poll saved wait", err)
		}
		s.executeQueuedTaskLaunch(ctx, *reserved, false)
		latest, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
		if err != nil || latest.Status != task.TaskStatusQueued || latest.LatestAttempt.ID != parentID || len(latest.Attempts) != len(d.Attempts) {
			t.Fatal("capacity poll created execution or lost queued state", err)
		}
		queued, err = s.taskSvc.GetLaunch(ctx, id)
		if err != nil || queued.State != "queued" {
			t.Fatal("capacity poll discarded saved work", err)
		}
	}
	// Make one permitted selected-model route ready; the same saved launch must
	// create exactly one new attempt, without a user message or manual task retry.
	account, err := agent.CreateProviderAccount(origin.OwnerID, "Ready fixture")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := agent.ProviderAccountHome(origin.OwnerID, account.ID)
	if err := os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-only-credentials"), 0600); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", account.ID, "account.json")
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	meta["state"], meta["model_id"] = "verified", "claude-sonnet-test"
	raw, _ = json.Marshal(meta)
	if err := os.WriteFile(metaPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	reserved, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now().Add(time.Hour), "capacity-ready", id)
	if err != nil || reserved == nil {
		t.Fatal("ready launch unavailable", err)
	}
	text := s.executeQueuedTaskLaunch(ctx, *reserved, false)
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || len(after.Attempts) != len(d.Attempts)+1 || after.LatestAttempt.ID == parentID || after.LatestAttempt.Status != task.AttemptStatusQueued {
		t.Fatalf("ready capacity did not continue exact saved work: %s %+v %v", text, after.LatestAttempt, err)
	}
	_, _, release, err := agent.LeaseHealthyBrainForTask(origin.OwnerID, nil)
	if err != nil {
		t.Fatal("admission kept worker slot", err)
	}
	release()
	if after.Instructions != d.Instructions || after.CommitSHA != d.CommitSHA || after.Status == task.TaskStatusDone {
		t.Fatal("capacity wait changed original scope or accepted result")
	}
	service.SetProjectFinder(s.projectSvc)
	next, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "capacity-worker", SupportsNativeVisualVerification: true, SupportsLiveInspection: true, SupportsLiveInspectionRecovery: true, SupportsCurrentOutcomeInspection: true}, time.Minute)
	if err != nil || !next.Claimed || next.Attempt.ID != after.LatestAttempt.ID || next.LiveInspection.CapacityRecoveryAttemptID != parentID || !next.PublicationLimits.ReadOnlyRepository || !next.PublicationLimits.DenyPush || !next.PublicationLimits.DenyDeploy {
		t.Fatalf("capacity continuation invalid at worker claim: %+v %v", next, err)
	}
	if previous.VerifierRecoveryAttemptID == "" {
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "capacity-final-report", TaskID: d.ID, AttemptID: &next.Attempt.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Actual current feature checks complete", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		result, err := service.Complete(ctx, next.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: "capacity-worker", Status: task.AttemptStatusSucceeded, ExitCode: 0, CurrentOutcomeVerified: true, LiveInspectionCommit: bound.TargetCommit, AgentReportRequired: true, AgentReportConfirmed: true, TestSummary: "Project verification: test.sh --fast (exit 0)\nFresh checks", RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("c", 64), Final: strings.Repeat("c", 64)}})
		if err != nil || result.Status != task.AttemptStatusSucceeded {
			t.Fatal("capacity completion rejected fresh verification", result, err)
		}
	} else {
		if _, err := service.Complete(ctx, next.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: "capacity-worker", Status: task.AttemptStatusFailed, ExitCode: 75, ErrorMessage: failure}); err != nil {
			t.Fatal(err)
		}
		repeated, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
		if err != nil || s.recoverCapacityInspection(ctx, repeated, origin) {
			t.Fatal("capacity continuation renewed its retry allowance", err)
		}
	}
	final, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || final.Status == task.TaskStatusDone {
		t.Fatal("resource continuation bypassed independent acceptance", err)
	}
	old, err := store.GetAttempt(ctx, parentID)
	if err != nil || old.Status != task.AttemptStatusFailed || old.ErrorMessage != failure {
		t.Fatal("capacity wait rewrote failed history", err)
	}
}

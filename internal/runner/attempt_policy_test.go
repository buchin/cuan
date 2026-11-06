package runner

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/executionpolicy"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunnerContinuationPublicationLimitsBindClaimAndCompletion(t *testing.T) {
	ctx := context.Background()
	store := task.NewMemoryStore()
	tasks := task.NewService(store, nil)
	created, e := tasks.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Verify existing implementation", ProjectID: "project", Instructions: "Original approved implementation"})
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	requestID := "91000000-0000-4000-8000-000000000002"
	job, e := store.SaveLaunch(ctx, task.TaskLaunch{TaskID: created.ID, ProjectID: "project", RequestID: requestID, SourceRequestID: "owner", OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", AgentID: "agent", InstructionsHash: task.LaunchInstructionsHash(created.Instructions), Revision: "Do not push or deploy. Inspect existing files and report actual evidence.", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = store.ClaimLaunch(ctx, now.Add(time.Millisecond), "launch-worker", job.ID); e != nil {
		t.Fatal(e)
	}
	a, e := tasks.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "fixture-runner", AgentName: "agent", RequestID: requestID})
	if e != nil {
		t.Fatal(e)
	}
	finder := &mockProjectFinder{projects: map[string]*project.ProjectResponse{"project": {ID: "project", Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`)}}}
	svc := NewService(NewMemoryStore(store), store, finder)
	claim, e := svc.Claim(ctx, ClaimTaskRequest{RunnerID: "fixture-runner"}, time.Minute)
	if e != nil || !claim.Claimed {
		t.Fatal(claim, e)
	}
	if claim.AutoDeploy || !claim.PublicationLimits.DenyDeploy || !claim.PublicationLimits.DenyPush {
		t.Fatal("claim lost explicit limits", claim.PublicationLimits)
	}
	result, e := svc.Complete(ctx, a.ID, CompleteAttemptRequest{RunnerID: "fixture-runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, CommitSHA: "previous-implementation", AgentReportRequired: true, AgentReportConfirmed: true})
	if e != nil || result.Status != task.AttemptStatusSucceeded {
		t.Fatal("completion demanded forbidden deployment", result, e)
	}
	// Success here is execution evidence; it does not fabricate checklist/live acceptance.
}

func TestRevisionNoPushPreventsAutomaticCommit(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(string(out), err)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=main")
	os.WriteFile(filepath.Join(root, "guide.md"), []byte("saved\n"), 0600)
	run("add", "guide.md")
	run("commit", "-m", "Saved work")
	head := run("rev-parse", "HEAD")
	os.WriteFile(filepath.Join(root, "guide.md"), []byte("pending changes\n"), 0600)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer api.Close()
	runner := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL}, nil)
	outcome := CompleteAttemptRequest{Status: task.AttemptStatusSucceeded}
	runner.postExecutionGitFlow(context.Background(), &cuangit.TaskWorkspace{Path: root, BranchName: "main"}, &task.Task{ID: "task", Title: "Keep local work"}, &task.TaskAttempt{ID: "attempt"}, ClaimTaskResponse{PublicationLimits: executionpolicy.PublicationPolicy{DenyPush: true}}, &outcome, task.ExecutionCategoryCode)
	if run("rev-parse", "HEAD") != head || run("status", "--porcelain") == "" {
		t.Fatal("revision prohibition lost local changes through automatic commit")
	}
}

func TestReadOnlyContinuationCompletionRequiresRunnerRepositoryEvidence(t *testing.T) {
	for _, scenario := range []string{"missing", "changed", "unchanged", "published", "timeout", "timeout_changed"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			svc := task.NewService(store, nil)
			created, err := svc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Audit existing code", ProjectID: "project", Instructions: "Approved original task", ExecutionCategory: task.ExecutionCategoryCode})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			requestID := "91000000-0000-4000-8000-000000000002"
			job, err := store.SaveLaunch(ctx, task.TaskLaunch{TaskID: created.ID, ProjectID: "project", RequestID: requestID, SourceRequestID: "owner", OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", AgentID: "agent", InstructionsHash: task.LaunchInstructionsHash(created.Instructions), Revision: "Binding approved continuation:\nLanjutkan audit read-only pada repositori tanpa perubahan kode, konfigurasi, push, atau deploy.\nAutomatic correction feedback:\nFix scripts/test.sh", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.ClaimLaunch(ctx, now, "launch", job.ID); err != nil {
				t.Fatal(err)
			}
			att, err := svc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "fixture", AgentName: "agent", RequestID: requestID})
			if err != nil {
				t.Fatal(err)
			}
			runnerSvc := NewService(NewMemoryStore(store), store, nil)
			claim, err := runnerSvc.Claim(ctx, ClaimTaskRequest{RunnerID: "fixture"}, time.Minute)
			if err != nil || !claim.Claimed || !claim.PublicationLimits.ReadOnlyRepository || !claim.PublicationLimits.DenyPush || !claim.PublicationLimits.DenyDeploy {
				t.Fatalf("claim lost scope: %+v %v", claim, err)
			}
			req := CompleteAttemptRequest{RunnerID: "fixture", Status: task.AttemptStatusSucceeded, AgentReportConfirmed: true, AgentReportRequired: true}
			if scenario != "missing" {
				req.RepositoryVerification = &RepositoryVerification{Baseline: strings.Repeat("a", 64), Final: strings.Repeat("a", 64)}
			}
			if scenario == "changed" || scenario == "timeout_changed" {
				req.RepositoryVerification.Final = strings.Repeat("b", 64)
			}
			if strings.HasPrefix(scenario, "timeout") {
				req.Status, req.ExitCode, req.ErrorMessage = task.AttemptStatusFailed, 124, "Execution timed out after adaptive timeout (1m0s)"
				req.AgentReportConfirmed = false
			}
			if scenario == "published" {
				req.PRNumber = 15
				req.CommitSHA = "new-commit"
			}
			result, err := runnerSvc.Complete(ctx, att.ID, req)
			if err != nil {
				t.Fatal(err)
			}
			want := task.AttemptStatusFailed
			if scenario == "unchanged" {
				want = task.AttemptStatusSucceeded
			}
			if result.Status != want {
				t.Fatalf("got %s want %s", result.Status, want)
			}
			detail, err := svc.GetTaskDetail(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			verified := false
			observed := false
			for _, act := range detail.Activities {
				if act.ActivityType == "repository_scope_verified" && act.AttemptID != nil && *act.AttemptID == att.ID {
					verified = true
				}
				if act.ActivityType == "repository_scope_observed" && act.AttemptID != nil && *act.AttemptID == att.ID && act.Author == "fixture" {
					observed = true
				}
			}
			if verified != (scenario == "unchanged") {
				t.Fatal("incorrect persisted scope evidence")
			}
			if strings.HasPrefix(scenario, "timeout") && (!observed || verified || detail.Status == task.TaskStatusDone || result.ExitCode == nil || *result.ExitCode != 124) {
				t.Fatal("failed execution lost its scope observation or became accepted")
			}
		})
	}
}

func TestLegacyManagedFooterPreservesDeploymentReceiptGate(t *testing.T) {
	for _, scenario := range []string{"missing receipt", "verified receipt", "owner forbids deployment"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			tasks := task.NewService(store, nil)
			ownerScope := "Implement and deploy the feature to production."
			if scenario == "owner forbids deployment" {
				ownerScope = "Do not deploy. Return local results only."
			}
			original := "Cuan managed delegation v1.\nImplement the approved feature.\nApproval message: approved-message\nUser authorization: " + ownerScope + "\nConversation: 91000000-0000-4000-8000-000000000001\nReport actual progress and verification evidence. Stay within this scope; do not deploy unless authorized."
			created, err := tasks.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Implement approved feature", ProjectID: "project", Instructions: original, ExecutionCategory: task.ExecutionCategoryCode})
			if err != nil {
				t.Fatal(err)
			}
			a, err := tasks.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "fixture", AgentName: "agent"})
			if err != nil {
				t.Fatal(err)
			}
			finder := &mockProjectFinder{projects: map[string]*project.ProjectResponse{"project": {ID: "project", Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`)}}}
			svc := NewService(NewMemoryStore(store), store, finder)
			claim, err := svc.Claim(ctx, ClaimTaskRequest{RunnerID: "fixture"}, time.Minute)
			if err != nil || !claim.Claimed {
				t.Fatal(claim, err)
			}
			denied := scenario == "owner forbids deployment"
			if claim.AutoDeploy == denied || claim.PublicationLimits.DenyDeploy != denied {
				t.Fatalf("incorrect publication boundary: %+v", claim.PublicationLimits)
			}
			commit := strings.Repeat("c", 40)
			changed := true
			req := CompleteAttemptRequest{RunnerID: "fixture", Status: task.AttemptStatusSucceeded, ExitCode: 0, CommitSHA: commit, AgentReportRequired: true, AgentReportConfirmed: true, HasCodeChanges: &changed, RequiresDeploy: &changed}
			if scenario == "verified receipt" {
				req.LiveDeploymentConfirmed = true
				receipt, _ := json.Marshal(task.DeploymentReceipt{Version: 1, TaskID: created.ID, AttemptID: a.ID, CommitSHA: commit, ReleaseSHA: commit, Environment: "production", Health: "healthy"})
				if err := store.CreateActivity(ctx, &task.TaskActivity{ID: task.DeploymentReceiptID(created.ID, a.ID, commit), TaskID: created.ID, AttemptID: &a.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(receipt), CreatedAt: time.Now().UTC()}); err != nil {
					t.Fatal(err)
				}
			}
			result, err := svc.Complete(ctx, a.ID, req)
			if err != nil {
				t.Fatal(err)
			}
			want := task.AttemptStatusSucceeded
			if scenario == "missing receipt" {
				want = task.AttemptStatusFailed
			}
			if result.Status != want {
				t.Fatalf("deployment gate bypassed: %+v", result)
			}
			saved, err := store.GetTask(ctx, created.ID)
			if err != nil || saved.Instructions != original {
				t.Fatal("legacy interpretation rewrote approved instructions")
			}
			if scenario == "missing receipt" && (saved.CommitSHA != commit || saved.Status == task.TaskStatusDone || task.VerifiedDeployment(nil, result, commit, time.Now().UTC())) {
				t.Fatal("publication failure must retain the result identity without accepting or inventing deployment")
			}
		})
	}
}

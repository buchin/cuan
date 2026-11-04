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

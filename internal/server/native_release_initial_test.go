package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/release"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func verifyInitialNativePreparation(t *testing.T, s *Server, token string, item *task.Task, claim *runner.ClaimTaskResponse, intent task.NativeReleaseIntent, target, scenario string) {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.projectSvc.Get(ctx, item.ProjectID)
	must(err)
	realGit, err := exec.LookPath("git")
	must(err)
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(realGit, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git: %s", out)
		}
		return strings.TrimSpace(string(out))
	}
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	run("clone", "--bare", p.LocalPath, remote)
	s.cfg.WorkspacesDir = filepath.Join(base, "tasks")
	must(os.Mkdir(s.cfg.WorkspacesDir, 0700))
	sourceRoot := filepath.Join(s.cfg.WorkspacesDir, item.ID)
	run("clone", p.LocalPath, sourceRoot)
	run("-C", sourceRoot, "checkout", "-b", "task/source", intent.SourceCommit)
	run("-C", sourceRoot, "remote", "set-url", "origin", "https://github.com/owner/repo.git")
	currentTask, err := s.taskSvc.Store().GetTask(ctx, item.ID)
	must(err)
	currentTask.BranchName = "task/source"
	must(s.taskSvc.Store().UpdateTask(ctx, currentTask))
	item.BranchName = "task/source"
	if scenario == "initial wrong source" {
		must(os.WriteFile(filepath.Join(sourceRoot, "uncommitted"), []byte("preserve"), 0600))
	}
	if scenario == "initial revoked" {
		source, err := s.taskSvc.Store().GetAttempt(ctx, intent.SourceAttemptID)
		must(err)
		source.CancelRequested = true
		must(s.taskSvc.Store().UpdateAttempt(ctx, source))
	}
	gets, merges := 0, 0
	merged := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer saved-owner" {
			t.Error("merge borrowed wrong owner")
		}
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/commits/"):
			gets++
			sha := strings.Repeat("d", 40)
			if scenario == "initial contained" || merged {
				sha = target
			}
			json.NewEncoder(w).Encode(map[string]string{"sha": sha})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/compare/"):
			gets++
			status, ancestor := "diverged", strings.Repeat("e", 40)
			if scenario == "initial contained" {
				status, ancestor = "ahead", intent.SourceCommit
			}
			json.NewEncoder(w).Encode(map[string]any{"status": status, "merge_base_commit": map[string]string{"sha": ancestor}})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/merges":
			merges++
			merged = true
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if payload["head"] != intent.SourceCommit || payload["base"] != intent.TargetBranch {
				t.Error("merge changed approved source/target")
			}
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]string{"sha": target})
		default:
			t.Errorf("unexpected native publication request %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer api.Close()
	client := git.NewClient()
	client.SetAPIBaseURL(api.URL)
	projects := s.projectSvc.Store()
	s.projectSvc = project.NewService(projects, client, "fixture", t.TempDir())
	if scenario == "initial missing verifier" {
		current, err := projects.GetProject(ctx, item.ProjectID)
		must(err)
		current.Metadata = json.RawMessage(`{"cuan_settings":{"workspace_revision":"initial","auto_deploy":true,"environment":"CUAN_MANAGED_RELEASE=1"}}`)
		must(projects.UpdateProject(ctx, current))
	}
	access := 0
	s.projectSvc.SetSharedRepositoryAccess(func(context.Context) (string, error) {
		access++
		if scenario == "initial no access" {
			return "", nil
		}
		return "saved-owner", nil
	})
	bin, calls := filepath.Join(base, "bin"), filepath.Join(base, "git-calls")
	must(os.Mkdir(bin, 0700))
	quote := func(value string) string { raw, _ := json.Marshal(value); return string(raw) }
	script := "#!/usr/bin/python3\nimport sys,os,base64\na=sys.argv[1:]\nif 'push' in a or 'ls-remote' in a:\n assert not os.environ.get('GH_TOKEN') and not os.environ.get('GITHUB_TOKEN') and not os.environ.get('SSH_AUTH_SOCK')\n assert base64.b64decode(os.environ['GIT_CONFIG_VALUE_0'].split('Basic ')[1]).decode()=='x-access-token:saved-owner'\n with open(" + quote(calls) + ",'a') as f:f.write(('push' if 'push' in a else 'observe')+'\\n')\n a=[" + quote(remote) + " if x=='https://github.com/owner/repo.git' else x for x in a]\nos.execv(" + quote(realGit) + ",['git']+a)\n"
	must(os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("GH_TOKEN", "wrong-host-token")
	t.Setenv("SSH_AUTH_SOCK", "wrong-host-agent")
	request := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"task_id": item.ID, "runner_id": claim.Attempt.RunnerID})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/runner/attempts/"+claim.Attempt.ID+"/release/prepare", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		s.Router().ServeHTTP(response, req)
		return response
	}
	var response *httptest.ResponseRecorder
	if scenario == "initial merge" {
		responses := make(chan *httptest.ResponseRecorder, 2)
		for i := 0; i < 2; i++ {
			go func() { responses <- request() }()
		}
		response = <-responses
		other := <-responses
		if other.Code != 200 {
			t.Fatalf("concurrent native preparation failed: %d", other.Code)
		}
	} else {
		response = request()
	}
	success := scenario == "initial merge" || strings.HasPrefix(scenario, "initial verify") || scenario == "initial contained"
	if !success {
		expectedCode := 409
		if scenario == "initial missing verifier" {
			expectedCode = 503
		}
		if response.Code != expectedCode || merges != 0 || gets != 0 {
			t.Fatal("invalid initial source reached merge")
		}
		data, _ := os.ReadFile(calls)
		if len(data) != 0 {
			t.Fatal("invalid source reached remote push")
		}
		return
	}
	if response.Code != 200 {
		t.Fatalf("initial native preparation failed: %d %s", response.Code, response.Body.String())
	}
	var execution runner.NativeReleaseExecution
	must(json.Unmarshal(response.Body.Bytes(), &execution))
	if execution.Workspace == nil || execution.Workspace.SourceCommit != intent.SourceCommit || execution.Workspace.TargetCommit != target {
		t.Fatal("initial preparation lost approved lineage")
	}
	expectedMerges := 1
	if scenario == "initial contained" {
		expectedMerges = 0
	}
	expectedRequests := 1
	if scenario == "initial merge" {
		expectedRequests = 2
	}
	if merges != expectedMerges || gets != 2+expectedRequests {
		t.Fatal("remote reconciliation did not precede merge", merges, gets)
	}
	data, _ := os.ReadFile(calls)
	if string(data) != "observe\npush\n" || run("--git-dir", remote, "rev-parse", "refs/heads/task/source") != intent.SourceCommit {
		t.Fatal("initial push was not exact")
	}
	replay := request()
	if replay.Code != 200 || merges != expectedMerges || gets != 3+expectedRequests {
		t.Fatal("prepared recovery repeated merge")
	}
	data, _ = os.ReadFile(calls)
	if string(data) != "observe\npush\n" {
		t.Fatal("prepared recovery repeated push")
	}
	fresh, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if fresh.CommitSHA != intent.SourceCommit || fresh.Status == task.TaskStatusDone {
		t.Fatal("preparation changed original source or accepted task")
	}
	for _, root := range []string{sourceRoot, p.LocalPath, execution.Workspace.Path} {
		if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
			t.Fatal("preparation ran deployment")
		}
	}
	if run("-C", sourceRoot, "rev-parse", "HEAD") != intent.SourceCommit {
		t.Fatal("source checkout changed")
	}
	if access != 3+expectedRequests {
		t.Fatal("prepared recovery did not reobserve saved branch access", access)
	}
	if strings.HasPrefix(scenario, "initial verify") {
		verifyNativeDeliveryStartsIndependentInspection(t, s, item, claim, intent, target, scenario)
	}
}

func verifyNativeDeliveryStartsIndependentInspection(t *testing.T, s *Server, item *task.Task, claim *runner.ClaimTaskResponse, intent task.NativeReleaseIntent, target, scenario string) {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// Native process/observation facts are fixture receipts here; actual process
	// dispatch and registered observation have separate integration coverage.
	started := time.Now().UTC()
	finished := time.Now().UTC()
	receipt := task.DeploymentReceipt{Version: 1, TaskID: item.ID, AttemptID: claim.Attempt.ID, CommitSHA: target, ReleaseSHA: target, Environment: "production", Health: "healthy"}
	raw, _ := json.Marshal(receipt)
	must(s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: task.DeploymentReceiptID(item.ID, claim.Attempt.ID, target), TaskID: item.ID, AttemptID: &claim.Attempt.ID, Author: "cuan-deploy", ActivityType: "deployment_verified", Message: string(raw), CreatedAt: time.Now().UTC()}))
	s.runnerSvc.SetProjectFinder(s.projectSvc)
	completed, err := s.runnerSvc.Complete(ctx, claim.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: claim.Attempt.RunnerID, Status: task.AttemptStatusSucceeded, ExitCode: 0, NativeReleaseProcess: &release.Result{Commit: target, Branch: intent.TargetBranch, StartedAt: started, FinishedAt: finished, ExitCode: 0, SourceUnchanged: true}})
	must(err)
	if completed.Status != task.AttemptStatusSucceeded {
		t.Fatal("fixture native completion rejected", completed.ErrorMessage)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	s.cfg.Environment = "production"
	s.cfg.SourceRepository = intent.Repository
	s.cfg.CommitSHA = target
	invalid := scenario != "initial verify"
	switch scenario {
	case "initial verify changed release":
		s.cfg.CommitSHA = strings.Repeat("f", 40)
	case "initial verify archived":
		p, err := s.projectSvc.Store().GetProject(ctx, item.ProjectID)
		must(err)
		p.IsArchived = true
		must(s.projectSvc.Store().UpdateProject(ctx, p))
	case "initial verify revoked":
		source, err := s.taskSvc.Store().GetAttempt(ctx, intent.SourceAttemptID)
		must(err)
		source.CancelRequested = true
		must(s.taskSvc.Store().UpdateAttempt(ctx, source))
	}
	if s.preparedSourceAlreadyPublished(ctx, detail) == invalid {
		t.Fatal("historical preparation/current publication inspection eligibility mismatch", scenario)
	}
	requests := s.projectStudioRepo.(db.InboxRequestRepository)
	beforeRequests, err := requests.ListInboxRequests(ctx, item.SourceRoomID)
	must(err)
	s.queueInboxTaskReview(ctx, detail)
	proof := task.LiveInspectionIntent{Purpose: task.NativeReleaseInspection, SourceAttemptID: claim.Attempt.ID, InstructionsHash: intent.InstructionsHash}
	saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, proof.RequestID(item.ID)))
	if invalid {
		if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
			t.Fatal(err)
		}
		if saved != nil {
			t.Fatal("invalid current release/source scheduled native verification")
		}
		current, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
		must(err)
		if current.Status == task.TaskStatusDone {
			t.Fatal("invalid source accepted")
		}
		return
	}
	must(err)
	afterRequests, err := requests.ListInboxRequests(ctx, item.SourceRoomID)
	must(err)
	if len(beforeRequests) != len(afterRequests) {
		t.Fatal("native verifier scheduling created a model PM turn")
	}
	if saved == nil {
		t.Fatal("native delivery did not schedule independent inspection")
	}
	parsed, err := task.LiveInspectionForLaunch(&detail.Task, saved)
	must(err)
	if parsed == nil || parsed.Purpose != task.NativeReleaseInspection || parsed.SourceAttemptID != claim.Attempt.ID || parsed.SourceCommit != intent.SourceCommit || parsed.TargetCommit != target || parsed.SourceRequestID != intent.SourceRequestID {
		t.Fatal("scheduled inspection lost original source/approval")
	}
	if saved.State != "queued" && saved.State != "claimed" && saved.State != "started" {
		t.Fatal("native verifier was not durably queued", saved.State, saved.SafeError)
	}
	current, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if current.CommitSHA != intent.SourceCommit || current.Status == task.TaskStatusDone {
		t.Fatal("scheduling rewrote source or accepted task")
	}
	before := len(current.Attempts)
	restarted := *s
	restarted.queueInboxTaskReview(ctx, detail)
	after, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if len(after.Attempts) != before {
		t.Fatal("reconstructed review duplicated independent verifier")
	}
	verifyNativeInspectionCompletionAndAcceptance(t, s, item, claim.Attempt.ID, saved, parsed)
}

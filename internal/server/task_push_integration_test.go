package server

import (
	"context"
	"encoding/json"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskPushConnectionRecoveryRealGit(t *testing.T) {
	s, ownerToken, _, _ := inboxFixture(t)
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	remote := filepath.Join(root, "remote.git")
	actualGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	run := func(args ...string) string {
		t.Helper()
		out, e := exec.Command(actualGit, args...).CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--bare", remote)
	run("init", "--initial-branch=main", source)
	os.WriteFile(filepath.Join(source, "README"), []byte("initial"), 0600)
	run("-C", source, "add", "README")
	run("-C", source, "commit", "-m", "Fixture initial")
	run("-C", source, "update-ref", "refs/remotes/origin/main", "HEAD")
	projects := project.NewMemoryStore()
	p := &project.Project{ID: "repo-task-project", Name: "Repo task", RepoURL: "https://github.com/fixture/repo", LocalPath: source, DefaultBranch: "main", UpdatedAt: time.Now()}
	projects.CreateProject(ctx, p)
	svc := project.NewService(projects, nil, s.cfg.AuthSecret, filepath.Join(root, "workspaces"))
	s.SetProjectService(svc)
	s.taskSvc = task.NewService(task.NewMemoryStore(), svc)
	created, e := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: p.ID, Title: "Approved repository work", Instructions: "Create one result and save it"})
	if e != nil {
		t.Fatal(e)
	}
	attempt, e := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if e != nil {
		t.Fatal(e)
	}
	s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	_, _, capability, cleanup, e := s.issueTaskAgent(ctx, created.ID, attempt.ID, "agent_lifecycle")
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup()
	push := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/task-agent/"+created.ID+"/push", nil)
		r.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	first := push()
	if first.Code != 409 || !strings.Contains(first.Body.String(), "github_connection_required") {
		t.Fatal(first.Code, first.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	response, _ := svc.TaskWorkspace(ctx, p.ID, detail.Instructions)
	workspace, e := svc.TaskExecutionWorkspace(ctx, created.ID, detail.Instructions, response)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(workspace, "result.txt"), []byte("one delivery"), 0600)
	run("-C", workspace, "add", "result.txt")
	run("-C", workspace, "commit", "-m", "Implement once")
	commit := run("-C", workspace, "rev-parse", "HEAD")
	quote := func(value string) string { raw, _ := json.Marshal(value); return string(raw) }
	bin := t.TempDir()
	wrapper := "#!/usr/bin/env python3\nimport os,sys\na=sys.argv[1:]\na=[" + quote(remote) + " if x=='https://github.com/fixture/repo.git' else x for x in a]\nos.execv(" + quote(actualGit) + ",['git']+a)\n"
	os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	owner, _ := s.authSvc.ValidateToken(ctx, ownerToken)
	path, _ := githubConnectionPath(owner.ID)
	if e = cuangit.SaveConnection(path, s.cfg.AuthSecret, "fixture-token", cuangit.Account{Login: "fixture"}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		saved := push()
		if saved.Code != 200 || !strings.Contains(saved.Body.String(), commit) {
			t.Fatal(saved.Code, saved.Body.String())
		}
	}
	final, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	required, resolved := 0, 0
	for _, event := range final.Activities {
		if event.ActivityType == "github_connection_required" {
			required++
		}
		if event.ActivityType == "github_connection_resolved" {
			resolved++
		}
	}
	if required != 1 || resolved != 1 {
		t.Fatal("prerequisite state duplicated", required, resolved)
	}
	if strings.Count(run("-C", workspace, "log", "--format=%s"), "Implement once") != 1 {
		t.Fatal("push recovery duplicated implementation")
	}
	if final.Instructions != detail.Instructions {
		t.Fatal("connection changed original instructions")
	}
}

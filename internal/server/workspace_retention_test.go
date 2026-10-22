package server

import (
	"context"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceRetentionWorkerAndRestore(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, e := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=main")
	os.WriteFile(filepath.Join(source, "result.txt"), []byte("completed result"), 0600)
	run("add", "result.txt")
	run("commit", "-m", "Fixture result")
	head := run("rev-parse", "HEAD")
	run("update-ref", "refs/remotes/origin/main", head)
	remoteAvailable := false
	remoteReads := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteReads++
		if !remoteAvailable {
			w.WriteHeader(503)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("shared access missing")
		}
		w.Write([]byte(`{"object":{"sha":"` + head + `"}}`))
	}))
	defer api.Close()
	client := cuangit.NewClient()
	client.SetAPIBaseURL(api.URL)
	projects := project.NewMemoryStore()
	p := &project.Project{ID: "project_fixture", Name: "Fixture", RepoURL: "https://github.com/fixture/repo", LocalPath: source, DefaultBranch: "main", UpdatedAt: time.Now()}
	if e := projects.CreateProject(ctx, p); e != nil {
		t.Fatal(e)
	}
	projectSvc := project.NewService(projects, client, "fixture-secret", t.TempDir())
	projectSvc.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "fixture", nil })
	tasks := task.NewMemoryStore()
	item := &task.Task{ID: "completed-task", ProjectID: p.ID, Title: "Completed", Status: "done", UpdatedAt: time.Now()}
	if e := tasks.CreateTask(ctx, item); e != nil {
		t.Fatal(e)
	}
	manager := terminal.NewManager(terminal.ManagerConfig{WorkspacesDir: t.TempDir()})
	defer manager.CloseAll()
	s := &Server{projectSvc: projectSvc, taskSvc: task.NewService(tasks, projectSvc), terminalManager: manager}
	response, _ := projectSvc.Get(ctx, p.ID)
	workspace, e := projectSvc.ExecutionWorkspace(ctx, item.ID, response)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().Add(8 * 24 * time.Hour)
	lease, e := manager.AcquireTaskExecutionLease(item.ID)
	if e != nil {
		t.Fatal(e)
	}
	s.runWorkspaceRetention(ctx, 0, now)
	if remoteReads != 0 {
		t.Fatal("cleanup queried remote while execution owned")
	}
	lease.Close()
	s.runWorkspaceRetention(ctx, 0, now)
	if _, e = os.Stat(workspace); e != nil {
		t.Fatal("remote outage deleted work")
	}
	remoteAvailable = true
	s.runWorkspaceRetention(ctx, 0, now)
	if _, e = os.Stat(workspace); !os.IsNotExist(e) {
		t.Fatal("eligible completed checkout retained")
	}
	restored, e := projectSvc.ExecutionWorkspace(ctx, item.ID, response)
	if e != nil || restored != workspace {
		t.Fatal("reopen did not restore", e)
	}
	contents, _ := os.ReadFile(filepath.Join(restored, "result.txt"))
	if string(contents) != "completed result" {
		t.Fatal("result lost")
	}
	saved, e := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if e != nil || saved.Title != "Completed" || saved.Status != "done" {
		t.Fatal("cleanup changed task record", e)
	}
}

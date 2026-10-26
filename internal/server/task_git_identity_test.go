package server

import (
	"context"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreshTaskCommitUsesOwnerWithoutGitConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	owner, e := s.authSvc.OwnerIdentity(ctx)
	if e != nil {
		t.Fatal(e)
	}
	env, e := s.taskCommitEnvironment(ctx, owner.ID)
	if e != nil {
		t.Fatal(e)
	}
	repo := t.TempDir()
	cleanHome := t.TempDir()
	env = terminal.SanitizeEnvironment(append(env, "HOME="+cleanHome, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"), "task-identity", repo)
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = env
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=main")
	os.WriteFile(filepath.Join(repo, "result"), []byte("user result"), 0600)
	run("add", "result")
	run("commit", "-m", "Commit without user setup")
	if author := run("log", "-1", "--format=%ae"); author != owner.Email {
		t.Fatal("commit used another author's email")
	}
	if _, e = os.Stat(filepath.Join(cleanHome, ".gitconfig")); !os.IsNotExist(e) {
		t.Fatal("task modified shared Git configuration")
	}
}
func TestTaskCommitIdentityGitHubAndOverride(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	owner, _ := s.authSvc.OwnerID(ctx)
	path, _ := githubConnectionPath(owner)
	if e := git.SaveConnection(path, s.cfg.AuthSecret, "fixture-token", git.Account{ID: 123, Login: "human-owner", Name: "Human Owner"}); e != nil {
		t.Fatal(e)
	}
	env, e := s.taskCommitEnvironment(ctx, owner)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(strings.Join(env, "\n"), "GIT_AUTHOR_EMAIL=123+human-owner@users.noreply.github.com") {
		t.Fatal("GitHub no-reply identity not used")
	}
	s.cfg.GitAuthorName = "Explicit Human"
	s.cfg.GitAuthorEmail = "explicit@example.invalid"
	env, e = s.taskCommitEnvironment(ctx, owner)
	if e != nil || !strings.Contains(strings.Join(env, "\n"), "GIT_AUTHOR_NAME=Explicit Human") {
		t.Fatal("explicit identity ignored", e)
	}
	if _, e = s.taskCommitEnvironment(ctx, "other-owner"); e == nil {
		t.Fatal("other owner identity accepted")
	}
	s.cfg.GitAuthorName = "invalid\nname"
	if _, e = s.taskCommitEnvironment(ctx, owner); e == nil {
		t.Fatal("invalid identity accepted")
	}
}

func TestFreshTaskProcessCommitsWithOwnerIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow task git identity process test in short mode")
	}
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	owner, _ := s.authSvc.OwnerIdentity(ctx)
	created, e := s.taskSvc.CreateTask(ctx, owner.ID, task.CreateTaskRequest{Title: "Create and commit one result", Instructions: "Create result.txt and commit it in the task workspace"})
	if e != nil {
		t.Fatal(e)
	}
	script := `#!/bin/sh
set -eu
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
git init -q
printf 'verified result\n' > result.txt
git add -- result.txt
git commit -m 'Create verified task result'
"$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Created and committed result.txt' --complete-note 'Verified real commit with human author in fresh HOME'
while :;do sleep 0.1;done
`
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	response := call("POST", "/api/v1/tasks/"+created.ID+"/run-agent", `{"request_id":"40000000-0000-4000-8000-000000000001","agent_id":"agent_lifecycle"}`)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	final := waitWorkspaceTask(t, s, created.ID, "succeeded")
	if final.Status != "done" {
		t.Fatal("real task did not complete")
	}
	workspace := ""
	if created.ProjectID != "" {
		projectResponse, err := s.projectSvc.TaskWorkspace(ctx, created.ProjectID, created.Instructions)
		if err != nil {
			t.Fatal(err)
		}
		workspace = projectResponse.LocalPath
	}
	if workspace == "" {
		workspace, e = s.terminalManager.ResolveTaskWorkspace(created.ID)
		if e != nil {
			t.Fatal(e)
		}
	}
	author, e := exec.Command("git", "-C", workspace, "log", "-1", "--format=%ae").Output()
	if e != nil || strings.TrimSpace(string(author)) != owner.Email {
		t.Fatal("real task used wrong human author", e)
	}
}

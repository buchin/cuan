package server

import (
	"context"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/terminal"
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

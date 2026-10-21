package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExecutionWorkspacesIsolateAndResume(t *testing.T) {
	ctx := context.Background()
	source := filepath.Join(t.TempDir(), "source")
	os.MkdirAll(source, 0700)
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
	}
	run("init", "--initial-branch=trunk", source)
	os.WriteFile(filepath.Join(source, "README"), []byte("initial"), 0600)
	run("-C", source, "add", "README")
	run("-C", source, "commit", "-m", "Create fixture")
	run("-C", source, "update-ref", "refs/remotes/origin/trunk", "HEAD")
	s := NewService(NewMemoryStore(), nil, "fixture-secret", t.TempDir())
	p := &ProjectResponse{RepoURL: "https://github.com/fixture/repo", LocalPath: source, DefaultBranch: "trunk"}
	legacy, err := s.TaskExecutionWorkspace(ctx, "legacy-task", "Old instructions", p)
	if err != nil || legacy != source {
		t.Fatal("upgrade moved existing task", err)
	}
	first, err := s.TaskExecutionWorkspace(ctx, "task-one", SnapshotProjectGuidance("Approved work", p), p)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ExecutionWorkspace(ctx, "task-two", p)
	if err != nil || first == second {
		t.Fatal("tasks share workspace", err)
	}
	os.WriteFile(filepath.Join(first, "README"), []byte("task one pending edits"), 0600)
	data, _ := os.ReadFile(filepath.Join(second, "README"))
	if string(data) != "initial" {
		t.Fatal("cross-task edits leaked")
	}
	restarted := NewService(NewMemoryStore(), nil, "fixture-secret", s.baseDir)
	resumed, err := restarted.ExecutionWorkspace(ctx, "task-one", p)
	if err != nil || resumed != first {
		t.Fatal("resume duplicated workspace", err)
	}
	data, _ = os.ReadFile(filepath.Join(resumed, "README"))
	if string(data) != "task one pending edits" {
		t.Fatal("resume discarded work")
	}
	run("-C", second, "checkout", "--detach")
	if _, err = s.ExecutionWorkspace(ctx, "task-two", p); err == nil {
		t.Fatal("changed branch reused")
	}
	run("-C", source, "worktree", "remove", "--force", first)
	if _, err = s.ExecutionWorkspace(ctx, "task-one", p); err == nil {
		t.Fatal("missing workspace silently recreated")
	}
}

func TestRelativeExecutionWorkspaceUsesActualGitDirectory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Chdir(root)
	source := filepath.Join("data", "projects", "source")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
	}
	run("init", "--initial-branch=main", source)
	if err := os.WriteFile(filepath.Join(source, "README"), []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	run("-C", source, "add", "README")
	run("-C", source, "commit", "-m", "Initial")
	run("-C", source, "update-ref", "refs/remotes/origin/main", "HEAD")
	svc := NewService(NewMemoryStore(), nil, "fixture-secret", filepath.Join("data", "projects"))
	p := &ProjectResponse{RepoURL: "https://github.com/fixture/repo", LocalPath: source, DefaultBranch: "main"}
	work, err := svc.ExecutionWorkspace(ctx, "relative-task", p)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(work) {
		t.Fatal("execution directory must be absolute", work)
	}
	if _, err := os.Stat(filepath.Join(work, "README")); err != nil {
		t.Fatal("returned workspace is not actual Git directory", err)
	}
	if err := os.WriteFile(filepath.Join(work, "pending"), []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err := svc.ExecutionWorkspace(ctx, "relative-task", p)
	if err != nil || again != work {
		t.Fatal("resume moved workspace", again, err)
	}
	// Reproduce an existing worktree created by the old relative -C behavior.
	digest := sha256.Sum256([]byte("legacy-relative-task" + "\x00" + source))
	key := hex.EncodeToString(digest[:])
	oldDestination := filepath.Join("data", "projects", "task-workspaces", key)
	run("-C", source, "worktree", "add", "-b", "cuan/task-"+key, oldDestination, "refs/remotes/origin/main")
	legacy, _ := filepath.Abs(filepath.Join(source, oldDestination))
	if err := os.WriteFile(filepath.Join(legacy, "pending"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	resumed, err := svc.ExecutionWorkspace(ctx, "legacy-relative-task", p)
	if err != nil || resumed != legacy {
		t.Fatal("legacy worktree abandoned", resumed, err)
	}
	data, err := os.ReadFile(filepath.Join(resumed, "pending"))
	if err != nil || string(data) != "original" {
		t.Fatal("legacy pending work lost", err)
	}
}

package project

import (
	"context"
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

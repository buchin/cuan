package project

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRetiredWorkspaceRestoresWithoutReplayingWork(t *testing.T) {
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
	os.WriteFile(filepath.Join(source, "result"), []byte("delivered"), 0600)
	run("add", "result")
	run("commit", "-m", "Fixture completed work")
	head := run("rev-parse", "HEAD")
	run("update-ref", "refs/remotes/origin/main", head)
	s := NewService(NewMemoryStore(), nil, "fixture-secret", t.TempDir())
	p := &ProjectResponse{RepoURL: "https://github.com/fixture/app", LocalPath: source, DefaultBranch: "main"}
	workspace, e := s.ExecutionWorkspace(ctx, "retire-task", p)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	observe := func() (WorkspaceRetirementState, error) {
		return WorkspaceRetirementState{Status: "done", UpdatedAt: now.Add(-8 * 24 * time.Hour), RemoteHead: head}, nil
	}
	acquire := func() (io.Closer, error) { return os.Open(os.DevNull) }
	if _, e = s.RetireExecutionWorkspace(ctx, "retire-task", p, now, func() (io.Closer, error) { return nil, errors.New("busy") }, observe); e == nil {
		t.Fatal("removed without execution lease")
	}
	if _, e = os.Stat(workspace); e != nil {
		t.Fatal("busy workspace changed")
	}
	d, e := s.RetireExecutionWorkspace(ctx, "retire-task", p, now, acquire, observe)
	if e != nil || !d.Eligible {
		t.Fatal(d, e)
	}
	if _, e = os.Stat(workspace); !os.IsNotExist(e) {
		t.Fatal("worktree not removed")
	}
	restarted := NewService(NewMemoryStore(), nil, "fixture-secret", s.baseDir)
	restored, e := restarted.ExecutionWorkspace(ctx, "retire-task", p)
	if e != nil || restored != workspace {
		t.Fatal("retired task did not restore", e)
	}
	data, e := os.ReadFile(filepath.Join(restored, "result"))
	if e != nil || string(data) != "delivered" {
		t.Fatal("delivered result lost")
	}
	os.WriteFile(filepath.Join(restored, "pending"), []byte("new revision"), 0600)
	d, e = s.RetireExecutionWorkspace(ctx, "retire-task", p, now, acquire, observe)
	if e != nil || d.Eligible {
		t.Fatal("new revision removed", e)
	}
	// A previously used retirement receipt cannot authorize a later accidental deletion.
	run("worktree", "remove", "--force", workspace)
	if _, e = restarted.ExecutionWorkspace(ctx, "retire-task", p); e == nil {
		t.Fatal("old retirement receipt reused after later deletion")
	}
}

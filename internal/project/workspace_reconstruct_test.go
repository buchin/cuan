package project

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingWorkspaceReconstructionPreservesEvidence(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	run := func(args ...string) string {
		t.Helper()
		out, e := exec.Command("git", args...).CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=main", source)
	os.WriteFile(filepath.Join(source, "README"), []byte("original\n"), 0600)
	run("-C", source, "add", "README")
	run("-C", source, "commit", "-m", "Fixture")
	svc := NewService(NewMemoryStore(), nil, "secret", filepath.Join(root, "state"))
	p := &ProjectResponse{RepoURL: "https://github.com/fixture/repo", LocalPath: source, DefaultBranch: "main"}
	original, e := svc.ExecutionWorkspace(ctx, "same-task", p)
	if e != nil {
		t.Fatal(e)
	}
	head := run("-C", original, "rev-parse", "HEAD")
	branch := run("-C", original, "symbolic-ref", "--short", "HEAD")
	os.WriteFile(filepath.Join(original, "README"), []byte("staged change\n"), 0600)
	run("-C", original, "add", "README")
	os.WriteFile(filepath.Join(original, "untracked"), []byte("not recoverable"), 0600)
	meta := run("-C", original, "rev-parse", "--absolute-git-dir")
	index, _ := os.ReadFile(filepath.Join(meta, "index"))
	os.RemoveAll(original) // External loss, not approved retirement.
	if _, e = svc.ExecutionWorkspace(ctx, "same-task", p); e != ErrExecutionWorkspaceMissing {
		t.Fatal("viewing silently reconstructed missing files", e)
	}
	work, e := svc.ReconstructTaskExecutionWorkspace(ctx, "same-task", p)
	if e != nil {
		t.Fatal(e)
	}
	if work == original {
		t.Fatal("overwrote missing original registration")
	}
	if got := run("-C", source, "rev-parse", "refs/heads/"+branch); got != head {
		t.Fatal("retained branch changed")
	}
	saved, _ := os.ReadFile(filepath.Join(meta, "index"))
	if string(saved) != string(index) {
		t.Fatal("retained index changed")
	}
	evidence, _ := os.ReadFile(filepath.Join(filepath.Dir(work), "original-index"))
	if string(evidence) != string(index) {
		t.Fatal("index evidence missing")
	}
	patch, _ := os.ReadFile(filepath.Join(filepath.Dir(work), "staged.patch"))
	if !strings.Contains(string(patch), "+staged change") {
		t.Fatal("staged delta missing")
	}
	if b, _ := os.ReadFile(filepath.Join(work, "README")); string(b) != "original\n" {
		t.Fatal("claimed uncommitted state reconstructed")
	}
	if _, e = os.Stat(filepath.Join(work, "untracked")); !os.IsNotExist(e) {
		t.Fatal("invented untracked recovery")
	}
	note := WorkspaceReconstructionContext(work)
	if !strings.Contains(note, "NOT proven recovered") || !strings.Contains(note, "Recheck every original acceptance criterion") {
		t.Fatal("missing recovery disclosure")
	}
	os.WriteFile(filepath.Join(work, "new-work"), []byte("retain during restart"), 0600)
	restarted := NewService(NewMemoryStore(), nil, "secret", svc.baseDir)
	again, e := restarted.ExecutionWorkspace(ctx, "same-task", p)
	if e != nil || again != work {
		t.Fatal("restart replaced reconstruction", e)
	}
	b, _ := os.ReadFile(filepath.Join(again, "new-work"))
	if string(b) != "retain during restart" {
		t.Fatal("resume lost pending work")
	}
	// A second deletion must not silently reconstruct a third working copy.
	os.RemoveAll(work)
	if _, e = restarted.ReconstructTaskExecutionWorkspace(ctx, "same-task", p); e == nil {
		t.Fatal("repeated deletion silently reset work")
	}
}

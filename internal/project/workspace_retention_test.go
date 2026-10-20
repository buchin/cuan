package project

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceRetentionProtectsLocalFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(string(out), err)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=main")
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("private-output/\n"), 0600)
	run("add", ".gitignore")
	run("commit", "-m", "Fixture")
	head := run("rev-parse", "HEAD")
	now := time.Now()
	old := now.Add(-8 * 24 * time.Hour)
	inspect := func(status string, updated time.Time, active bool, remote string) WorkspaceRetentionDecision {
		t.Helper()
		d, e := InspectWorkspaceRetention(ctx, dir, status, updated, now, active, remote)
		if e != nil {
			t.Fatal(e)
		}
		return d
	}
	if !inspect("done", old, false, head).Eligible {
		t.Fatal("clean pushed completion not eligible")
	}
	for _, status := range []string{"doing", "blocked", "review"} {
		if inspect(status, old, false, head).Eligible {
			t.Fatal("unfinished task eligible")
		}
	}
	if inspect("done", old, true, head).Eligible || inspect("done", now, false, head).Eligible || inspect("done", old, false, "").Eligible {
		t.Fatal("missing retention/ownership/remote gate")
	}
	os.Mkdir(filepath.Join(dir, "private-output"), 0700)
	os.WriteFile(filepath.Join(dir, "private-output", "only-copy"), []byte("keep"), 0600)
	if inspect("done", old, false, head).Eligible {
		t.Fatal("ignored user output treated as disposable")
	}
	os.RemoveAll(filepath.Join(dir, "private-output"))
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("untracked"), 0600)
	if inspect("done", old, false, head).Eligible {
		t.Fatal("untracked edits eligible")
	}
	os.Remove(filepath.Join(dir, "new.txt"))
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("changed"), 0600)
	if inspect("done", old, false, head).Eligible {
		t.Fatal("tracked edits eligible")
	}
}

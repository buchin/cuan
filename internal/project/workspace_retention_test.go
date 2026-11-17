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

	resolved, err := InspectResolvedWorkspaceRetention(ctx, dir, "done", now, now, false, head)
	if err != nil || !resolved.Eligible {
		t.Fatal("explicit resolution did not release recent clean checkout", resolved, err)
	}
	for _, scenario := range []struct {
		status, remote string
		active         bool
	}{{"doing", head, false}, {"done", head, true}} {
		decision, err := InspectResolvedWorkspaceRetention(ctx, dir, scenario.status, now, now, scenario.active, scenario.remote)
		if err != nil || decision.Eligible {
			t.Fatal("resolution bypassed source/ownership checks", decision, err)
		}
	}
	os.Mkdir(filepath.Join(dir, "private-output"), 0700)
	os.WriteFile(filepath.Join(dir, "private-output", "only-copy"), []byte("keep"), 0600)
	if inspect("done", old, false, head).Eligible {
		t.Fatal("ignored user output treated as disposable")
	}
	if decision, err := InspectResolvedWorkspaceRetention(ctx, dir, "done", now, now, false, head); err != nil || !decision.Eligible {
		t.Fatal("explicit resolution did not authorize disposing workspace output", decision, err)
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
	if inspect("cancelled", now, false, head).Eligible {
		t.Fatal("dirty cancelled task eligible")
	}
	// Revert gitignore so working tree is clean again
	run("checkout", ".gitignore")
	if !inspect("cancelled", now, false, head).Eligible {
		t.Fatal("clean cancelled task not eligible")
	}
	if inspect("cancelled", now, true, head).Eligible {
		t.Fatal("active cancelled task eligible")
	}
}

func TestWorkspaceRetentionPeriodConfigurable(t *testing.T) {
	os.Unsetenv("CUAN_WORKSPACE_RETENTION_HOURS")
	os.Unsetenv("CUAN_WORKSPACE_TTL")
	if p := WorkspaceRetentionPeriod(); p != 7*24*time.Hour {
		t.Fatalf("expected default 7 days, got %v", p)
	}
	t.Setenv("CUAN_WORKSPACE_RETENTION_HOURS", "24")
	if p := WorkspaceRetentionPeriod(); p != 24*time.Hour {
		t.Fatalf("expected 24 hours, got %v", p)
	}
	t.Setenv("CUAN_WORKSPACE_RETENTION_HOURS", "0")
	if p := WorkspaceRetentionPeriod(); p != 0 {
		t.Fatalf("expected 0 duration for immediate retention, got %v", p)
	}
	t.Setenv("CUAN_WORKSPACE_TTL", "12h")
	if p := WorkspaceRetentionPeriod(); p != 12*time.Hour {
		t.Fatalf("expected 12 hours from TTL, got %v", p)
	}
	t.Setenv("CUAN_WORKSPACE_TTL", "48")
	if p := WorkspaceRetentionPeriod(); p != 48*time.Hour {
		t.Fatalf("expected 48 hours from integer TTL, got %v", p)
	}
}

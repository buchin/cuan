package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitIntegrityInspection(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v, output: %s", strings.Join(args, " "), err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	git("init", "-q")
	git("config", "user.name", "Test User")
	git("config", "user.email", "test@test.local")
	git("checkout", "-b", "main")

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0600)
	git("add", "README.md")
	git("commit", "-m", "initial commit")

	headSHA := git("rev-parse", "HEAD")

	// 1. In sync / release matches HEAD
	report := inspectGitIntegrity(ctx, dir, headSHA)
	if report.Branch != "main" {
		t.Fatalf("expected main branch, got: %s", report.Branch)
	}
	if report.HeadSHA != headSHA {
		t.Fatalf("expected head SHA %s, got: %s", headSHA, report.HeadSHA)
	}
	if !report.ReleaseMatch {
		t.Fatalf("expected release match to be true")
	}

	summary := report.Summary()
	if !strings.Contains(summary, "Git & Deployment Integrity:") {
		t.Fatalf("expected header in summary: %s", summary)
	}
	if !strings.Contains(summary, "matches HEAD") {
		t.Fatalf("expected matches HEAD in summary: %s", summary)
	}

	// 2. Release mismatch
	mismatchReport := inspectGitIntegrity(ctx, dir, "old_release_sha_12345")
	if mismatchReport.ReleaseMatch {
		t.Fatalf("expected release match to be false")
	}
	mismatchSummary := mismatchReport.Summary()
	if !strings.Contains(mismatchSummary, "MISMATCH with HEAD") {
		t.Fatalf("expected mismatch indicator in summary: %s", mismatchSummary)
	}
	if !strings.Contains(mismatchSummary, "DEPLOYMENT INTEGRITY MISMATCH") {
		t.Fatalf("expected warning banner: %s", mismatchSummary)
	}

	// 3. Unmerged task branch
	git("checkout", "-b", "task/feature-x")
	os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("work in progress\n"), 0600)
	git("add", "feature.txt")
	git("commit", "-m", "wip feature")
	git("checkout", "main")

	unmergedReport := inspectGitIntegrity(ctx, dir, headSHA)
	if len(unmergedReport.UnmergedBranches) == 0 {
		t.Fatalf("expected unmerged branches, got none")
	}
	if unmergedReport.UnmergedBranches[0] != "task/feature-x" {
		t.Fatalf("expected task/feature-x unmerged, got: %v", unmergedReport.UnmergedBranches)
	}
	unmergedSummary := unmergedReport.Summary()
	if !strings.Contains(unmergedSummary, "Unmerged Task Branches (not in main): task/feature-x") {
		t.Fatalf("expected unmerged branch list in summary: %s", unmergedSummary)
	}
	if !strings.Contains(unmergedSummary, "UNMERGED WORK DETECTED") {
		t.Fatalf("expected unmerged work alert in summary: %s", unmergedSummary)
	}
}

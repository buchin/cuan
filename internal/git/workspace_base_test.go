package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceUsesSelectedBaseInsteadOfCacheHEAD(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	createTestBaseRepo(t, base)
	retentionGit(t, base, "checkout", "-b", "dev")
	os.WriteFile(filepath.Join(base, "selected.txt"), []byte("approved dev"), 0644)
	retentionGit(t, base, "add", ".")
	retentionGit(t, base, "commit", "-m", "Dev baseline")
	dev := retentionGit(t, base, "rev-parse", "HEAD")
	retentionGit(t, base, "checkout", "main")
	c := NewClient()
	cfg := WorkspaceConfig{TaskID: "task", BaseRepoDir: base, WorkspacesBaseDir: filepath.Join(root, "workspaces"), DefaultBranch: "dev", BranchName: "task/selected"}
	ws, err := c.PrepareTaskWorkspace(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if retentionGit(t, ws.Path, "rev-parse", "HEAD") != dev {
		t.Fatal("task started from unrelated cache HEAD")
	}
	if retentionGit(t, base, "symbolic-ref", "--short", "HEAD") != "main\n" {
		t.Fatal("source checkout changed")
	}
	if ws.BaseCommit != strings.TrimSpace(dev) {
		t.Fatal("selected commit was not pinned", ws.BaseCommit)
	}
	if ws.BaseRef != "refs/remotes/cuan-base/dev" {
		t.Fatal("missing explicit baseline", ws.BaseRef)
	}
}

func TestWorkspaceRejectsMissingSelectedBase(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	createTestBaseRepo(t, base)
	_, err := NewClient().PrepareTaskWorkspace(context.Background(), WorkspaceConfig{
		TaskID: "missing", BaseRepoDir: base, WorkspacesBaseDir: filepath.Join(root, "workspaces"), DefaultBranch: "missing",
	})
	if err == nil || !strings.Contains(err.Error(), "selected project base branch is unavailable") {
		t.Fatalf("unrelated HEAD accepted as selected branch: %v", err)
	}
}

func TestBoundBaseEmptyDiffDoesNotReportPreviousCommit(t *testing.T) {
	root := t.TempDir()
	createTestBaseRepo(t, root)
	os.WriteFile(filepath.Join(root, "upstream.go"), []byte("package upstream"), 0644)
	retentionGit(t, root, "add", ".")
	retentionGit(t, root, "commit", "-m", "Upstream change")
	retentionGit(t, root, "update-ref", "refs/remotes/cuan-base/main", "HEAD")
	commit := strings.TrimSpace(retentionGit(t, root, "rev-parse", "HEAD"))
	for _, base := range []string{"refs/remotes/cuan-base/main", commit} {
		summary, err := NewClient().GetDiffSummary(context.Background(), root, base)
		if err != nil || summary != "" {
			t.Fatalf("upstream commit misreported as task work for %s: %q %v", base, summary, err)
		}
	}
}

func TestBoundDiffCannotHideFilesUsingReplaceRefs(t *testing.T) {
	root, _, base, head, run := snapshotFixture(t)
	run(root, "replace", head, base)
	before := run(root, "status", "--porcelain=v1")
	summary, err := NewClient().GetDiffSummary(context.Background(), root, base)
	if err != nil || !strings.Contains(summary, "feature.txt") {
		t.Fatal("replacement history hid committed changed files", summary, err)
	}
	if run(root, "status", "--porcelain=v1") != before {
		t.Fatal("diff inspection changed working tree")
	}
}

func TestWorkspaceImportsFreshCacheRefWithoutOverwritingRetainedTask(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	createTestBaseRepo(t, base)
	c := NewClient()
	cfg := WorkspaceConfig{TaskID: "task", BaseRepoDir: base, WorkspacesBaseDir: filepath.Join(root, "workspaces"), DefaultBranch: "main", BranchName: "task/retained"}
	ws, err := c.PrepareTaskWorkspace(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	original := retentionGit(t, ws.Path, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(ws.Path, "README.md"), []byte("pending approved change"), 0644)
	retentionGit(t, base, "checkout", "-b", "new-cache")
	os.WriteFile(filepath.Join(base, "verifier.txt"), []byte("current verification contract"), 0644)
	retentionGit(t, base, "add", ".")
	retentionGit(t, base, "commit", "-m", "Current baseline")
	latest := retentionGit(t, base, "rev-parse", "HEAD")
	retentionGit(t, base, "update-ref", "refs/remotes/origin/main", strings.TrimSpace(latest))
	retentionGit(t, base, "checkout", "main") // Working files still represent the older cache checkout.
	again, err := c.PrepareTaskWorkspace(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if again.BaseCommit != strings.TrimSpace(latest) || ws.BaseCommit != strings.TrimSpace(original) {
		t.Fatal("preparing a new attempt changed the previous attempt baseline", ws.BaseCommit, again.BaseCommit)
	}
	if retentionGit(t, again.Path, "rev-parse", again.BaseRef) != latest {
		t.Fatal("fresh remote-tracking ref was not imported")
	}
	if retentionGit(t, again.Path, "rev-parse", "HEAD") != original {
		t.Fatal("retained task branch moved")
	}
	if content, _ := os.ReadFile(filepath.Join(again.Path, "README.md")); string(content) != "pending approved change" {
		t.Fatal("pending task edits replaced")
	}
	if got := retentionGit(t, again.Path, "show", again.BaseRef+":verifier.txt"); got != "current verification contract" {
		t.Fatal("fresh verifier unavailable", got)
	}
}

func TestPinnedMissingCommitDoesNotFallbackToOtherHistory(t *testing.T) {
	root := t.TempDir()
	createTestBaseRepo(t, root)
	_, err := NewClient().GetDiffSummary(context.Background(), root, strings.Repeat("0", 40))
	if err == nil {
		t.Fatal("unavailable pinned commit silently used another baseline")
	}
}
func TestCommitHashRequiresFullObjectName(t *testing.T) {
	for _, value := range []string{strings.Repeat("a", 40), strings.Repeat("f", 64)} {
		if !IsCommitHash(value) {
			t.Fatal("full object name rejected")
		}
	}
	for _, value := range []string{"main", "refs/remotes/cuan-base/dev", "abcdef0", strings.Repeat("x", 40), ""} {
		if IsCommitHash(value) {
			t.Fatal("non-object ref treated as pinned commit", value)
		}
	}
}

package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func createTestBaseRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir base repo: %v", err)
	}

	cmdInit := exec.Command("git", "-C", dir, "init", "-b", "main")
	if out, err := cmdInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", string(out))
	}

	_ = exec.Command("git", "-C", dir, "config", "user.name", "Buchin").Run()
	_ = exec.Command("git", "-C", dir, "config", "user.email", "buchin@dropsugar.com").Run()

	file1 := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file1, []byte("# Cuan Stable Baseline\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_ = exec.Command("git", "-C", dir, "add", ".").Run()
	cmdCommit := exec.Command("git", "-C", dir, "commit", "-m", "initial stable commit")
	if out, err := cmdCommit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %s", string(out))
	}
}

func TestWorkspaceIsolation(t *testing.T) {
	tempDir := t.TempDir()
	baseRepoDir := filepath.Join(tempDir, "base_repo")
	workspacesDir := filepath.Join(tempDir, "workspaces")

	createTestBaseRepo(t, baseRepoDir)

	client := NewClient()
	ctx := context.Background()

	// 1. Prepare workspace for Task 1
	ws1, err := client.PrepareTaskWorkspace(ctx, WorkspaceConfig{
		TaskID:            "task_001",
		ProjectID:         "proj_cuan",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: workspacesDir,
		BranchName:        "task/task_001",
		DefaultBranch:     "main",
		AuthorName:        "Buchin",
		AuthorEmail:       "buchin@dropsugar.com",
	})
	if err != nil {
		t.Fatalf("prepare workspace 1: %v", err)
	}

	// 2. Prepare workspace for Task 2
	ws2, err := client.PrepareTaskWorkspace(ctx, WorkspaceConfig{
		TaskID:            "task_002",
		ProjectID:         "proj_cuan",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: workspacesDir,
		BranchName:        "task/task_002",
		DefaultBranch:     "main",
		AuthorName:        "Buchin",
		AuthorEmail:       "buchin@dropsugar.com",
	})
	if err != nil {
		t.Fatalf("prepare workspace 2: %v", err)
	}

	// Modify file in Workspace 1
	ws1File := filepath.Join(ws1.Path, "feature_1.txt")
	if err := os.WriteFile(ws1File, []byte("Work from task 1\n"), 0644); err != nil {
		t.Fatalf("write ws1 file: %v", err)
	}

	// Modify README in Workspace 1
	ws1Readme := filepath.Join(ws1.Path, "README.md")
	if err := os.WriteFile(ws1Readme, []byte("# Cuan Modified by Task 1\n"), 0644); err != nil {
		t.Fatalf("write ws1 readme: %v", err)
	}

	// VERIFY ISOLATION:
	// A. Base repo must be UNTOUCHED
	baseReadme, err := os.ReadFile(filepath.Join(baseRepoDir, "README.md"))
	if err != nil {
		t.Fatalf("read base readme: %v", err)
	}
	if string(baseReadme) != "# Cuan Stable Baseline\n" {
		t.Fatalf("ISOLATION BREACH: base repository was modified! Content: %s", string(baseReadme))
	}
	if _, err := os.Stat(filepath.Join(baseRepoDir, "feature_1.txt")); !os.IsNotExist(err) {
		t.Fatalf("ISOLATION BREACH: feature_1.txt leaked into base repository!")
	}

	// B. Workspace 2 must be UNTOUCHED
	ws2Readme, err := os.ReadFile(filepath.Join(ws2.Path, "README.md"))
	if err != nil {
		t.Fatalf("read ws2 readme: %v", err)
	}
	if string(ws2Readme) != "# Cuan Stable Baseline\n" {
		t.Fatalf("ISOLATION BREACH: workspace 2 was modified! Content: %s", string(ws2Readme))
	}
	if _, err := os.Stat(filepath.Join(ws2.Path, "feature_1.txt")); !os.IsNotExist(err) {
		t.Fatalf("ISOLATION BREACH: feature_1.txt leaked into workspace 2!")
	}
}

func TestCommitChanges_AuthorIdentity(t *testing.T) {
	tempDir := t.TempDir()
	baseRepoDir := filepath.Join(tempDir, "base_repo")
	workspacesDir := filepath.Join(tempDir, "workspaces")

	createTestBaseRepo(t, baseRepoDir)

	client := NewClient()
	ctx := context.Background()

	authorName := "Buchin"
	authorEmail := "buchin@dropsugar.com"

	ws, err := client.PrepareTaskWorkspace(ctx, WorkspaceConfig{
		TaskID:            "task_auth_test",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: workspacesDir,
		AuthorName:        authorName,
		AuthorEmail:       authorEmail,
	})
	if err != nil {
		t.Fatalf("prepare workspace: %v", err)
	}

	// Make changes
	newFile := filepath.Join(ws.Path, "auth_feature.go")
	if err := os.WriteFile(newFile, []byte("package auth\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	commitSHA, err := client.CommitChanges(ctx, ws.Path, "feat: add auth feature", authorName, authorEmail)
	if err != nil {
		t.Fatalf("commit changes: %v", err)
	}

	if len(commitSHA) < 7 {
		t.Fatalf("invalid commit SHA: %s", commitSHA)
	}

	// Verify author and committer identity on commit
	cmdLog := exec.Command("git", "-C", ws.Path, "log", "-n", "1", "--pretty=format:%an|%ae|%cn|%ce")
	out, err := cmdLog.Output()
	if err != nil {
		t.Fatalf("git log format: %v", err)
	}

	parts := strings.Split(string(out), "|")
	if len(parts) != 4 {
		t.Fatalf("unexpected git log format: %s", string(out))
	}

	gotAuthorName, gotAuthorEmail := parts[0], parts[1]
	gotCommitterName, gotCommitterEmail := parts[2], parts[3]

	if gotAuthorName != authorName || gotAuthorEmail != authorEmail {
		t.Fatalf("Author identity mismatch! Want %s <%s>, got %s <%s>", authorName, authorEmail, gotAuthorName, gotAuthorEmail)
	}

	if gotCommitterName != authorName || gotCommitterEmail != authorEmail {
		t.Fatalf("Committer identity mismatch! Want %s <%s>, got %s <%s>", authorName, authorEmail, gotCommitterName, gotCommitterEmail)
	}
}

func TestBranchProtection_DirectPushRejected(t *testing.T) {
	tempDir := t.TempDir()
	baseRepoDir := filepath.Join(tempDir, "base_repo")
	workspacesDir := filepath.Join(tempDir, "workspaces")

	createTestBaseRepo(t, baseRepoDir)

	client := NewClient()
	ctx := context.Background()

	ws, err := client.PrepareTaskWorkspace(ctx, WorkspaceConfig{
		TaskID:            "task_protect_test",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: workspacesDir,
		DefaultBranch:     "main",
	})
	if err != nil {
		t.Fatalf("prepare workspace: %v", err)
	}

	// 1. Direct Go API check: PushTaskBranch targeting "main" must be rejected
	err = client.PushTaskBranch(ctx, ws.Path, "main", "main", "")
	if err != ErrDefaultBranchProtected {
		t.Fatalf("expected ErrDefaultBranchProtected when pushing to main, got %v", err)
	}

	err = client.PushTaskBranch(ctx, ws.Path, "master", "main", "")
	if err != ErrDefaultBranchProtected {
		t.Fatalf("expected ErrDefaultBranchProtected when pushing to master, got %v", err)
	}

	// 2. Pre-push hook guardrail test:
	// Verify that the pre-push hook installed in .git/hooks/pre-push rejects pushes to main
	hookPath := filepath.Join(ws.Path, ".git", "hooks", "pre-push")
	if fi, err := os.Stat(hookPath); err != nil || fi.Mode()&0111 == 0 {
		t.Fatalf("pre-push hook not found or not executable at %s", hookPath)
	}

	// Simulate pushing refs to main through the hook
	hookCmd := exec.Command(hookPath)
	hookCmd.Stdin = strings.NewReader("refs/heads/task_protect_test 1111 refs/heads/main 2222\n")
	hookOut, hookErr := hookCmd.CombinedOutput()
	if hookErr == nil {
		t.Fatalf("pre-push hook should have rejected push to main, but succeeded!")
	}
	if !strings.Contains(string(hookOut), "PROHIBITED") {
		t.Fatalf("expected PROHIBITED in hook output, got: %s", string(hookOut))
	}
}

func TestWorkspaceRevisionResume(t *testing.T) {
	tempDir := t.TempDir()
	baseRepoDir := filepath.Join(tempDir, "base_repo")
	workspacesDir := filepath.Join(tempDir, "workspaces")

	createTestBaseRepo(t, baseRepoDir)

	client := NewClient()
	ctx := context.Background()

	// Initial cycle: prepare workspace
	ws1, err := client.PrepareTaskWorkspace(ctx, WorkspaceConfig{
		TaskID:            "task_revision_cycle",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: workspacesDir,
		BranchName:        "task/task_revision_cycle",
	})
	if err != nil {
		t.Fatalf("initial workspace prepare: %v", err)
	}

	// Initial commit
	initialFile := filepath.Join(ws1.Path, "cycle1.txt")
	_ = os.WriteFile(initialFile, []byte("cycle 1\n"), 0644)
	sha1, err := client.CommitChanges(ctx, ws1.Path, "feat: cycle 1", "Buchin", "buchin@dropsugar.com")
	if err != nil {
		t.Fatalf("commit cycle 1: %v", err)
	}

	// Revision cycle: re-prepare same workspace for revision
	ws2, err := client.PrepareTaskWorkspace(ctx, WorkspaceConfig{
		TaskID:            "task_revision_cycle",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: workspacesDir,
		BranchName:        "task/task_revision_cycle",
	})
	if err != nil {
		t.Fatalf("revision workspace prepare: %v", err)
	}

	if ws1.Path != ws2.Path {
		t.Fatalf("workspace paths differ across revision: %s vs %s", ws1.Path, ws2.Path)
	}

	// Verify cycle1.txt still exists
	if _, err := os.Stat(filepath.Join(ws2.Path, "cycle1.txt")); err != nil {
		t.Fatalf("cycle1.txt was lost during workspace resume!")
	}

	// Make revision changes
	revFile := filepath.Join(ws2.Path, "cycle2.txt")
	_ = os.WriteFile(revFile, []byte("cycle 2 revision\n"), 0644)
	sha2, err := client.CommitChanges(ctx, ws2.Path, "fix: cycle 2 revision", "Buchin", "buchin@dropsugar.com")
	if err != nil {
		t.Fatalf("commit cycle 2: %v", err)
	}

	if sha1 == sha2 {
		t.Fatalf("expected new commit sha for revision, got same: %s", sha1)
	}

	diffSummary, err := client.GetDiffSummary(ctx, ws2.Path, "main")
	if err != nil {
		t.Fatalf("get diff summary: %v", err)
	}
	if !strings.Contains(diffSummary, "cycle1.txt") || !strings.Contains(diffSummary, "cycle2.txt") {
		t.Fatalf("diff summary should contain both files, got:\n%s", diffSummary)
	}
}

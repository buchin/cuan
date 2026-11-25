package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
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

func TestPruneStaleWorkspaces(t *testing.T) {
	tempDir := t.TempDir()
	base := filepath.Join(tempDir, "backed-up-repo")
	createTestBaseRepo(t, base)
	workspacesDir := filepath.Join(tempDir, "workspaces")
	if err := os.MkdirAll(workspacesDir, 0755); err != nil {
		t.Fatalf("mkdir workspaces: %v", err)
	}

	locksDir := filepath.Join(workspacesDir, ".task-execution-locks")
	if err := os.MkdirAll(locksDir, 0700); err != nil {
		t.Fatalf("mkdir locks dir: %v", err)
	}

	now := time.Now()

	createWs := func(name string, modTime time.Time) string {
		p := filepath.Join(workspacesDir, name)
		if out, err := exec.Command("git", "clone", base, p).CombinedOutput(); err != nil {
			t.Fatalf("clone ws %s: %s", name, out)
		}
		_ = os.Chtimes(p, modTime, modTime)
		return p
	}

	// 1. Workspaces older than 24h
	oldWs1 := createWs("ws_old_1", now.Add(-48*time.Hour))
	oldWs2 := createWs("ws_old_2", now.Add(-36*time.Hour))

	// 2. Recent workspaces (< 24h)
	recentWs1 := createWs("ws_recent_1", now.Add(-3*time.Hour))
	recentWs2 := createWs("ws_recent_2", now.Add(-2*time.Hour))
	recentWs3 := createWs("ws_recent_3", now.Add(-1*time.Hour))

	// 3. Locked workspace (older than 24h, but locked with OS flock)
	lockedWs := createWs("ws_locked", now.Add(-50*time.Hour))
	lockFile := filepath.Join(locksDir, "ws_locked.lock")
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("flock error: %v", err)
	}
	defer func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}()

	// 4. Active task workspace (older than 24h, but matches activeTaskID)
	activeWs := createWs("ws_active", now.Add(-50*time.Hour))

	// 5. Non-directory regular file inside workspacesDir
	regularFile := filepath.Join(workspacesDir, "info.txt")
	_ = os.WriteFile(regularFile, []byte("standalone file"), 0644)

	client := NewClient()

	// Prune with maxAge=24h, activeTaskID="ws_active", maxWorkspaces=2
	// Expected:
	// - ws_old_1 and ws_old_2 pruned (older than 24h)
	// - ws_locked kept (lock held)
	// - ws_active kept (active task)
	// - Among recent: ws_recent_1, ws_recent_2, ws_recent_3 (3 items) with quota 2 -> oldest (ws_recent_1) pruned
	// Total pruned: 3 (ws_old_1, ws_old_2, ws_recent_1)
	pruned, err := client.PruneStaleWorkspaces(workspacesDir, 24*time.Hour, "ws_active", 2)
	if err != nil {
		t.Fatalf("PruneStaleWorkspaces failed: %v", err)
	}

	if pruned != 3 {
		t.Fatalf("expected 3 pruned workspaces, got %d", pruned)
	}

	// Verify old workspaces deleted
	if _, err := os.Stat(oldWs1); !os.IsNotExist(err) {
		t.Errorf("expected ws_old_1 to be pruned, but still exists")
	}
	if _, err := os.Stat(oldWs2); !os.IsNotExist(err) {
		t.Errorf("expected ws_old_2 to be pruned, but still exists")
	}

	// Verify excess recent workspace deleted
	if _, err := os.Stat(recentWs1); !os.IsNotExist(err) {
		t.Errorf("expected ws_recent_1 to be pruned by quota, but still exists")
	}

	// Verify kept recent workspaces
	if _, err := os.Stat(recentWs2); err != nil {
		t.Errorf("expected ws_recent_2 to exist, got err: %v", err)
	}
	if _, err := os.Stat(recentWs3); err != nil {
		t.Errorf("expected ws_recent_3 to exist, got err: %v", err)
	}

	// Verify locked workspace kept
	if _, err := os.Stat(lockedWs); err != nil {
		t.Errorf("expected ws_locked to be preserved due to active lock, got err: %v", err)
	}

	// Verify active workspace kept
	if _, err := os.Stat(activeWs); err != nil {
		t.Errorf("expected ws_active to be preserved, got err: %v", err)
	}

	// Verify hidden directory and regular file preserved
	if _, err := os.Stat(locksDir); err != nil {
		t.Errorf("expected .task-execution-locks directory to be preserved, got err: %v", err)
	}
	if _, err := os.Stat(regularFile); err != nil {
		t.Errorf("expected regularFile to be preserved, got err: %v", err)
	}
}

func TestPrepareTaskWorkspace_AutomaticPruning(t *testing.T) {
	tempDir := t.TempDir()
	baseRepoDir := filepath.Join(tempDir, "base_repo")
	workspacesDir := filepath.Join(tempDir, "workspaces")

	createTestBaseRepo(t, baseRepoDir)

	client := NewClient()
	ctx := context.Background()

	// Age alone does not prove unknown workspace contents are disposable.
	oldWsDir := filepath.Join(workspacesDir, "old_abandoned_task")
	_ = os.MkdirAll(oldWsDir, 0755)
	oldTime := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(oldWsDir, oldTime, oldTime)

	// Now call PrepareTaskWorkspace for a new task
	ws, err := client.PrepareTaskWorkspace(ctx, WorkspaceConfig{
		TaskID:            "new_task_001",
		BaseRepoDir:       baseRepoDir,
		WorkspacesBaseDir: workspacesDir,
		AuthorName:        "Buchin",
		AuthorEmail:       "buchin@dropsugar.com",
	})
	if err != nil {
		t.Fatalf("PrepareTaskWorkspace failed: %v", err)
	}

	// Preserve unknown contents while preparing the next task normally.
	if _, err := os.Stat(oldWsDir); err != nil {
		t.Errorf("unknown abandoned workspace must be retained: %v", err)
	}

	// The new workspace must exist
	if _, err := os.Stat(ws.Path); err != nil {
		t.Errorf("new workspace path %s does not exist: %v", ws.Path, err)
	}
}

func TestPruneMergedCuanBranches(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	createTestBaseRepo(t, repoDir)

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}

	// Ensure default branch is named main or dev
	defaultBranch := runGit("symbolic-ref", "--short", "HEAD")

	// 1. Merged cuan/task-* without worktree -> should be pruned
	runGit("branch", "cuan/task-merged-1")
	// 2. Merged cuan/recovered-* without worktree -> should be pruned
	runGit("branch", "cuan/recovered-merged-1")
	// 3. Non-cuan merged branch -> should be preserved
	runGit("branch", "feature/keep-me")
	// 4. Merged cuan/task-* bound to an active worktree -> should be preserved
	wtDir := filepath.Join(tempDir, "active-wt")
	runGit("worktree", "add", "-b", "cuan/task-active-wt", wtDir, defaultBranch)
	// 5. Unmerged cuan/task-* -> should be preserved
	runGit("checkout", "-b", "cuan/task-unmerged-1")
	if err := os.WriteFile(filepath.Join(repoDir, "unmerged.txt"), []byte("wip"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "unmerged.txt")
	runGit("commit", "-m", "unmerged work")
	runGit("checkout", defaultBranch)

	client := NewClient()
	pruned, err := client.PruneMergedCuanBranches(context.Background(), repoDir, defaultBranch)
	if err != nil {
		t.Fatalf("PruneMergedCuanBranches failed: %v", err)
	}
	if pruned != 2 {
		t.Fatalf("expected 2 pruned branches, got %d", pruned)
	}

	hasBranch := func(name string) bool {
		err := exec.Command("git", "-C", repoDir, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run()
		return err == nil
	}
	if hasBranch("cuan/task-merged-1") || hasBranch("cuan/recovered-merged-1") {
		t.Fatal("expected merged non-worktree cuan/* branches to be deleted")
	}
	for _, keep := range []string{"feature/keep-me", "cuan/task-active-wt", "cuan/task-unmerged-1"} {
		if !hasBranch(keep) {
			t.Fatalf("expected branch %s to be preserved", keep)
		}
	}
}

func TestPruneReconstructedWorkspaces(t *testing.T) {
	tempDir := t.TempDir()
	base := filepath.Join(tempDir, "base-repo")
	createTestBaseRepo(t, base)

	recBase := filepath.Join(tempDir, "reconstructed-workspaces")
	now := time.Now()

	makeRec := func(key string, modTime time.Time) string {
		wt := filepath.Join(recBase, key, "worktree")
		if err := os.MkdirAll(filepath.Dir(wt), 0755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("git", "clone", base, wt).CombinedOutput(); err != nil {
			t.Fatalf("clone %s: %s", key, out)
		}
		_ = os.Chtimes(wt, modTime, modTime)
		return wt
	}

	oldWt := makeRec("old_key", now.Add(-10*24*time.Hour))
	recentWt := makeRec("recent_key", now.Add(-2*time.Hour))

	client := NewClient()
	pruned, err := client.PruneReconstructedWorkspaces(recBase, 7*24*time.Hour, 10)
	if err != nil {
		t.Fatalf("PruneReconstructedWorkspaces failed: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned reconstructed workspace, got %d", pruned)
	}
	if _, err := os.Stat(filepath.Dir(oldWt)); !os.IsNotExist(err) {
		t.Fatalf("expected old reconstructed parent dir to be removed")
	}
	if _, err := os.Stat(recentWt); err != nil {
		t.Fatalf("expected recent reconstructed workspace to remain: %v", err)
	}
}

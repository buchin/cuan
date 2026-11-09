package git

import (
	"context"
	"github.com/buchin/cuan/internal/agent"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func retentionGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return string(out)
}

func TestResumePreservesUnresolvedMergeAndUntrackedProgress(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	createTestBaseRepo(t, base)
	c := NewClient()
	cfg := WorkspaceConfig{TaskID: "task", BaseRepoDir: base, WorkspacesBaseDir: filepath.Join(root, "workspaces"), BranchName: "task/retained"}
	ws, err := c.PrepareTaskWorkspace(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	retentionGit(t, ws.Path, "checkout", "-b", "conflicting")
	os.WriteFile(filepath.Join(ws.Path, "README.md"), []byte("other\n"), 0644)
	retentionGit(t, ws.Path, "commit", "-am", "other change")
	retentionGit(t, ws.Path, "checkout", cfg.BranchName)
	os.WriteFile(filepath.Join(ws.Path, "README.md"), []byte("task\n"), 0644)
	retentionGit(t, ws.Path, "commit", "-am", "task change")
	if exec.Command("git", "-C", ws.Path, "merge", "conflicting").Run() == nil {
		t.Fatal("fixture must conflict")
	}
	os.WriteFile(filepath.Join(ws.Path, "checkpoint.txt"), []byte("retained findings"), 0644)
	before := retentionGit(t, ws.Path, "ls-files", "--unmerged") + retentionGit(t, ws.Path, "diff", "--binary") + retentionGit(t, ws.Path, "show-ref")
	if _, err = c.PrepareTaskWorkspace(context.Background(), cfg); err != nil {
		t.Fatal("same-branch conflict must remain available to agent", err)
	}
	after := retentionGit(t, ws.Path, "ls-files", "--unmerged") + retentionGit(t, ws.Path, "diff", "--binary") + retentionGit(t, ws.Path, "show-ref")
	if before != after {
		t.Fatal("resume destroyed merge/index/branch evidence")
	}
	if content, err := os.ReadFile(filepath.Join(ws.Path, "checkpoint.txt")); err != nil || string(content) != "retained findings" {
		t.Fatal("saved progress lost")
	}
}

func TestResumeDoesNotRewindExistingTaskBranchFromAnotherBranch(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	createTestBaseRepo(t, base)
	c := NewClient()
	cfg := WorkspaceConfig{TaskID: "task", BaseRepoDir: base, WorkspacesBaseDir: filepath.Join(root, "workspaces"), BranchName: "task/retained"}
	ws, err := c.PrepareTaskWorkspace(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(ws.Path, "README.md"), []byte("approved task commit"), 0644)
	retentionGit(t, ws.Path, "commit", "-am", "retained result")
	taskHead := retentionGit(t, ws.Path, "rev-parse", cfg.BranchName)
	retentionGit(t, ws.Path, "checkout", "main")
	os.WriteFile(filepath.Join(ws.Path, "README.md"), []byte("unsaved other state"), 0644)
	os.WriteFile(filepath.Join(ws.Path, "untracked.txt"), []byte("do not clean"), 0644)
	if _, err = c.PrepareTaskWorkspace(context.Background(), cfg); err == nil {
		t.Fatal("conflicting branch switch must not overwrite saved work")
	}
	if retentionGit(t, ws.Path, "rev-parse", cfg.BranchName) != taskHead {
		t.Fatal("task branch was rewound")
	}
	if content, _ := os.ReadFile(filepath.Join(ws.Path, "README.md")); string(content) != "unsaved other state" {
		t.Fatal("dirty work discarded")
	}
	if _, err := os.Stat(filepath.Join(ws.Path, "untracked.txt")); err != nil {
		t.Fatal("untracked work removed")
	}
}

func TestFreshWorkspaceRestoresPublishedTaskBranch(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	createTestBaseRepo(t, base)
	retentionGit(t, base, "checkout", "-b", "task/published")
	os.WriteFile(filepath.Join(base, "retained.txt"), []byte("published work"), 0644)
	retentionGit(t, base, "add", ".")
	retentionGit(t, base, "commit", "-m", "published result")
	head := retentionGit(t, base, "rev-parse", "HEAD")
	retentionGit(t, base, "checkout", "main")
	ws, err := NewClient().PrepareTaskWorkspace(context.Background(), WorkspaceConfig{TaskID: "task", BaseRepoDir: base, WorkspacesBaseDir: filepath.Join(root, "workspaces"), BranchName: "task/published"})
	if err != nil {
		t.Fatal(err)
	}
	if retentionGit(t, ws.Path, "rev-parse", "HEAD") != head {
		t.Fatal("remote task progress was replaced with default branch")
	}
}

func TestPruningPreservesUnverifiedWorkDespiteAgeAndQuota(t *testing.T) {
	for _, kind := range []string{"dirty", "untracked", "ignored", "unpushed", "stash", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "base")
			createTestBaseRepo(t, base)
			workspaces := filepath.Join(root, "workspaces")
			dir := filepath.Join(workspaces, "old-task")
			os.MkdirAll(workspaces, 0755)
			if kind == "unknown" {
				os.MkdirAll(dir, 0755)
			} else {
				if out, err := exec.Command("git", "clone", base, dir).CombinedOutput(); err != nil {
					t.Fatal(string(out))
				}
				retentionGit(t, dir, "config", "user.name", "Fixture")
				retentionGit(t, dir, "config", "user.email", "fixture@example.invalid")
			}
			switch kind {
			case "dirty":
				os.WriteFile(filepath.Join(dir, "README.md"), []byte("uncommitted work"), 0644)
			case "untracked", "unknown":
				os.WriteFile(filepath.Join(dir, "findings.txt"), []byte("findings"), 0644)
			case "ignored":
				os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"), []byte("checkpoint.txt\n"), 0644)
				os.WriteFile(filepath.Join(dir, "checkpoint.txt"), []byte("checkpoint"), 0644)
			case "unpushed":
				os.WriteFile(filepath.Join(dir, "README.md"), []byte("local result"), 0644)
				retentionGit(t, dir, "commit", "-am", "local")
			case "stash":
				os.WriteFile(filepath.Join(dir, "README.md"), []byte("stashed result"), 0644)
				retentionGit(t, dir, "stash")
			}
			old := time.Now().Add(-48 * time.Hour)
			os.Chtimes(dir, old, old)
			count, err := NewClient().PruneStaleWorkspaces(workspaces, time.Hour, "", 1)
			if err != nil || count != 0 {
				t.Fatal("saved work pruned", count, err)
			}
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				t.Fatal("workspace removed")
			}
			if kind == "stash" && !strings.Contains(retentionGit(t, dir, "stash", "list"), "stashed") { // Check actual stored diff, not its generated title.
				if !strings.Contains(retentionGit(t, dir, "stash", "show", "-p"), "stashed result") {
					t.Fatal("stash lost")
				}
			}
		})
	}
}

func TestPruningReclaimsOnlyExactGeneratedWorkspaceConfiguration(t *testing.T) {
	for _, scenario := range []string{"generated", "edited-config", "extra-report", "extra-empty-directory", "other-ignored-data"} {
		t.Run(scenario, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "base")
			createTestBaseRepo(t, base)
			os.WriteFile(filepath.Join(base, ".gitignore"), []byte(".agents/\n.gemini/\nlocal-data/\n"), 0600)
			retentionGit(t, base, "add", ".gitignore")
			retentionGit(t, base, "commit", "-m", "Ignore generated configuration")
			workspaces := t.TempDir()
			workspace := filepath.Join(workspaces, "old-task")
			if out, err := exec.Command("git", "clone", base, workspace).CombinedOutput(); err != nil {
				t.Fatal(string(out))
			}
			if err := agent.EnsureWorkspaceMCPConfig(workspace); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "edited-config":
				os.WriteFile(filepath.Join(workspace, ".agents", "mcp_config.json"), []byte("custom configuration"), 0600)
			case "extra-report":
				os.WriteFile(filepath.Join(workspace, ".gemini", "findings.md"), []byte("preserve findings"), 0600)
			case "extra-empty-directory":
				os.Mkdir(filepath.Join(workspace, ".agents", "saved"), 0700)
			case "other-ignored-data":
				os.Mkdir(filepath.Join(workspace, "local-data"), 0700)
				os.WriteFile(filepath.Join(workspace, "local-data", "report.txt"), []byte("retain"), 0600)
			}
			old := time.Now().Add(-48 * time.Hour)
			os.Chtimes(workspace, old, old)
			count, err := NewClient().PruneStaleWorkspaces(workspaces, time.Hour, "", 1)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "generated" {
				if count != 1 {
					t.Fatal("Cuan's generated defaults prevented reclaim", count)
				}
				if _, err := os.Stat(workspace); !os.IsNotExist(err) {
					t.Fatal("published disposable workspace retained")
				}
			} else {
				if count != 0 {
					t.Fatal("unique data deleted", scenario)
				}
				if _, err := os.Stat(workspace); err != nil {
					t.Fatal("saved workspace lost", err)
				}
			}
		})
	}
}

package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalBranchApplyPreservesWorkAndUsesExactSource(t *testing.T) {
	for _, scenario := range []string{"success", "dirty", "wrong source", "missing target", "target busy", "missing task", "invalid task", "conflict"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatal("fixture git failed", args, string(out))
				}
				return strings.TrimSpace(string(out))
			}
			run("init", "--initial-branch=main")
			run("config", "user.name", "Fixture Human")
			run("config", "user.email", "human@example.invalid")
			write := func(name, content string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("common.txt", "base\n")
			run("add", ".")
			run("commit", "-m", "Base")
			run("checkout", "-b", "task/one")
			write("common.txt", "task\n")
			run("add", ".")
			run("commit", "-m", "Task")
			source := run("rev-parse", "HEAD")
			run("checkout", "main")
			if scenario == "conflict" {
				write("common.txt", "target\n")
				run("add", ".")
				run("commit", "-m", "Target change")
			}
			targetBefore := run("rev-parse", "HEAD")
			run("checkout", "-b", "side")
			write("side.txt", "retained\n")
			run("add", ".")
			run("commit", "-m", "Unrelated side")
			before := run("rev-parse", "HEAD")
			base, branch, expected := "main", "task/one", source
			switch scenario {
			case "target busy":
				run("worktree", "add", filepath.Join(t.TempDir(), "target"), "main")
			case "dirty":
				write("pending.txt", "do not discard\n")
			case "wrong source":
				expected = strings.Repeat("a", 40)
			case "missing target":
				base = "absent"
			case "missing task":
				branch = "task/absent"
			case "invalid task":
				branch = "--orphan=wrong"
			}
			result, err := NewClient().MergeLocalBranch(context.Background(), root, base, branch, expected, "Apply exact source")
			if scenario == "success" {
				if err != nil || result == nil || !IsCommitHash(result.SHA) || run("symbolic-ref", "--short", "HEAD") != "main" || result.SHA != run("rev-parse", "HEAD") || run("status", "--porcelain") != "" {
					t.Fatal("exact local merge failed", err)
				}
				run("merge-base", "--is-ancestor", source, "HEAD")
			} else {
				if err == nil || result != nil {
					t.Fatal("failed merge accepted")
				}
				if scenario == "conflict" {
					if run("rev-parse", "HEAD") != targetBefore || run("rev-parse", "MERGE_HEAD") != source || !strings.Contains(run("status", "--porcelain"), "UU common.txt") {
						t.Fatal("merge conflict was discarded or misattributed")
					}
				} else if run("rev-parse", "HEAD") != before || run("symbolic-ref", "--short", "HEAD") != "side" {
					t.Fatal("failed preflight changed current branch or source")
				}
			}
			if run("rev-parse", "refs/heads/task/one") != source || run("rev-parse", "refs/heads/side") != before {
				t.Fatal("retained task or unrelated branch changed")
			}
			if scenario == "dirty" {
				data, _ := os.ReadFile(filepath.Join(root, "pending.txt"))
				if string(data) != "do not discard\n" {
					t.Fatal("pending file lost")
				}
			}
		})
	}
}

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationAncestryNeedsCompleteTargetHistory(t *testing.T) {
	for _, scenario := range []string{"contained", "known nonancestor", "unpublished source", "unknown target", "shallow unpublished", "shallow known nonancestor", "missing parent", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("fixture git: %v %s", err, out)
				}
				return strings.TrimSpace(string(out))
			}
			run("init", "--initial-branch=dev")
			run("config", "user.name", "Fixture Human")
			run("config", "user.email", "human@example.invalid")
			run("commit", "--allow-empty", "-m", "Root")
			ancestor := run("rev-parse", "HEAD")
			run("commit", "--allow-empty", "-m", "Target")
			target := run("rev-parse", "HEAD")
			run("checkout", "--orphan", "other")
			run("commit", "--allow-empty", "-m", "Other source")
			other := run("rev-parse", "HEAD")
			run("checkout", "dev")
			source := other
			switch scenario {
			case "contained":
				source = ancestor
			case "unpublished source", "shallow unpublished", "missing parent":
				source = strings.Repeat("f", 40)
			case "unknown target":
				target = strings.Repeat("e", 40)
			}
			if strings.HasPrefix(scenario, "shallow") {
				if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), []byte(target+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing parent" {
				if err := os.Remove(filepath.Join(root, ".git", "objects", ancestor[:2], ancestor[2:])); err != nil {
					t.Fatal(err)
				}
			}
			before := run("status", "--porcelain")
			refs := run("for-each-ref", "--format=%(refname):%(objectname)")
			ctx := context.Background()
			if scenario == "cancelled" {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			contained, err := NewClient().CommitContained(ctx, root, source, target)
			unknown := scenario == "unknown target" || strings.HasPrefix(scenario, "shallow") || scenario == "missing parent" || scenario == "cancelled"
			if unknown {
				if err == nil || contained {
					t.Fatalf("incomplete ancestry became conclusive: contained=%v err=%v", contained, err)
				}
			} else if err != nil || contained != (scenario == "contained") {
				t.Fatalf("wrong complete-history observation: contained=%v err=%v", contained, err)
			}
			if scenario == "shallow unpublished" && !errors.Is(err, ErrVerificationCommitUnavailable) {
				t.Fatal("missing source lost recoverable unknown state", err)
			}
			if run("status", "--porcelain") != before || run("for-each-ref", "--format=%(refname):%(objectname)") != refs {
				t.Fatal("ancestry inspection mutated working tree or refs")
			}
		})
	}
}

func TestUnpublishedSourceAncestryStreamsCompleteHistory(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "--initial-branch=dev", root).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	var history strings.Builder
	for i := 1; i <= 2000; i++ {
		fmt.Fprintf(&history, "commit refs/heads/dev\nmark :%d\ncommitter Fixture Human <human@example.invalid> %d +0000\ndata 1\nx\n", i, i)
		if i > 1 {
			fmt.Fprintf(&history, "from :%d\n", i-1)
		}
		history.WriteString("\n")
	}
	importer := exec.Command("git", "-C", root, "fast-import", "--quiet")
	importer.Stdin = strings.NewReader(history.String())
	if out, err := importer.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	// More than 64 KiB of commit IDs must not be truncated into a false result
	// or rejected by the ordinary bounded snapshot-output collector.
	contained, err := NewClient().CommitContained(context.Background(), root, strings.Repeat("f", 40), strings.TrimSpace(string(head)))
	if err != nil || contained {
		t.Fatal("complete history did not stream independently of output size", contained, err)
	}
}

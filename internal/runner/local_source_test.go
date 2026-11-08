package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cuangit "github.com/buchin/cuan/internal/git"
)

func TestLocalImplementationSourceRecordsCheckpointWithoutPublication(t *testing.T) {
	for _, scenario := range []string{"clean", "dirty", "untracked", "unchanged", "missing baseline"} {
		t.Run(scenario, func(t *testing.T) {
			root := documentationFixture(t)
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatal(err, string(out))
				}
				return strings.TrimSpace(string(out))
			}
			base := run("rev-parse", "HEAD")
			if scenario != "unchanged" {
				if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package feature\n"), 0600); err != nil {
					t.Fatal(err)
				}
				run("add", "feature.go")
				run("commit", "-m", "Approved checkpoint")
			}
			if scenario == "dirty" || scenario == "untracked" {
				name := "feature.go"
				if scenario == "untracked" {
					name = "pending.txt"
				}
				if err := os.WriteFile(filepath.Join(root, name), []byte("preserve pending work\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing baseline" {
				base = strings.Repeat("f", 40)
			}
			head, status, refs := run("rev-parse", "HEAD"), run("status", "--porcelain"), run("show-ref")
			result := CompleteAttemptRequest{}
			runner := &Runner{gitClient: cuangit.NewClient()}
			err := runner.observeLocalImplementationSource(context.Background(), &cuangit.TaskWorkspace{Path: root, BaseCommit: base}, "dev", &result)
			if (err != nil) != (scenario == "missing baseline") {
				t.Fatal("comparison failure was hidden", err)
			}
			if scenario == "clean" {
				if result.CommitSHA != head || result.BranchName != run("rev-parse", "--abbrev-ref", "HEAD") || !strings.Contains(result.DiffSummary, "feature.go") || result.HasCodeChanges == nil || !*result.HasCodeChanges {
					t.Fatal("local implementation source not recorded", result)
				}
			} else if result.CommitSHA != "" || result.DiffSummary != "" || result.HasCodeChanges != nil {
				t.Fatal("dirty, absent or unverifiable work attributed to HEAD", result)
			}
			if result.LiveDeploymentConfirmed || result.ManagedReleaseSource != nil || result.RequiresDeploy != nil || result.PRURL != "" || result.PRNumber != 0 {
				t.Fatal("source observation manufactured publication authority")
			}
			if head != run("rev-parse", "HEAD") || status != run("status", "--porcelain") || refs != run("show-ref") {
				t.Fatal("source observation modified local work or refs")
			}
		})
	}
}

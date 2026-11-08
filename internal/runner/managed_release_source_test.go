package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	cuanGit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedReleaseSourceObservesExactDeveloperCheckpointWithoutExecution(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong commit", "wrong branch", "dirty", "failed checks", "missing descriptor", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".cuan"), 0700); err != nil {
				t.Fatal(err)
			}
			manifest := `{"version":1,"command":["release.sh"],"timeout_seconds":5}`
			for name, body := range map[string]string{".cuan/release.json": manifest, "release.sh": "#!/bin/sh\ntouch executed\n"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing descriptor" {
				if err := os.Remove(filepath.Join(root, ".cuan/release.json")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "symlink" {
				if err := os.Remove(filepath.Join(root, ".cuan/release.json")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../release.sh", filepath.Join(root, ".cuan/release.json")); err != nil {
					t.Fatal(err)
				}
			}
			git := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("fixture git failed: %s", out)
				}
				return strings.TrimSpace(string(out))
			}
			git("init", "--initial-branch=task/dev")
			git("add", ".")
			git("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Approved implementation")
			commit := git("rev-parse", "HEAD")
			branch := "task/dev"
			summary := "Project verification: test.sh --fast (exit 0)"
			switch scenario {
			case "wrong commit":
				commit = strings.Repeat("f", 40)
			case "wrong branch":
				branch = "dev"
			case "dirty":
				if err := os.WriteFile(filepath.Join(root, "uncommitted"), []byte("pending"), 0600); err != nil {
					t.Fatal(err)
				}
			case "failed checks":
				summary = "Project verification failed"
			}
			evidence, err := observeManagedReleaseSource(context.Background(), root, commit, branch, summary)
			if scenario == "valid" || scenario == "dirty" || scenario == "failed checks" {
				unchanged := false
				req := CompleteAttemptRequest{TestSummary: summary, HasCodeChanges: &unchanged, RequiresDeploy: &unchanged}
				err := observeUnchangedManagedSource(context.Background(), &cuanGit.TaskWorkspace{Path: root, BranchName: branch}, &req)
				if scenario == "valid" {
					if err != nil || req.ManagedReleaseSource == nil || req.CommitSHA != commit || req.BranchName != branch || req.DiffSummary == "" || req.LiveDeploymentConfirmed || *req.HasCodeChanges || *req.RequiresDeploy || git("rev-parse", "HEAD") != commit {
						t.Fatal("unchanged checked source lost identity or invented effects", err)
					}
				} else if err == nil || req.ManagedReleaseSource != nil {
					t.Fatal("dirty or unchecked unchanged source received a handoff")
				}
			}
			if scenario == "valid" {
				hash := sha256.Sum256([]byte(manifest))
				if err != nil || evidence == nil || evidence.Commit != commit || evidence.Branch != "task/dev" || evidence.ManifestHash != hex.EncodeToString(hash[:]) || evidence.VerificationHash == "" {
					t.Fatalf("source observation invalid: %v", err)
				}
			} else if err == nil || evidence != nil {
				t.Fatal("unverified source accepted")
			}
			if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
				t.Fatal("source observation executed release")
			}
		})
	}
}

func TestManagedReleaseSourceCompletionCannotInventOwnership(t *testing.T) {
	for _, scenario := range []string{"valid", "absent", "no binding", "commit", "branch", "checks", "manifest"} {
		t.Run(scenario, func(t *testing.T) {
			binding := &ManagedReleaseClaim{Version: 1}
			req := CompleteAttemptRequest{CommitSHA: strings.Repeat("a", 40), BranchName: "task/dev", TestSummary: "Project verification: test.sh --fast (exit 0)"}
			req.ManagedReleaseSource = &ManagedReleaseSource{Commit: req.CommitSHA, Branch: req.BranchName, ManifestHash: strings.Repeat("b", 64), VerificationHash: task.VerificationSummaryHash(req.TestSummary)}
			switch scenario {
			case "absent":
				req.ManagedReleaseSource = nil
				binding = nil
			case "no binding":
				binding = nil
			case "commit":
				req.CommitSHA = strings.Repeat("c", 40)
			case "branch":
				req.BranchName = "other"
			case "checks":
				req.TestSummary = "Project verification failed"
			case "manifest":
				req.ManagedReleaseSource.ManifestHash = strings.Repeat("z", 64)
			}
			err := validateManagedReleaseSource(binding, req)
			if (err == nil) != (scenario == "valid" || scenario == "absent") {
				t.Fatalf("unexpected source validation: %v", err)
			}
		})
	}
}

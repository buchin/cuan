package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkspacePreservesDeveloperAndPinsSourceTargetBranch(t *testing.T) {
	for _, scenario := range []string{"valid", "dirty destination", "changed branch", "changed receipt", "wrong ancestry", "symlink", "invalid branch"} {
		t.Run(scenario, func(t *testing.T) {
			source, developer, original, target, run := snapshotFixture(t)
			sourceState := run(source, "status", "--porcelain=v1")
			devState := run(developer, "status", "--porcelain=v1")
			refs := run(source, "for-each-ref", "--format=%(refname):%(objectname)")
			destination := filepath.Join(t.TempDir(), "release")
			branch := "release/production"
			client := NewClient()
			ctx := context.Background()
			if scenario == "wrong ancestry" {
				original, target = target, original
			}
			if scenario == "invalid branch" {
				branch = "--orphan"
			}
			if scenario == "symlink" {
				if err := os.Symlink(source, destination); err != nil {
					t.Fatal(err)
				}
			}
			path, err := client.PrepareReleaseWorkspace(ctx, source, destination, original, target, branch)
			invalid := scenario == "wrong ancestry" || scenario == "invalid branch" || scenario == "symlink"
			if invalid {
				if err == nil || path != "" {
					t.Fatal("invalid release identity accepted")
				}
			} else {
				if err != nil || path != destination || run(path, "rev-parse", "HEAD") != target || run(path, "symbolic-ref", "--short", "HEAD") != branch {
					t.Fatal("release workspace not bound", err)
				}
				switch scenario {
				case "dirty destination":
					if err := os.WriteFile(filepath.Join(path, "pending.txt"), []byte("preserve"), 0600); err != nil {
						t.Fatal(err)
					}
				case "changed branch":
					run(path, "checkout", "-b", "unexpected")
				case "changed receipt":
					if err := os.WriteFile(filepath.Join(path, ".git", "cuan-release-workspace.json"), []byte(`{}`), 0600); err != nil {
						t.Fatal(err)
					}
				}
				again, reuseErr := client.PrepareReleaseWorkspace(ctx, source, destination, original, target, branch)
				if scenario == "valid" {
					if reuseErr != nil || again != destination {
						t.Fatal("exact workspace was not reused", reuseErr)
					}
				} else if reuseErr == nil || again != "" {
					t.Fatal("changed workspace silently repaired")
				}
				if scenario == "dirty destination" {
					data, _ := os.ReadFile(filepath.Join(path, "pending.txt"))
					if string(data) != "preserve" {
						t.Fatal("pending release work discarded")
					}
				}
				if scenario == "changed branch" && run(path, "symbolic-ref", "--short", "HEAD") != "unexpected" {
					t.Fatal("release branch reset")
				}
			}
			if run(source, "status", "--porcelain=v1") != sourceState || run(developer, "status", "--porcelain=v1") != devState || run(source, "for-each-ref", "--format=%(refname):%(objectname)") != refs {
				t.Fatal("release preparation changed source/developer")
			}
		})
	}
}

func TestReleaseWorkspaceCarriesFullCommitGraphWithoutHistoricalBlobs(t *testing.T) {
	source, _, active, _, run := snapshotFixture(t)
	var oldBlob string
	for i := 0; i < 7; i++ {
		content := strings.Repeat(fmt.Sprintf("historical-content-%d\n", i), 100)
		if err := os.WriteFile(filepath.Join(source, "history.txt"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		run(source, "add", "history.txt")
		run(source, "commit", "-m", fmt.Sprintf("History %d", i))
		if i == 0 {
			oldBlob = run(source, "rev-parse", "HEAD:history.txt")
		}
	}
	target := run(source, "rev-parse", "HEAD")
	result := run(source, "rev-parse", "HEAD^")
	root, err := NewClient().PrepareReleaseWorkspace(context.Background(), source, filepath.Join(t.TempDir(), "release"), result, target, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got := run(root, "rev-parse", "--is-shallow-repository"); got != "false" {
		t.Fatal("release ancestry graph remains shallow", got)
	}
	run(root, "merge-base", "--is-ancestor", active, target)
	// Exercise the same commit-only transport used by release activation.
	pack := exec.Command("git", "-C", root, "pack-objects", "--stdout")
	pack.Stdin = strings.NewReader(run(root, "rev-list", "HEAD") + "\n")
	packed, err := pack.Output()
	if err != nil {
		t.Fatal(err)
	}
	graph := filepath.Join(t.TempDir(), "graph.git")
	run(source, "init", "--bare", "--quiet", graph)
	index := exec.Command("git", "--git-dir", graph, "index-pack", "--stdin")
	index.Stdin = bytes.NewReader(packed)
	if output, err := index.CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	if output, err := exec.Command("git", "--git-dir", graph, "merge-base", "--is-ancestor", active, target).CombinedOutput(); err != nil {
		t.Fatal("activation graph cannot prove real ancestry", err, string(output))
	}
	if err := exec.Command("git", "-C", root, "cat-file", "-e", oldBlob).Run(); err == nil {
		t.Fatal("release copied historical source blobs")
	}
}

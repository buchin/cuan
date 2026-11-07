package git

import (
	"context"
	"os"
	"path/filepath"
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

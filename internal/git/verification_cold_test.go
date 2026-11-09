package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/agent"
)

func coldSnapshotFixture(t *testing.T) (string, string, string, string, func(string, ...string) string) {
	t.Helper()
	source, _, original, live, run := snapshotFixture(t)
	root := filepath.Join(t.TempDir(), "snapshot")
	if _, err := NewClient().PrepareVerificationSnapshot(context.Background(), source, root, original, live); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireVerificationLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	lease.Close()
	binding, err := os.ReadFile(filepath.Join(root, ".git", "cuan-verification.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".git", verificationColdMarker), binding, 0600); err != nil {
		t.Fatal(err)
	}
	run(root, "sparse-checkout", "set", "--no-cone", "!*")
	if _, err = os.Stat(filepath.Join(root, "feature.txt")); !os.IsNotExist(err) {
		t.Fatal("fixture was not cold", err)
	}
	return source, root, original, live, run
}

func TestColdVerificationRestoresExactIndependentSourceUnderLease(t *testing.T) {
	source, root, original, live, run := coldSnapshotFixture(t)
	client := NewClient()
	before, _ := os.Stat(filepath.Join(root, ".git", "cuan-verification.lock"))
	objects := run(root, "count-objects", "-v")
	if client.VerifySnapshotCommit(context.Background(), root, original, live) == nil {
		t.Fatal("cold tree accepted as actual source")
	}
	// Retained Git objects must be sufficient after the disposable source goes.
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireVerificationLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err = client.VerifySnapshotCommit(context.Background(), root, original, live); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "feature.txt"))
	if err != nil || string(data) != "LIVE_RESULT" {
		t.Fatal("exact source was not restored", err)
	}
	if _, err = os.Lstat(filepath.Join(root, ".git", verificationColdMarker)); !os.IsNotExist(err) {
		t.Fatal("cold marker remains", err)
	}
	after, _ := os.Stat(filepath.Join(root, ".git", "cuan-verification.lock"))
	if !os.SameFile(before, after) || objects != run(root, "count-objects", "-v") {
		t.Fatal("lease inode or retained objects changed")
	}
}

func TestColdVerificationPreservesUnknownFilesAndInvalidBindings(t *testing.T) {
	for _, mode := range []string{"tracked path with unique work", "untracked evidence", "mismatched marker", "linked marker"} {
		t.Run(mode, func(t *testing.T) {
			_, root, _, _, _ := coldSnapshotFixture(t)
			marker := filepath.Join(root, ".git", verificationColdMarker)
			var unique string
			switch mode {
			case "tracked path with unique work":
				unique = filepath.Join(root, "feature.txt")
			case "untracked evidence":
				unique = filepath.Join(root, "verification-report.md")
			case "mismatched marker":
				if err := os.WriteFile(marker, []byte(`{}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "linked marker":
				if err := os.Remove(marker); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, ".git", "cuan-verification.json"), marker); err != nil {
					t.Fatal(err)
				}
			}
			if unique != "" {
				if err := os.WriteFile(unique, []byte("PRESERVE_UNIQUE_WORK"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			lease, err := AcquireVerificationLease(context.Background(), root)
			if err == nil || lease != nil {
				t.Fatal("unproven restoration admitted")
			}
			if unique != "" {
				data, e := os.ReadFile(unique)
				if e != nil || string(data) != "PRESERVE_UNIQUE_WORK" {
					t.Fatal("unique work overwritten", e)
				}
			}
			if _, err = os.Lstat(marker); err != nil {
				t.Fatal("failed restoration erased its marker", err)
			}
		})
	}
}

func TestColdVerificationPreparationRestoresSavedSnapshot(t *testing.T) {
	source, root, original, live, _ := coldSnapshotFixture(t)
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	path, err := NewClient().PrepareVerificationSnapshot(context.Background(), source, root, original, live)
	if err != nil || path != root {
		t.Fatal("existing project inspection path did not restore", err)
	}
	if _, err = os.Stat(filepath.Join(root, "feature.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestColdVerificationPreservesGeneratedDefaultsAndInterruptedTransition(t *testing.T) {
	for _, mode := range []string{"generated defaults", "marker before sparsification"} {
		t.Run(mode, func(t *testing.T) {
			_, root, original, live, run := coldSnapshotFixture(t)
			if mode == "generated defaults" {
				if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".agents/\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(root, ".agents"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".agents", "mcp_config.json"), agent.GeneratedWorkspaceMCPConfig(), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				run(root, "sparse-checkout", "disable")
			}
			lease, err := AcquireVerificationLease(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			if NewClient().VerifySnapshotCommit(context.Background(), root, original, live) != nil {
				t.Fatal("restored source unavailable")
			}
			if mode == "generated defaults" {
				data, err := os.ReadFile(filepath.Join(root, ".agents", "mcp_config.json"))
				if err != nil || string(data) != string(agent.GeneratedWorkspaceMCPConfig()) {
					t.Fatal("generated config was changed", err)
				}
			}
		})
	}
}

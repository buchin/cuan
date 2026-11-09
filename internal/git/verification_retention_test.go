package git

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

func TestAcceptedSnapshotCompactorAndNativeLeaseRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	source, _, original, live, run := snapshotFixture(t)
	root := VerificationWorkspacePath(t.TempDir(), "accepted-fixture", live)
	client := NewClient()
	if _, err := client.PrepareVerificationSnapshot(ctx, source, root, original, live); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".agents/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".agents"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agents", "mcp_config.json"), agent.GeneratedWorkspaceMCPConfig(), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-72 * time.Hour)
	if err := client.MarkVerificationSnapshotAccepted(ctx, root, "accepted-fixture", original, live, old); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(filepath.Join(root, ".git", "cuan-verification.lock"))
	objects := run(root, "count-objects", "-v")
	if err := os.Chtimes(root, old, old); err != nil {
		t.Fatal(err)
	}
	proc := t.TempDir()
	script := os.Getenv("CUAN_TEST_SNAPSHOT_COMPACTOR")
	if script == "" {
		script, _ = filepath.Abs("../../scripts/compact-verification-source.py")
	}
	code := `import importlib.util,json,sys;from pathlib import Path
spec=importlib.util.spec_from_file_location('compactor',sys.argv[1]);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
print(json.dumps(module.compact(Path(sys.argv[2]),apply=True,proc=Path(sys.argv[3]))))`
	output, err := exec.Command("python3", "-c", code, script, filepath.Dir(root), proc).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	var result struct {
		Compacted []string `json:"compacted"`
		Bytes     int64    `json:"reclaimed_materialized_bytes"`
		Failed    int      `json:"failed"`
	}
	if json.Unmarshal(output, &result) != nil || len(result.Compacted) != 1 || result.Bytes <= 0 || result.Failed != 0 {
		t.Fatal("native accepted snapshot did not compact", string(output))
	}
	if _, err = os.Stat(filepath.Join(root, "feature.txt")); !os.IsNotExist(err) {
		t.Fatal("materialization was not released", err)
	}
	if run(root, "show", "HEAD:feature.txt") != "LIVE_RESULT" {
		t.Fatal("source objects were removed")
	}
	// Simulate an interrupted restoration that already wrote one exact file.
	if err = os.WriteFile(filepath.Join(root, "feature.txt"), []byte("LIVE_RESULT"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	lease, err := AcquireVerificationLease(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err = client.VerifySnapshotCommit(ctx, root, original, live); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(filepath.Join(root, ".git", "cuan-verification.lock"))
	if !os.SameFile(before, after) || objects != run(root, "count-objects", "-v") {
		t.Fatal("objects or lease identity changed")
	}
	data, err := os.ReadFile(filepath.Join(root, ".agents", "mcp_config.json"))
	if err != nil || string(data) != string(agent.GeneratedWorkspaceMCPConfig()) {
		t.Fatal("generated defaults changed", err)
	}
}

func TestAcceptedSnapshotReceiptRejectsUniqueSourceAndWrongIdentity(t *testing.T) {
	for _, mode := range []string{"unique report", "changed source", "wrong target"} {
		t.Run(mode, func(t *testing.T) {
			source, _, original, live, _ := snapshotFixture(t)
			root := filepath.Join(t.TempDir(), "snapshot")
			client := NewClient()
			if _, err := client.PrepareVerificationSnapshot(context.Background(), source, root, original, live); err != nil {
				t.Fatal(err)
			}
			target := live
			if mode == "wrong target" {
				target = strings.Repeat("a", 40)
			} else {
				name := "verification-report.md"
				if mode == "changed source" {
					name = "feature.txt"
				}
				if err := os.WriteFile(filepath.Join(root, name), []byte("PRESERVE_UNIQUE"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := client.MarkVerificationSnapshotAccepted(context.Background(), root, "task", original, target, time.Now()); err == nil {
				t.Fatal("unproven acceptance retention admitted")
			}
			if _, err := os.Stat(filepath.Join(root, ".git", "cuan-verification-accepted.json")); !os.IsNotExist(err) {
				t.Fatal("rejected snapshot received receipt", err)
			}
		})
	}
}

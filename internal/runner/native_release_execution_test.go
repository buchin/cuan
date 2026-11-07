package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestNativeReleaseExecutionUsesBoundCheckoutAndExplicitEnvironment(t *testing.T) {
	for _, scenario := range []string{"valid", "failed script", "nil environment", "wrong source", "wrong target", "wrong branch", "wrong manifest", "dirty"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("NATIVE_AMBIENT_CREDENTIAL", "must-not-inherit")
			source := t.TempDir()
			if err := os.Mkdir(filepath.Join(source, ".cuan"), 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "executed")
			script := "#!/bin/sh\nprintf '%s/%s' \"$BOUND_VALUE\" \"${NATIVE_AMBIENT_CREDENTIAL:-absent}\" > \"$MARKER\"\nprintf 'private child output\\n'\n"
			if scenario == "failed script" {
				script += "exit 7\n"
			}
			manifest := `{"version":1,"command":["release.sh"],"timeout_seconds":5}`
			for name, body := range map[string]string{".cuan/release.json": manifest, "release.sh": script} {
				if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git fixture: %s", out)
				}
				return strings.TrimSpace(string(out))
			}
			run("init", "--initial-branch=dev")
			run("add", ".")
			run("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Source")
			original := run("rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(source, "result.txt"), []byte("target"), 0600); err != nil {
				t.Fatal(err)
			}
			run("add", ".")
			run("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Merge target")
			target := run("rev-parse", "HEAD")
			workspace, err := git.NewClient().PrepareReleaseWorkspace(context.Background(), source, filepath.Join(t.TempDir(), "release"), original, target, "production")
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256([]byte(manifest))
			intent := &task.NativeReleaseIntent{SourceCommit: original, TargetBranch: "production", ManifestHash: hex.EncodeToString(hash[:])}
			prepared := &project.PreparedNativeReleaseWorkspace{Path: workspace, SourceCommit: original, TargetCommit: target, Branch: "production", ManifestHash: intent.ManifestHash}
			environment := []string{"MARKER=" + marker, "BOUND_VALUE=saved-owner"}
			switch scenario {
			case "nil environment":
				environment = nil
			case "wrong source":
				prepared.SourceCommit = strings.Repeat("f", 40)
			case "wrong target":
				prepared.TargetCommit = original
			case "wrong branch":
				prepared.Branch = "dev"
			case "wrong manifest":
				prepared.ManifestHash = strings.Repeat("f", 64)
			case "dirty":
				if err := os.WriteFile(filepath.Join(workspace, "pending"), []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := executePreparedNativeRelease(context.Background(), intent, prepared, environment)
			ran := scenario == "valid" || scenario == "failed script"
			if ran {
				data, readErr := os.ReadFile(marker)
				if err != nil || readErr != nil || string(data) != "saved-owner/absent" || result.Commit != target || result.Branch != "production" || result.StartedAt.IsZero() || result.FinishedAt.Before(result.StartedAt) || !result.SourceUnchanged {
					t.Fatal("native process facts or environment incorrect", err, result)
				}
				expectedExit := 0
				if scenario == "failed script" {
					expectedExit = 7
				}
				if result.ExitCode != expectedExit {
					t.Fatal("script exit lost", result.ExitCode)
				}
				raw, _ := json.Marshal(result)
				if strings.Contains(string(raw), "private child output") {
					t.Fatal("raw diagnostic escaped process boundary")
				}
			} else {
				if err == nil || result.ExitCode != -1 {
					t.Fatal("invalid preparation accepted")
				}
				if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
					t.Fatal("rejected source executed")
				}
			}
			if run("symbolic-ref", "--short", "HEAD") != "dev" || run("rev-parse", "HEAD") != target {
				t.Fatal("native executor changed developer source")
			}
		})
	}
}

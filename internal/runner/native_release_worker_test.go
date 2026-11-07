package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestNativeReleaseWorkerExecutesOnceAndDeliversObservedResultWithoutModel(t *testing.T) {
	for _, scenario := range []string{"verified", "unverified", "unknown process", "active process", "changed environment"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			marker := filepath.Join(t.TempDir(), "executions")
			t.Setenv("NATIVE_AMBIENT_CREDENTIAL", "host-secret")
			if err := os.Mkdir(filepath.Join(root, ".cuan"), 0700); err != nil {
				t.Fatal(err)
			}
			manifest := `{"version":1,"command":["release.sh"],"timeout_seconds":5}`
			for name, body := range map[string]string{".cuan/release.json": manifest, "release.sh": "#!/bin/sh\nprintf '%s/%s\\n' \"$BOUND_VALUE\" \"${NATIVE_AMBIENT_CREDENTIAL:-absent}\" >> \"$MARKER\"\n"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("fixture git: %s", out)
				}
				return strings.TrimSpace(string(out))
			}
			run("init", "--initial-branch=production")
			run("add", ".")
			run("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Release")
			commit := run("rev-parse", "HEAD")
			hash := sha256.Sum256([]byte(manifest))
			now := time.Now().UTC()
			claim := ClaimTaskResponse{Task: &task.Task{ID: "task", CreatedAt: now}, Attempt: &task.TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "native", CreatedAt: now, StartedAt: &now}, NativeRelease: &task.NativeReleaseIntent{SourceCommit: commit, TargetBranch: "production", ManifestHash: hex.EncodeToString(hash[:])}, NativeReleaseExecution: &NativeReleaseExecution{Workspace: &project.PreparedNativeReleaseWorkspace{Path: root, SourceCommit: commit, TargetCommit: commit, Branch: "production", ManifestHash: hex.EncodeToString(hash[:])}, Environment: []string{"MARKER=" + marker, "BOUND_VALUE=saved"}}}
			observations, completions := 0, 0
			var completed CompleteAttemptRequest
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/publication/verify") {
					observations++
					data, err := os.ReadFile(marker)
					if err != nil || string(data) != "saved/absent\n" {
						t.Error("observation preceded process or process replayed")
					}
					if scenario == "unverified" {
						w.WriteHeader(422)
						return
					}
					json.NewEncoder(w).Encode(map[string]any{"verified": true, "receipt": task.DeploymentReceipt{Version: 1, TaskID: "task", AttemptID: "attempt", CommitSHA: commit, ReleaseSHA: commit, Environment: "production", Health: "healthy"}})
					return
				}
				if strings.HasSuffix(r.URL.Path, "/complete") {
					completions++
					if err := json.NewDecoder(r.Body).Decode(&completed); err != nil {
						t.Error(err)
					}
					json.NewEncoder(w).Encode(task.TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "native", Status: completed.Status})
					return
				}
				t.Errorf("native worker entered unexpected model route %s", r.URL.Path)
				w.WriteHeader(500)
			}))
			defer api.Close()
			worker := NewRunner(&config.Config{RunnerID: "native", RunnerToken: "fixture", RunnerServerURL: api.URL, RunnerWorkerRegistryDir: t.TempDir()}, nil, api.Client())
			if scenario == "active process" {
				journal, err := worker.openCompletionOutbox()
				if err != nil {
					t.Fatal(err)
				}
				defer journal.Close()
				active, err := lockCompletion(journal, "native-attempt")
				if err != nil {
					t.Fatal(err)
				}
				defer active.Close()
			}
			if scenario == "unknown process" {
				journal, err := worker.openCompletionOutbox()
				if err != nil {
					t.Fatal(err)
				}
				if err := journal.Mkdir("native-process", 0700); err != nil {
					t.Fatal(err)
				}
				if err := writeNativeProcessRecord(journal, "native-process/attempt.json", nativeProcessRecord{Version: 1, Binding: nativeExecutionBinding(claim, "native")}, true); err != nil {
					t.Fatal(err)
				}
				journal.Close()
			}
			worker.runAttempt(ctx, claim)
			if scenario == "active process" {
				if completions != 0 || observations != 0 {
					t.Fatal("duplicate worker completed an active native process")
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("duplicate process executed")
				}
				return
			}
			if completions != 1 || completed.AgentReportRequired || completed.AgentReportConfirmed || completed.CommitSHA != "" || completed.BranchName != "" {
				t.Fatal("native completion entered developer reporting")
			}
			if scenario == "unknown process" {
				if completed.Status != task.AttemptStatusFailed || observations != 0 {
					t.Fatal("unknown process replayed")
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("unknown process reran")
				}
				return
			}
			if observations != 1 || completed.NativeReleaseProcess == nil || completed.NativeReleaseProcess.Commit != commit || completed.NativeReleaseProcess.ExitCode != 0 {
				t.Fatal("native process facts lost")
			}
			if scenario == "unverified" {
				if completed.Status != task.AttemptStatusFailed || completed.LiveDeploymentConfirmed {
					t.Fatal("process exit accepted without live observation")
				}
				return
			}
			if completed.Status != task.AttemptStatusSucceeded || !completed.LiveDeploymentConfirmed {
				t.Fatal("observed native result failed")
			}
			if scenario == "changed environment" {
				claim.NativeReleaseExecution.Environment = append(claim.NativeReleaseExecution.Environment, "NEW_VALUE=changed")
				if _, err := worker.executeNativeReleaseOnce(ctx, claim); err == nil {
					t.Fatal("changed execution replayed")
				}
			} else {
				// Reconstruct the worker against the same private journal, as after restart.
				restarted := NewRunner(worker.cfg, nil, api.Client())
				restarted.runAttempt(ctx, claim)
				if observations != 2 || completions != 1 {
					t.Fatal("saved result was not reconciled without duplicate completion")
				}
			}
			data, _ := os.ReadFile(marker)
			if string(data) != "saved/absent\n" {
				t.Fatal("release executed more than once")
			}
			records, err := os.ReadFile(filepath.Join(worker.completionDirectory(), "native-process", "attempt.json"))
			if err != nil || strings.Contains(string(records), "BOUND_VALUE") || strings.Contains(string(records), "MARKER=") || strings.Contains(string(records), "host-secret") {
				t.Fatal("journal leaked environment")
			}
		})
	}
}

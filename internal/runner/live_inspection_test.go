package runner

import (
	"context"
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
	"github.com/buchin/cuan/internal/executionpolicy"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

func TestLiveInspectionRunsFreshChecksInSnapshotWithoutPublication(t *testing.T) {
	for _, scenario := range []string{"valid", "failed checks", "changed source", "valid recovery", "unavailable previous"} {
		t.Run(scenario, func(t *testing.T) {
			source := documentationFixture(t)
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatal(err, string(out))
				}
				return strings.TrimSpace(string(out))
			}
			original := run("rev-parse", "HEAD")
			exit := "0"
			if scenario == "failed checks" {
				exit = "4"
			}
			if err := os.WriteFile(filepath.Join(source, "scripts", "test.sh"), []byte("#!/bin/sh\nprintf 'LIVE_MACHINE_CHECK\\n'\nexit "+exit+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			run("add", ".")
			run("commit", "-m", "Published result")
			live := run("rev-parse", "HEAD")
			snapshot, err := cuangit.NewClient().PrepareVerificationSnapshot(context.Background(), source, filepath.Join(t.TempDir(), "audit"), original, live)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "pending.txt"), []byte("PRESERVE_ORIGINAL_WORK"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", t.TempDir())
			var completion CompleteAttemptRequest
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/report" {
					w.Write([]byte(`{"summary":"Current result inspected","complete_note":"Current targeted findings recorded","activities":[{"attempt_id":"live-attempt","activity_type":"agent_report","new_status":"review"}]}`))
					return
				}
				if strings.HasSuffix(r.URL.Path, "/complete") {
					if err := json.NewDecoder(r.Body).Decode(&completion); err != nil {
						t.Error(err)
					}
				}
				w.Write([]byte(`{}`))
			}))
			defer api.Close()
			runner := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL, WorkspacesDir: t.TempDir()}, &readOnlyAuditAdapter{mutate: scenario == "changed source", t: t}, api.Client())
			now := time.Now()
			claim := ClaimTaskResponse{Claimed: true, OwnerID: "owner", WorkingDir: snapshot, AutoDeploy: false, PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true, DenyDeploy: true, DenyPush: true}, Task: &task.Task{ID: "live", Title: "Inspect current implementation", Instructions: "Approved result inspection", SourceRoomID: "room", ExecutionCategory: task.ExecutionCategoryCode}, Attempt: &task.TaskAttempt{ID: "live-attempt", TaskID: "live", CreatedAt: now}, LiveInspection: &task.LiveInspectionIntent{SourceCommit: original, TargetCommit: live}, TaskAgent: &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"}}
			if scenario == "valid recovery" {
				claim.LiveInspection.PreviousCommit = original
			}
			if scenario == "unavailable previous" {
				claim.LiveInspection.PreviousCommit = strings.Repeat("f", 40)
			}
			runner.runAttempt(context.Background(), claim)
			if (completion.Status == task.AttemptStatusSucceeded) != (scenario == "valid" || scenario == "valid recovery") {
				t.Fatal("snapshot verification outcome incorrect", completion.Status, completion.ErrorMessage)
			}
			if scenario != "changed source" && scenario != "unavailable previous" && !strings.Contains(completion.TestSummary, "LIVE_MACHINE_CHECK") {
				t.Fatal("native runner skipped fresh fixed project checks")
			}
			if scenario == "valid" && (!completion.SourceAncestryVerified || completion.LiveInspectionCommit != live || !completion.RepositoryVerification.Unchanged() || !completion.AgentReportConfirmed) {
				t.Fatal("missing source/target/scope/report evidence")
			}
			if completion.CommitSHA != "" || completion.PRURL != "" || completion.LiveDeploymentConfirmed {
				t.Fatal("read-only inspection fabricated publication")
			}
			if data, err := os.ReadFile(filepath.Join(source, "pending.txt")); err != nil || string(data) != "PRESERVE_ORIGINAL_WORK" {
				t.Fatal("inspection modified old work", err)
			}
		})
	}
}

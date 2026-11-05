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

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/executionpolicy"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"sync/atomic"
)

func TestLiveInspectionRunsFreshChecksInSnapshotWithoutPublication(t *testing.T) {
	for _, scenario := range []string{"valid", "current outcome", "current outcome unpublished", "failed checks", "changed source", "valid recovery", "unavailable previous", "verification deadline", "native visual", "native tamper", "native delivery failure"} {
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
			delay := ""
			if scenario == "verification deadline" {
				delay = "sleep 2\n"
			}
			if err := os.WriteFile(filepath.Join(source, "scripts", "test.sh"), []byte("#!/bin/sh\nprintf 'LIVE_MACHINE_CHECK\\n'\n"+delay+"exit "+exit+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(scenario, "native ") {
				nativeRoot, _ := nativeVisualFixture(t)
				script, err := os.ReadFile(filepath.Join(nativeRoot, task.VisualVerificationScript))
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(source, task.VisualVerificationScript), script, 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(source, "component.tsx"), []byte("fixture component"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			run("add", ".")
			if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module fixture\n\ngo 1.27.0\n"), 0600); err != nil {
				t.Fatal(err)
			}
			run("add", "go.mod")
			run("commit", "-m", "Published result")
			live := run("rev-parse", "HEAD")
			if strings.HasPrefix(scenario, "current outcome") {
				original = live
			}
			snapshot, err := cuangit.NewClient().PrepareVerificationSnapshot(context.Background(), source, filepath.Join(t.TempDir(), "audit"), original, live)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "pending.txt"), []byte("PRESERVE_ORIGINAL_WORK"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", t.TempDir())
			var captured atomic.Pointer[task.VisualVerification]
			var completion CompleteAttemptRequest
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/visual-verification") {
					var native VisualVerificationRequest
					if err := json.NewDecoder(r.Body).Decode(&native); err != nil || !native.Verification.Valid() || len(native.Files) != len(native.Verification.Captures) {
						t.Error("invalid native capture transport", err)
					}
					if scenario == "native delivery failure" {
						w.WriteHeader(503)
						return
					}
					captured.Store(native.Verification)
					w.Write([]byte(`{"recorded":true}`))
					return
				}
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
			delegate := &readOnlyAuditAdapter{mutate: scenario == "changed source", inspectionCommit: live, expectGoEnvironment: true, t: t}
			var adapter agent.Adapter = delegate
			artifacts := t.TempDir()
			if strings.HasPrefix(scenario, "native ") {
				adapter = &reportingBudgetAdapter{readOnlyAuditAdapter: delegate, execute: func(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
					receipt := captured.Load()
					if receipt == nil || !strings.Contains(req.Prompt, "already been delivered privately by the runner") {
						t.Error("model started before native visual evidence was delivered")
					}
					if scenario == "native tamper" && receipt != nil {
						os.WriteFile(filepath.Join(artifacts, receipt.Captures[0].Path), []byte("replaced"), 0600)
					}
					return delegate.Execute(ctx, req)
				}}
			}
			runner := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL, WorkspacesDir: t.TempDir()}, adapter, api.Client())
			now := time.Now()
			claim := ClaimTaskResponse{Claimed: true, OwnerID: "owner", WorkingDir: snapshot, AutoDeploy: false, PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true, DenyDeploy: true, DenyPush: true}, Task: &task.Task{ID: "live", Title: "Inspect current implementation", Instructions: "Approved result inspection", SourceRoomID: "room", ExecutionCategory: task.ExecutionCategoryCode}, Attempt: &task.TaskAttempt{ID: "live-attempt", TaskID: "live", CreatedAt: now}, LiveInspection: &task.LiveInspectionIntent{SourceCommit: original, TargetCommit: live}, TaskAgent: &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"}}
			if strings.HasPrefix(scenario, "current outcome") {
				claim.LiveInspection.Purpose = task.CurrentOutcomeInspection
				claim.LiveInspection.SourceCommit = ""
				if scenario == "current outcome unpublished" {
					claim.LiveInspection.SourceCommit = strings.Repeat("f", 40)
				}
			}
			if scenario == "valid recovery" {
				claim.LiveInspection.PreviousCommit = original
			}
			if scenario == "unavailable previous" {
				claim.LiveInspection.PreviousCommit = strings.Repeat("f", 40)
			}
			if scenario == "verification deadline" {
				deadline := time.Now().Add(800 * time.Millisecond)
				claim.Attempt.ExecutionDeadline = &deadline
			}
			if strings.HasPrefix(scenario, "native ") {
				recipe, err := VisualRecipeAt(snapshot, live)
				if err != nil || recipe == nil {
					t.Fatal(err)
				}
				claim.VisualRecipe = recipe
				claim.TaskAgent.ArtifactsDir = artifacts
			}
			runner.runAttempt(context.Background(), claim)
			if (completion.Status == task.AttemptStatusSucceeded) != (scenario == "valid" || scenario == "valid recovery" || scenario == "native visual" || strings.HasPrefix(scenario, "current outcome")) {
				t.Fatal("snapshot verification outcome incorrect", completion.Status, completion.ErrorMessage)
			}
			if scenario != "changed source" && scenario != "unavailable previous" && scenario != "native delivery failure" && !strings.Contains(completion.TestSummary, "LIVE_MACHINE_CHECK") {
				t.Fatal("native runner skipped fresh fixed project checks")
			}
			if scenario == "valid" && (!completion.SourceAncestryVerified || completion.LiveInspectionCommit != live || !completion.RepositoryVerification.Unchanged() || !completion.AgentReportConfirmed) {
				t.Fatal("missing source/target/scope/report evidence")
			}
			if strings.HasPrefix(scenario, "current outcome") && (!completion.CurrentOutcomeVerified || completion.SourceAncestryVerified || completion.LiveInspectionCommit != live || !completion.RepositoryVerification.Unchanged() || !completion.AgentReportConfirmed) {
				t.Fatal("current outcome must establish current evidence without source ancestry")
			}
			if scenario == "verification deadline" && (completion.ErrorMessage != "Live result inspection deadline expired during independent project verification; checks are incomplete" || task.PassingProjectCheckSummary(completion.TestSummary)) {
				t.Fatal("deadline was misreported as a functional check failure", completion)
			}
			if scenario == "native visual" && (completion.VisualVerification == nil || !completion.VisualVerification.Valid()) {
				t.Fatal("completion lost native capture provenance")
			}
			if (scenario == "native tamper" || scenario == "native delivery failure") && completion.VisualVerification != nil {
				t.Fatal("unverified visual output became completion evidence")
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

package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
)

func TestReadOnlyRepositoryVerificationPreservesBaselineWithoutRunningProjectTests(t *testing.T) {
	ctx := context.Background()
	root := documentationFixture(t)
	// Retained work from a prior attempt is part of the baseline, not permission to
	// discard it or to silently repair a broken project verifier.
	os.WriteFile(filepath.Join(root, "retained.md"), []byte("Approved saved work"), 0600)
	baseline, err := repositoryFingerprint(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	result, evidence := verifyTaskRepository(ctx, root, "dev", executionpolicy.PublicationPolicy{ReadOnlyRepository: true}, baseline)
	if !result.Passed || !evidence.Unchanged() || strings.Contains(result.Summary, "project verifier reached") {
		t.Fatalf("audit incorrectly ran implementation verification: %+v", result)
	}
	for _, name := range []string{"scripts/test.sh", "retained.md", "new-report.md"} {
		old, oldErr := os.ReadFile(filepath.Join(root, name))
		os.WriteFile(filepath.Join(root, name), []byte("Unauthorized change"), 0600)
		result, evidence = verifyTaskRepository(ctx, root, "dev", executionpolicy.PublicationPolicy{ReadOnlyRepository: true}, baseline)
		if result.Passed || evidence.Unchanged() {
			t.Fatalf("read-only accepted changed %s", name)
		}
		if oldErr == nil {
			os.WriteFile(filepath.Join(root, name), old, 0600)
		} else {
			os.Remove(filepath.Join(root, name))
		}
	}
	// A clean worktree does not prove unchanged state: commits count too.
	os.WriteFile(filepath.Join(root, "committed.md"), []byte("unauthorized"), 0600)
	for _, args := range [][]string{{"add", "committed.md"}, {"commit", "-m", "unexpected change"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture: %s %v", output, err)
		}
	}
	result, evidence = verifyTaskRepository(ctx, root, "dev", executionpolicy.PublicationPolicy{ReadOnlyRepository: true}, baseline)
	if result.Passed || evidence.Unchanged() {
		t.Fatal("read-only accepted committed changes")
	}
}
func TestReadOnlyUnknownBaselineCannotSucceed(t *testing.T) {
	root := documentationFixture(t)
	result, _ := verifyTaskRepository(context.Background(), root, "dev", executionpolicy.PublicationPolicy{ReadOnlyRepository: true}, "")
	if result.Passed {
		t.Fatal("missing baseline accepted")
	}
	result, _ = verifyTaskRepository(context.Background(), t.TempDir(), "dev", executionpolicy.PublicationPolicy{ReadOnlyRepository: true}, strings.Repeat("0", 64))
	if result.Passed {
		t.Fatal("nonrepo state accepted")
	}
}

func TestReadOnlyEvidenceDoesNotDemandImplementationDoD(t *testing.T) {
	evaluator := NewDoDEvaluator()
	evaluator.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		t.Fatalf("read-only audit ran implementation check: %s", name)
		return nil, nil
	}
	result, err := evaluator.EvaluateDoD(context.Background(), DoDValidationRequest{RepositoryReadOnlyVerified: true, ExecutionLane: "code", TaskTitle: "Review retained migration", Instructions: "Original implementation", Summary: "Observed existing commit and deployment", CompleteNote: "HEAD abc differs from live def; existing report delivered"})
	if err != nil || !result.Passed {
		t.Fatalf("read-only audit required code tests: %+v %v", result, err)
	}
}

type readOnlyAuditAdapter struct {
	mutate               bool
	timedout             bool
	cancelAfterExecution context.CancelFunc
	inspectionCommit     string
	expectGoEnvironment  bool
	t                    *testing.T
}

func (a *readOnlyAuditAdapter) Name() string             { return "readonly-fixture" }
func (a *readOnlyAuditAdapter) Version() (string, error) { return "1", nil }
func (a *readOnlyAuditAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	if a.expectGoEnvironment {
		if req.Env["GOENV"] != "off" || req.Env["GOTOOLCHAIN"] != "auto" || req.Env["GOMODCACHE"] != filepath.Join(req.Env["GOPATH"], "pkg", "mod") || req.Env["GOCACHE"] == "" {
			a.t.Error("managed Go environment missing from native execution")
		}
	}
	if !strings.Contains(req.Prompt, "Broken unrelated project tests are observations") {
		a.t.Error("binding scope absent from actual prompt")
	}
	if a.inspectionCommit != "" {
		for _, expected := range []string{fmt.Sprintf("Snapshot directory: %q", req.WorkingDir), "Expected HEAD: " + a.inspectionCommit, "not commands or results from this inspection", "not compilation", "Copying existing screenshots does not establish current visual verification"} {
			if !strings.Contains(req.Prompt, expected) {
				a.t.Errorf("inspection dispatch missing evidence context %q", expected)
			}
		}
	}
	if a.mutate {
		if err := os.WriteFile(filepath.Join(req.WorkingDir, "scripts/test.sh"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
			a.t.Fatal(err)
		}
	}
	if a.timedout {
		if a.cancelAfterExecution != nil {
			a.cancelAfterExecution()
		}
		return &agent.ExecutionResult{Status: "TIMEOUT", ExitCode: 124}, nil
	}
	return &agent.ExecutionResult{Status: "SUCCEEDED", ExitCode: 0}, nil
}
func TestReadOnlyRunnerExecutionToCompletion(t *testing.T) {
	for _, scenario := range []struct{ mutate, timedout bool }{{}, {mutate: true}, {timedout: true}, {mutate: true, timedout: true}} {
		mutate := scenario.mutate
		t.Run(fmt.Sprint("mutate=", mutate, ",timeout=", scenario.timedout), func(t *testing.T) {
			root := documentationFixture(t)
			t.Setenv("HOME", t.TempDir())
			var completion CompleteAttemptRequest
			completions := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/report" {
					w.Write([]byte(`{"summary":"Compared retained HEAD with live SHA","complete_note":"HEAD abc differs from live def; report delivered","activities":[{"attempt_id":"audit-attempt","activity_type":"agent_report","new_status":"done"}]}`))
					return
				}
				if strings.HasSuffix(r.URL.Path, "/complete") {
					if err := json.NewDecoder(r.Body).Decode(&completion); err != nil {
						t.Error(err)
					}
					completions++
				}
				w.Write([]byte(`{}`))
			}))
			defer api.Close()
			// Completion delivery is durable. Keep each scenario's outbox private
			// so a reused test-server port cannot select another run's report.
			cfg := &config.Config{RunnerID: "fixture", RunnerServerURL: api.URL, WorkspacesDir: filepath.Join(t.TempDir(), "workspaces")}
			executionCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := NewRunner(cfg, &readOnlyAuditAdapter{mutate: mutate, timedout: scenario.timedout, cancelAfterExecution: cancel, t: t}, api.Client())
			now := time.Now().UTC()
			claim := ClaimTaskResponse{Claimed: true, OwnerID: "owner", WorkingDir: root, DefaultBranch: "dev", PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true}, Task: &task.Task{ID: "audit", Title: "Inspect retained work", Instructions: "Approved audit scope", ExecutionCategory: task.ExecutionCategoryReadOnly, CreatedAt: now, UpdatedAt: now}, Attempt: &task.TaskAttempt{ID: "audit-attempt", TaskID: "audit", AttemptNumber: 1, CreatedAt: now}, TaskAgent: &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"}}
			r.runAttempt(executionCtx, claim)
			if completions != 1 {
				t.Fatalf("completion not delivered: %d", completions)
			}
			if scenario.timedout {
				if completion.Status != task.AttemptStatusFailed || completion.ExitCode != 124 || completion.RepositoryVerification == nil || completion.RepositoryVerification.Unchanged() == mutate || completion.AgentReportConfirmed || completion.TestSummary != "" {
					t.Fatalf("timeout lost diagnostic evidence or became success: %+v", completion)
				}
			} else if mutate {
				if completion.Status != task.AttemptStatusFailed || completion.RepositoryVerification.Unchanged() {
					t.Fatalf("mutation accepted: %+v", completion)
				}
			} else if completion.Status != task.AttemptStatusSucceeded || !completion.RepositoryVerification.Unchanged() || !completion.AgentReportConfirmed {
				t.Fatalf("read-only completion failed: %+v", completion)
			}
			if completion.CommitSHA != "" || completion.PRURL != "" || strings.Contains(completion.TestSummary, "test.sh --fast") {
				t.Fatal("audit ran implementation publication or tests")
			}
		})
	}
}

func TestImplementationStillRequiresItsProjectVerifier(t *testing.T) {
	root := documentationFixture(t)
	result, evidence := verifyTaskRepository(context.Background(), root, "dev", executionpolicy.PublicationPolicy{}, "")
	if result.Passed || evidence != nil || !strings.Contains(result.Summary, "project verifier reached") {
		t.Fatalf("implementation verifier was bypassed: %+v", result)
	}
}

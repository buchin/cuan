package runner

import (
	"context"
	"encoding/json"
	"fmt"
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
)

type reportingBudgetAdapter struct {
	*readOnlyAuditAdapter
	execute func(context.Context, agent.ExecutionRequest) (*agent.ExecutionResult, error)
}

func (a *reportingBudgetAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	return a.execute(ctx, req)
}

func TestInspectionReportingLeavesTimeForNativeChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated native process/deadline contract; run without -short")
	}
	for _, scenario := range []string{"late report", "reporting timeout", "reporting cancellation", "reporting error", "reporting failed with saved report", "owner cancellation"} {
		t.Run(scenario, func(t *testing.T) {
			root := documentationFixture(t)
			marker := filepath.Join(t.TempDir(), "native-check-ran")
			script := fmt.Sprintf("#!/bin/sh\nsleep 0.2\nprintf LIVE_NATIVE_CHECK\nprintf ran > %q\n", marker)
			if err := os.WriteFile(filepath.Join(root, "scripts/test.sh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"add", "."}, {"commit", "-m", "Native verifier"}} {
				if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
					t.Fatal(err, string(out))
				}
			}
			raw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
			if err != nil {
				t.Fatal(err)
			}
			commit := strings.TrimSpace(string(raw))
			snapshot, err := cuangit.NewClient().PrepareVerificationSnapshot(context.Background(), root, filepath.Join(t.TempDir(), "snapshot"), commit, commit)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", t.TempDir())
			var completion CompleteAttemptRequest
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/report" {
					w.Write([]byte(`{"summary":"Current inspection findings","complete_note":"Original scope checked","activities":[{"attempt_id":"budget-attempt","activity_type":"agent_report","new_status":"review"}]}`))
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
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			overall := time.Now().Add(3 * time.Second)
			delegate := &readOnlyAuditAdapter{t: t, inspectionCommit: commit}
			adapter := &reportingBudgetAdapter{readOnlyAuditAdapter: delegate}
			adapter.execute = func(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
				phase, ok := ctx.Deadline()
				if !ok || !phase.Before(overall) || overall.Sub(phase) < 1200*time.Millisecond {
					t.Fatal("native verification time was not protected")
				}
				if !strings.Contains(req.Prompt, "Inspection reporting deadline:") || !strings.Contains(req.Prompt, overall.UTC().Format(time.RFC3339Nano)) {
					t.Error("agent was not told its actual reporting/overall deadlines")
				}
				if req.Timeout >= time.Until(overall) {
					t.Error("adapter timeout extends into native verification window")
				}
				if d, err := time.ParseDuration(req.Env["CUAN_TASK_TIMEOUT"]); err != nil || d != req.Timeout {
					t.Error("runtime timeout environment disagrees with adapter budget")
				}
				if scenario == "owner cancellation" {
					cancel()
					return &agent.ExecutionResult{Status: "CANCELED", ExitCode: 130}, nil
				}
				if strings.HasPrefix(scenario, "reporting ") {
					<-ctx.Done()
					if scenario == "reporting error" {
						return nil, ctx.Err()
					}
					if scenario == "reporting failed with saved report" {
						return &agent.ExecutionResult{Status: "FAILED", ExitCode: 1}, nil
					}
					if scenario == "reporting cancellation" {
						return &agent.ExecutionResult{Status: "CANCELED", ExitCode: 130}, nil
					}
					return &agent.ExecutionResult{Status: "TIMEOUT", ExitCode: 124}, nil
				}
				timer := time.NewTimer(time.Until(phase.Add(-200 * time.Millisecond)))
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-ctx.Done():
					t.Fatal("reporting window unexpectedly canceled")
				}
				return delegate.Execute(ctx, req)
			}
			r := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL, WorkspacesDir: t.TempDir()}, adapter, api.Client())
			attempt := &task.TaskAttempt{ID: "budget-attempt", TaskID: "budget-task", CreatedAt: time.Now(), ExecutionDeadline: &overall}
			claim := ClaimTaskResponse{Claimed: true, OwnerID: "owner", WorkingDir: snapshot, PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true, DenyDeploy: true, DenyPush: true}, Task: &task.Task{ID: attempt.TaskID, Title: "Inspect existing result", SourceRoomID: "room", ExecutionCategory: task.ExecutionCategoryCode}, Attempt: attempt, LiveInspection: &task.LiveInspectionIntent{Purpose: task.CurrentOutcomeInspection, TargetCommit: commit}, TaskAgent: &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"}}
			r.runAttempt(parent, claim)
			if !attempt.ExecutionDeadline.Equal(overall) {
				t.Fatal("attempt deadline was changed")
			}
			_, markerErr := os.Stat(marker)
			switch scenario {
			case "late report":
				if completion.Status != task.AttemptStatusSucceeded || markerErr != nil || !task.PassingProjectCheckSummary(completion.TestSummary) {
					t.Fatal("native checks lost their reserved window", completion.Status, completion.ErrorMessage, markerErr)
				}
			case "reporting timeout", "reporting cancellation", "reporting error", "reporting failed with saved report":
				if completion.Status != task.AttemptStatusFailed || completion.ExitCode != 124 || completion.ErrorMessage != inspectionReportingTimeout || !os.IsNotExist(markerErr) {
					t.Fatal("reporting timeout became cancellation/acceptance", completion)
				}
			case "owner cancellation":
				if completion.Status != task.AttemptStatusCanceled || !os.IsNotExist(markerErr) {
					t.Fatal("owner cancellation was turned into reporting timeout", completion)
				}
			}
		})
	}
}

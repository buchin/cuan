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

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

type failedCheckSourceAdapter struct {
	t      *testing.T
	commit string
	path   string
}

func (a *failedCheckSourceAdapter) Name() string             { return "source-fixture" }
func (a *failedCheckSourceAdapter) Version() (string, error) { return "1", nil }
func (a *failedCheckSourceAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	if err := os.WriteFile(filepath.Join(req.WorkingDir, "feature.go"), []byte("package feature\n"), 0600); err != nil {
		a.t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "feature.go"}, {"-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Observed checkpoint"}} {
		if out, err := exec.CommandContext(ctx, "git", append([]string{"-C", req.WorkingDir}, args...)...).CombinedOutput(); err != nil {
			a.t.Fatalf("checkpoint: %v %s", err, out)
		}
	}
	out, err := exec.CommandContext(ctx, "git", "-C", req.WorkingDir, "rev-parse", "HEAD").Output()
	if err != nil {
		a.t.Fatal(err)
	}
	a.commit = strings.TrimSpace(string(out))
	a.path = req.WorkingDir
	return &agent.ExecutionResult{Status: "SUCCEEDED", ExitCode: 0}, nil
}

func TestFailedNativeChecksRetainActualDeveloperCheckpoint(t *testing.T) {
	root := documentationFixture(t)
	// Native provider setup writes its private local configuration. The fixture
	// ignores it just as a registered application does; source remains tracked.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".agents/\n.gemini/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"checkout", "dev"}, {"add", ".gitignore"}, {"commit", "-m", "Ignore native provider setup"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v %s", err, out)
		}
	}
	t.Setenv("HOME", t.TempDir())
	var completion CompleteAttemptRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/report" {
			w.Write([]byte(`{"summary":"Checkpoint saved","complete_note":"Source checkpoint prepared; native checks pending","activities":[{"attempt_id":"source-attempt","activity_type":"agent_report","new_status":"done"}]}`))
			return
		}
		if strings.HasSuffix(req.URL.Path, "/complete") {
			if err := json.NewDecoder(req.Body).Decode(&completion); err != nil {
				t.Error(err)
			}
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()
	a := &failedCheckSourceAdapter{t: t}
	r := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL, WorkspacesDir: filepath.Join(t.TempDir(), "workspaces")}, a, api.Client())
	r.runAttempt(context.Background(), ClaimTaskResponse{Claimed: true, OwnerID: "owner", WorkingDir: root, DefaultBranch: "dev", AutoDeploy: true, Task: &task.Task{ID: "source", ProjectID: "project", BranchName: "source-checkpoint", Title: "Implement approved feature", Instructions: "Implement the approved source correction", ExecutionCategory: task.ExecutionCategoryCode}, Attempt: &task.TaskAttempt{ID: "source-attempt", TaskID: "source", AttemptNumber: 1}, TaskAgent: &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"}})
	if completion.Status != task.AttemptStatusFailed || completion.ErrorMessage != "Project verification failed" || !strings.Contains(completion.TestSummary, "project verifier reached") {
		t.Fatalf("native failure lost: %+v", completion)
	}
	if a.commit == "" || completion.CommitSHA != a.commit || !strings.Contains(completion.DiffSummary, "feature.go") {
		state, _ := exec.Command("git", "-C", a.path, "status", "--porcelain").CombinedOutput()
		t.Fatalf("review retained stale source identity: %+v state=%s", completion, state)
	}
	if completion.LiveDeploymentConfirmed || completion.ManagedReleaseSource != nil || completion.RequiresDeploy != nil {
		t.Fatal("failed checkpoint became publication evidence")
	}
}

func TestLocalImplementationSourceRecordsCheckpointWithoutPublication(t *testing.T) {
	for _, scenario := range []string{"clean", "dirty", "untracked", "unchanged", "missing baseline"} {
		t.Run(scenario, func(t *testing.T) {
			root := documentationFixture(t)
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatal(err, string(out))
				}
				return strings.TrimSpace(string(out))
			}
			base := run("rev-parse", "HEAD")
			if scenario != "unchanged" {
				if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package feature\n"), 0600); err != nil {
					t.Fatal(err)
				}
				run("add", "feature.go")
				run("commit", "-m", "Approved checkpoint")
			}
			if scenario == "dirty" || scenario == "untracked" {
				name := "feature.go"
				if scenario == "untracked" {
					name = "pending.txt"
				}
				if err := os.WriteFile(filepath.Join(root, name), []byte("preserve pending work\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "missing baseline" {
				base = strings.Repeat("f", 40)
			}
			head, status, refs := run("rev-parse", "HEAD"), run("status", "--porcelain"), run("show-ref")
			result := CompleteAttemptRequest{}
			runner := &Runner{gitClient: cuangit.NewClient()}
			err := runner.observeLocalImplementationSource(context.Background(), &cuangit.TaskWorkspace{Path: root, BaseCommit: base}, "dev", &result)
			if (err != nil) != (scenario == "missing baseline") {
				t.Fatal("comparison failure was hidden", err)
			}
			if scenario == "clean" {
				if result.CommitSHA != head || result.BranchName != run("rev-parse", "--abbrev-ref", "HEAD") || !strings.Contains(result.DiffSummary, "feature.go") || result.HasCodeChanges == nil || !*result.HasCodeChanges {
					t.Fatal("local implementation source not recorded", result)
				}
			} else if result.CommitSHA != "" || result.DiffSummary != "" || result.HasCodeChanges != nil {
				t.Fatal("dirty, absent or unverifiable work attributed to HEAD", result)
			}
			if result.LiveDeploymentConfirmed || result.ManagedReleaseSource != nil || result.RequiresDeploy != nil || result.PRURL != "" || result.PRNumber != 0 {
				t.Fatal("source observation manufactured publication authority")
			}
			if head != run("rev-parse", "HEAD") || status != run("status", "--porcelain") || refs != run("show-ref") {
				t.Fatal("source observation modified local work or refs")
			}
		})
	}
}

type blockedReportAdapter struct{}

func (a *blockedReportAdapter) Name() string             { return "blocked-fixture" }
func (a *blockedReportAdapter) Version() (string, error) { return "1", nil }
func (a *blockedReportAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	return &agent.ExecutionResult{Status: "BLOCKED", ExitCode: 0, Error: "Agent reported a blocker; saved findings require review."}, nil
}

func TestRunAttemptPreservesConfirmedBlockedReport(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var completion CompleteAttemptRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/report" {
			w.Write([]byte(`{"summary":"Blocked on missing credentials","complete_note":"Missing VPS_IP","activities":[{"attempt_id":"blocked-attempt","activity_type":"agent_report","new_status":"blocked"}]}`))
			return
		}
		if strings.HasSuffix(req.URL.Path, "/complete") {
			if err := json.NewDecoder(req.Body).Decode(&completion); err != nil {
				t.Error(err)
			}
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()

	r := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL, WorkspacesDir: filepath.Join(t.TempDir(), "workspaces")}, &blockedReportAdapter{}, api.Client())
	r.runAttempt(context.Background(), ClaimTaskResponse{
		Claimed:    true,
		OwnerID:    "owner",
		WorkingDir: t.TempDir(),
		Task: &task.Task{
			ID:                "blocked-task",
			ProjectID:         "project",
			Title:             "Blocked investigation",
			Instructions:      "Investigate upstream issue",
			ExecutionCategory: task.ExecutionCategoryOps,
		},
		Attempt:   &task.TaskAttempt{ID: "blocked-attempt", TaskID: "blocked-task", AttemptNumber: 1},
		TaskAgent: &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"},
	})
	if completion.Status != task.AttemptStatusFailed || completion.ExitCode != 0 || !completion.AgentReportConfirmed {
		t.Fatalf("expected confirmed failed/blocked completion, got %+v", completion)
	}
	if completion.ErrorMessage != "Agent reported a blocker; saved findings require review." {
		t.Fatalf("unexpected error message: %q", completion.ErrorMessage)
	}
}

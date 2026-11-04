package runner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestExecutionBaselineDoesNotFollowMovedTrackingRef(t *testing.T) {
	root := documentationFixture(t)
	command := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	pinned := command("rev-parse", "dev")
	ref := "refs/remotes/cuan-base/dev"
	command("update-ref", ref, pinned)
	ws := &cuangit.TaskWorkspace{Path: root, BaseRef: ref, BaseCommit: pinned}
	if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package feature\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command("add", "feature.go")
	command("commit", "-m", "Approved implementation")
	implementation := command("rev-parse", "HEAD")
	// A fetch/ref update after execution started cannot erase the observed code
	// delta or make the implementation appear to have been in the original base.
	command("update-ref", ref, implementation)
	runner := &Runner{gitClient: cuangit.NewClient()}
	legacy := *ws
	legacy.BaseCommit = ""
	if changed, _, err := runner.detectRepoCodeChanges(context.Background(), &legacy, "dev"); err != nil || changed {
		t.Fatal("fixture did not reproduce the mutable-ref empty diff")
	}
	changed, diff, err := runner.detectRepoCodeChanges(context.Background(), ws, "dev")
	if err != nil || !changed || !strings.Contains(diff, "feature.go") {
		t.Fatalf("moved ref hid implementation changes: changed=%t diff=%q", changed, diff)
	}
	if actual := workspaceComparisonBase(ws, "dev"); actual != pinned {
		t.Fatal("comparison moved to a different baseline", actual)
	}
}
func TestExecutionBaselineRetainsLegacyFallbacks(t *testing.T) {
	for _, sample := range []struct {
		workspace *cuangit.TaskWorkspace
		want      string
	}{
		{nil, "dev"},
		{&cuangit.TaskWorkspace{}, "dev"},
		{&cuangit.TaskWorkspace{BaseRef: "refs/remotes/cuan-base/dev"}, "refs/remotes/cuan-base/dev"},
	} {
		if got := workspaceComparisonBase(sample.workspace, "dev"); got != sample.want {
			t.Fatalf("legacy baseline changed: got %q want %q", got, sample.want)
		}
	}
}

func TestPinnedCommitSupportsDocumentationVerification(t *testing.T) {
	root := documentationFixture(t)
	cmd := exec.Command("git", "rev-parse", "dev")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("Approved guide."), 0600); err != nil {
		t.Fatal(err)
	}
	result := runProjectVerification(context.Background(), root, strings.TrimSpace(string(raw)))
	if !result.Passed || !strings.Contains(result.Summary, "Markdown-only diff verified") {
		t.Fatalf("pinned commit lost documentation-only verification: %+v", result)
	}
}

func TestExecutionBaselineUnknownComparisonIsNotUnchanged(t *testing.T) {
	root := documentationFixture(t)
	runner := &Runner{gitClient: cuangit.NewClient()}
	ws := &cuangit.TaskWorkspace{Path: root, BaseCommit: strings.Repeat("0", 40)}
	if _, _, err := runner.detectRepoCodeChanges(context.Background(), ws, "dev"); err == nil {
		t.Fatal("unknown baseline was accepted as an unchanged repository")
	}
}

func TestDeploymentChangeDetectionSeparatesVerifiedDocumentation(t *testing.T) {
	for _, scenario := range []string{"untracked guide", "committed guide", "mixed code", "invalid guide", "deleted guide", "executable guide"} {
		t.Run(scenario, func(t *testing.T) {
			root := documentationFixture(t)
			command := func(args ...string) string {
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatal(err, string(out))
				}
				return strings.TrimSpace(string(out))
			}
			baseline := command("rev-parse", "HEAD")
			guide := filepath.Join(root, "investigation.md")
			if err := os.WriteFile(guide, []byte("Investigation findings and reproducible checks.\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "committed guide" || scenario == "deleted guide" {
				command("add", ".")
				command("commit", "-m", "Investigation report")
			}
			if scenario == "deleted guide" {
				baseline = command("rev-parse", "HEAD")
				if err := os.Remove(guide); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "mixed code" {
				if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "invalid guide" {
				if err := os.WriteFile(guide, []byte{0}, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "executable guide" {
				if err := os.Chmod(guide, 0700); err != nil {
					t.Fatal(err)
				}
			}
			runner := &Runner{gitClient: cuangit.NewClient()}
			changed, _, err := runner.detectRepoCodeChanges(context.Background(), &cuangit.TaskWorkspace{Path: root, BaseCommit: baseline}, "dev")
			wantCode := scenario == "mixed code" || scenario == "invalid guide" || scenario == "executable guide"
			if err != nil || changed != wantCode {
				t.Fatalf("runtime change=%t want=%t error=%v", changed, wantCode, err)
			}
			store := task.NewMemoryStore()
			item := &task.Task{ID: "investigation", ProjectID: "project", Title: "Investigate the issue", ExecutionCategory: task.ExecutionCategoryCode, Status: task.TaskStatusDoing}
			if err := store.CreateTask(context.Background(), item); err != nil {
				t.Fatal(err)
			}
			attempt := &task.TaskAttempt{ID: "attempt", TaskID: item.ID, RunnerID: "runner", Status: task.AttemptStatusRunning}
			if err := store.CreateAttempt(context.Background(), attempt); err != nil {
				t.Fatal(err)
			}
			finder := &mockProjectFinder{projects: map[string]*project.ProjectResponse{"project": {ID: "project", Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`)}}}
			service := NewService(NewMemoryStore(store), store, finder)
			completed, err := service.Complete(context.Background(), attempt.ID, CompleteAttemptRequest{RunnerID: "runner", Status: task.AttemptStatusSucceeded, AgentReportRequired: true, AgentReportConfirmed: true, HasCodeChanges: &changed, RequiresDeploy: &changed})
			if err != nil || (completed.Status == task.AttemptStatusSucceeded) == wantCode {
				t.Fatalf("project auto-deploy confused documentation with runtime code: %+v %v", completed, err)
			}
			stored, err := store.GetTask(context.Background(), item.ID)
			if err != nil || stored.Status == task.TaskStatusDone {
				t.Fatal("deployment classification bypassed independent task acceptance", err)
			}
		})
	}
}

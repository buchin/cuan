package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

type refreshingProjectFinder struct {
	*mockProjectFinder
	calls int
	fail  bool
}

func (f *refreshingProjectFinder) RefreshExecutionBase(_ context.Context, p *project.ProjectResponse) error {
	f.calls++
	if p.ID != "project" || p.DefaultBranch != "dev" || p.RepoURL != "https://github.com/owner/repo" {
		return errors.New("wrong saved baseline identity")
	}
	if f.fail {
		return errors.New("PRIVATE_CONNECTION_FAILURE")
	}
	return nil
}

func TestDeveloperClaimRefreshesBaselineAndStopsBeforeProviderOnFailure(t *testing.T) {
	for _, scenario := range []string{"developer", "failed refresh", "read only", "ops", "no repository"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			p := &project.ProjectResponse{ID: "project", DefaultBranch: "dev", RepoURL: "https://github.com/owner/repo", LocalPath: t.TempDir()}
			finder := &refreshingProjectFinder{mockProjectFinder: &mockProjectFinder{projects: map[string]*project.ProjectResponse{p.ID: p}}, fail: scenario == "failed refresh"}
			store := task.NewMemoryStore()
			tasks := task.NewService(store, nil)
			instructions, lane := "Implement the approved source change", task.ExecutionCategoryCode
			if scenario == "read only" {
				instructions = "Scope: read-only repository. Do not push. Do not deploy."
			}
			if scenario == "ops" {
				lane = task.ExecutionCategoryOps
			}
			if scenario == "no repository" {
				p.RepoURL = ""
			}
			item, err := tasks.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Approved work", ProjectID: p.ID, Instructions: instructions, ExecutionCategory: lane})
			if err != nil {
				t.Fatal(err)
			}
			attempt, err := tasks.CreateAttempt(ctx, item.ID, "owner", task.CreateAttemptRequest{RunnerID: "runner", AgentName: "agent"})
			if err != nil {
				t.Fatal(err)
			}
			svc := NewService(NewMemoryStore(store), store, finder)
			svc.SetEventBus(NewEventBus())
			claim, err := svc.Claim(ctx, ClaimTaskRequest{RunnerID: "runner"}, time.Minute)
			failed := scenario == "failed refresh"
			if (err != nil) != failed || (failed && claim != nil) || (!failed && (claim == nil || !claim.Claimed)) {
				t.Fatal("refresh admission returned invalid execution ownership", claim, err)
			}
			wantCalls := 0
			if scenario == "developer" || failed {
				wantCalls = 1
			}
			if finder.calls != wantCalls {
				t.Fatal("baseline refresh was skipped or changed another execution lane", finder.calls)
			}
			if failed {
				finished, err := store.GetAttempt(ctx, attempt.ID)
				if err != nil || finished.Status != task.AttemptStatusFailed || !strings.Contains(finished.ErrorMessage, "no provider execution") || strings.Contains(finished.ErrorMessage, "PRIVATE_") {
					t.Fatal("refresh failure left active work or leaked connection details", finished, err)
				}
			}
		})
	}
}

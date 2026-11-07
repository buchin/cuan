package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestManagedReleaseCompletionAtomicallyHandsOffWithoutAcceptingTask(t *testing.T) {
	for _, scenario := range []string{"valid", "no admission", "no report", "changed source", "false deployment"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			runnerStore := NewMemoryStore(store)
			now := time.Now().UTC().Add(-time.Second)
			item := &task.Task{ID: "task", ProjectID: "project", SourceRoomID: "room", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode, Instructions: "Implement and deploy the approved change", CreatedAt: now.Add(-time.Minute), UpdatedAt: now}
			att := &task.TaskAttempt{ID: "attempt", TaskID: item.ID, RunnerID: "runner", Status: task.AttemptStatusRunning, CreatedAt: now.Add(-time.Second), StartedAt: &now}
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(store.CreateTask(ctx, item))
			must(store.CreateAttempt(ctx, att))
			p := &project.ProjectResponse{ID: "project", RepoOwner: "owner", RepoName: "repo", DefaultBranch: "dev", Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true,"environment":"CUAN_MANAGED_RELEASE=1"}}`)}
			svc := NewService(runnerStore, store, &mockProjectFinder{projects: map[string]*project.ProjectResponse{"project": p}})
			claim := &ClaimTaskResponse{Claimed: true, Task: item, Attempt: att, AutoDeploy: true, Environment: "CUAN_MANAGED_RELEASE=1", RepoOwner: "owner", RepoName: "repo", DeployBranch: "dev"}
			if scenario != "no admission" {
				must(svc.bindManagedReleaseClaim(ctx, ClaimTaskRequest{RunnerID: "runner", SupportsNativeRelease: true}, claim))
			}
			if scenario != "no report" {
				must(store.CreateActivity(ctx, &task.TaskActivity{ID: "report", TaskID: item.ID, AttemptID: &att.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, CreatedAt: now}))
				must(store.CreateActivity(ctx, &task.TaskActivity{ID: "kind", TaskID: item.ID, AttemptID: &att.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: now}))
			}
			changed := true
			req := CompleteAttemptRequest{RunnerID: "runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, CommitSHA: strings.Repeat("a", 40), BranchName: "task/approved", DiffSummary: "main.go | 1 +", TestSummary: "Project verification: test.sh --fast (exit 0)", AgentReportRequired: true, AgentReportConfirmed: true, HasCodeChanges: &changed, RequiresDeploy: &changed, DoDValidation: &DoDValidationResult{Passed: true, Action: "done"}}
			req.ManagedReleaseSource = &ManagedReleaseSource{Commit: req.CommitSHA, Branch: req.BranchName, ManifestHash: strings.Repeat("b", 64), VerificationHash: task.VerificationSummaryHash(req.TestSummary)}
			if scenario == "changed source" {
				req.CommitSHA = strings.Repeat("c", 40)
			}
			if scenario == "false deployment" {
				req.LiveDeploymentConfirmed = true
			}
			completed, err := svc.Complete(ctx, att.ID, req)
			must(err)
			saved, err := store.GetTask(ctx, item.ID)
			must(err)
			activities, err := store.ListActivities(ctx, item.ID)
			must(err)
			handoffs := 0
			for _, a := range activities {
				if a.ActivityType == task.ActivityNativeReleaseHandoff {
					handoffs++
				}
			}
			if scenario != "valid" {
				if completed.Status == task.AttemptStatusSucceeded || handoffs != 0 || saved.Status == task.TaskStatusDone {
					t.Fatal("invalid source accepted or handoff emitted")
				}
				return
			}
			if completed.Status != task.AttemptStatusSucceeded || saved.Status != task.TaskStatusReview || handoffs != 1 {
				t.Fatal("source completion lost pending publication state")
			}
			intent := &task.NativeReleaseIntent{SourceAttemptID: att.ID, SourceCommit: req.CommitSHA, SourceVerificationHash: req.ManagedReleaseSource.VerificationHash, InstructionsHash: task.LaunchInstructionsHash(item.Instructions), OwnerID: "owner", RoomID: item.SourceRoomID, SourceRequestID: "approved-request", Repository: "owner/repo", TargetBranch: "dev", ManifestHash: req.ManagedReleaseSource.ManifestHash}
			must(task.ValidateNativeReleaseSource(ctx, store, saved, intent))
			if task.VerifiedDeployment(activities, completed, req.CommitSHA, time.Now()) {
				t.Fatal("source handoff manufactured deployment proof")
			}
		})
	}
}

func TestManagedReleaseDeveloperSkipsGenericPublication(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
			defer api.Close()
			root := t.TempDir()
			worker := NewRunner(&config.Config{RunnerID: "runner", RunnerServerURL: api.URL, RunnerWorkerRegistryDir: t.TempDir()}, nil, api.Client())
			claim := ClaimTaskResponse{ManagedRelease: &ManagedReleaseClaim{Version: 1}, RepoOwner: "owner", RepoName: "repo", TaskAgent: &TaskAgentCapability{API: api.URL + "/task", Token: "bound"}}
			req := CompleteAttemptRequest{Status: task.AttemptStatusSucceeded, CommitSHA: strings.Repeat("a", 40), BranchName: "task/dev", TestSummary: "Project verification: test.sh --fast (exit 0)"}
			if !missing {
				req.ManagedReleaseSource = &ManagedReleaseSource{Commit: req.CommitSHA, Branch: req.BranchName, ManifestHash: strings.Repeat("b", 64), VerificationHash: task.VerificationSummaryHash(req.TestSummary)}
			}
			worker.postExecutionGitFlow(context.Background(), &git.TaskWorkspace{Path: root, BranchName: req.BranchName}, &task.Task{ID: "task", Instructions: "Implement and deploy"}, &task.TaskAttempt{ID: "attempt"}, claim, &req, task.ExecutionCategoryCode)
			if calls != 0 || req.PRURL != "" || req.LiveDeploymentConfirmed {
				t.Fatal("developer performed publication or claimed live proof")
			}
			if missing && req.Status != task.AttemptStatusFailed {
				t.Fatal("missing source passed native developer handoff")
			}
			if !missing && req.Status != task.AttemptStatusSucceeded {
				t.Fatal("valid source lost")
			}
			if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
				t.Fatal("generic git flow ran")
			}
		})
	}
}

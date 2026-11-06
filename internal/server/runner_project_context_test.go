package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestRunnerClaimHandlerDoesNotOverwriteSavedProjectContext(t *testing.T) {
	ctx := context.Background()
	s, _, _, _ := inboxFixture(t)
	projects := project.NewMemoryStore()
	p := &project.Project{ID: "claim-context-project", LocalPath: t.TempDir(), DefaultBranch: "main", Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":false,"environment":"TARGET=saved","deploy_branch":"saved-release"}}`)}
	if err := projects.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "fixture", t.TempDir())
	store := task.NewMemoryStore()
	s.taskSvc = task.NewService(store, s.projectSvc)
	s.runnerSvc = runner.NewService(runner.NewMemoryStore(store), store, s.projectSvc)
	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: p.ID, Title: "Approved feature", AssignedAgent: "agent_lifecycle", Instructions: "Implement approved feature"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "context-runner", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	p.Metadata = json.RawMessage(`{"cuan_settings":{"auto_deploy":true,"environment":"TARGET=latest","deploy_branch":"latest-release"}}`)
	if err := projects.UpdateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.cfg.RunnerLeaseDuration = time.Minute
	request := httptest.NewRequest("POST", "/api/v1/runner/claim", bytes.NewBufferString(`{"runner_id":"context-runner"}`))
	response := httptest.NewRecorder()
	s.handleRunnerClaim(response, request)
	if response.Code != 200 {
		t.Fatalf("claim failed: status %d", response.Code)
	}
	var claim runner.ClaimTaskResponse
	if err := json.Unmarshal(response.Body.Bytes(), &claim); err != nil {
		t.Fatal(err)
	}
	if !claim.Claimed || claim.Attempt.ID != a.ID || claim.Environment != "TARGET=saved" || claim.AutoDeploy || claim.DeployBranch != "saved-release" {
		t.Fatal("handler replaced saved execution defaults")
	}
	if claim.TaskAgent == nil || claim.TaskAgent.Token == "" {
		t.Fatal("saved context lost reporting capability")
	}
}

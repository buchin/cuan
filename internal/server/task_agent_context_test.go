package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestBoundTaskReadPreservesGuidanceWithoutEnvironmentSnapshot(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	const privateValue = "private-project-fixture-value"
	raw, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{Goal: "Original product goal", Instructions: "Preserve original acceptance", Environment: "PASSWORD=" + privateValue}})
	original := project.SnapshotProjectGuidance("Approved task. Do not deploy.", &project.ProjectResponse{ID: "project", Metadata: raw})
	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Context fixture", Instructions: original})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	_, _, token, cleanup, err := s.issueTaskAgent(ctx, created.ID, a.ID, "agent_lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/v1/task-agent/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal("bound task read failed", w.Code)
	}
	for _, forbidden := range []string{privateValue, "PASSWORD=", "cuan-project-context"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatal("bound task response exposed environment snapshot")
		}
	}
	for _, required := range []string{"Approved task.", "Do not deploy.", "Original product goal", "Preserve original acceptance"} {
		if !strings.Contains(w.Body.String(), required) {
			t.Fatal("bound task response lost guidance", required)
		}
	}
	saved, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil || saved.Instructions != created.Instructions || !strings.Contains(saved.Instructions, privateValue) {
		t.Fatal("display projection changed canonical task snapshot")
	}
}

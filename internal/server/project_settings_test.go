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

func TestProjectSettingsOwnerSaveAndTaskSnapshot(t *testing.T) {
	s, token := setupTestChatServer(t)
	ctx := context.Background()
	p, err := s.projectSvc.EnsureDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	request := project.UpdateSettingsRequest{Name: "Default", Goal: "Deliver a useful result", Instructions: "Keep approved scope", RepoURL: p.RepoURL, DefaultBranch: p.DefaultBranch, ExpectedUpdatedAt: p.UpdatedAt}
	raw, _ := json.Marshal(request)
	call := func(auth bool, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PATCH", "/api/v1/projects/default/settings", strings.NewReader(body))
		if auth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		s.Router().ServeHTTP(response, req)
		return response
	}
	if response := call(false, string(raw)); response.Code != 401 {
		t.Fatal("unauthenticated settings write", response.Code)
	}
	response := call(true, string(raw))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if response = call(true, string(raw)); response.Code != 409 {
		t.Fatal("stale version accepted", response.Code)
	}
	saved, _ := s.projectSvc.Get(ctx, p.ID)
	if saved.Name != "Default" || project.SettingsFromMetadata(saved.Metadata).Goal != request.Goal {
		t.Fatal("settings not persisted")
	}
	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: p.ID, Title: "Approved task", Instructions: "Approved work"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created.Instructions, request.Goal) || !strings.Contains(created.Instructions, "workspace_revision") {
		t.Fatal("task missing frozen project context")
	}
	request.ExpectedUpdatedAt = saved.UpdatedAt
	request.Goal = "A later goal"
	raw, _ = json.Marshal(request)
	if response = call(true, string(raw)); response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if strings.Contains(detail.Instructions, request.Goal) {
		t.Fatal("existing task scope changed")
	}
}

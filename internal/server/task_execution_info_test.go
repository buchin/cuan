package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

type inspectionReadGuard struct {
	*task.MemoryStore
}

func TestExecutionInspectionSelectsHistoricalAttemptWithoutCurrentRecoveryPlan(t *testing.T) {
	s, token, _, _ := inboxFixture(t)
	ctx := context.Background()
	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "PRIVATE_TITLE", Instructions: "PRIVATE_INSTRUCTIONS"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, attempt := range []*task.TaskAttempt{
		{ID: "old-attempt", TaskID: created.ID, AgentName: "agent", Status: task.AttemptStatusSucceeded, CreatedAt: now.Add(-time.Minute)},
		{ID: "current-attempt", TaskID: created.ID, AgentName: "agent", Status: task.AttemptStatusSucceeded, CreatedAt: now},
		{ID: "foreign-attempt", TaskID: "another-task", AgentName: "agent", Status: task.AttemptStatusSucceeded, CreatedAt: now},
	} {
		if err := s.taskSvc.Store().CreateAttempt(ctx, attempt); err != nil {
			t.Fatal(err)
		}
	}
	for _, selected := range []string{"old-attempt", "foreign-attempt", "missing-attempt"} {
		request := httptest.NewRequest("GET", "/api/v1/tasks/"+created.ID+"/execution?attempt_id="+selected, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		s.Router().ServeHTTP(response, request)
		if selected == "old-attempt" {
			if response.Code != 200 || !strings.Contains(response.Body.String(), `"attempt_id":"old-attempt"`) || strings.Contains(response.Body.String(), "live_inspection_plan") {
				t.Fatal("historical inspection returned current execution context", response.Code, response.Body.String())
			}
		} else if response.Code != 404 {
			t.Fatal("missing/foreign attempt disclosed or replaced by latest", response.Code)
		}
		if strings.Contains(response.Body.String(), "PRIVATE_") {
			t.Fatal("diagnostic disclosed private task content")
		}
	}
}

func (g inspectionReadGuard) UpdateTask(context.Context, *task.Task) error {
	panic("inspection diagnostic attempted UpdateTask")
}
func (g inspectionReadGuard) SaveLaunch(context.Context, task.TaskLaunch) (*task.TaskLaunch, error) {
	panic("inspection diagnostic attempted SaveLaunch")
}

func TestExecutionInspectionIsAuthenticatedAndExcludesTaskContent(t *testing.T) {
	s, token, _, _ := inboxFixture(t)
	created, err := s.taskSvc.CreateTask(context.Background(), "owner", task.CreateTaskRequest{Title: "PRIVATE_TITLE", Instructions: "PRIVATE_INSTRUCTIONS"})
	if err != nil {
		t.Fatal(err)
	}
	for _, authorized := range []bool{false, true} {
		request := httptest.NewRequest("GET", "/api/v1/tasks/"+created.ID+"/execution", nil)
		if authorized {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		s.Router().ServeHTTP(response, request)
		if authorized && (response.Code != 200 || !strings.Contains(response.Body.String(), `"intent_kind":"no_attempt"`)) {
			t.Fatal("inspection unavailable", response.Code)
		}
		if !authorized && response.Code != 401 {
			t.Fatal("unauthenticated inspection allowed", response.Code)
		}
		if strings.Contains(response.Body.String(), "PRIVATE_") {
			t.Fatal("private content escaped diagnostic projection")
		}
		if authorized && !strings.Contains(response.Body.String(), `"code":"not_managed_inbox_work"`) {
			t.Fatal("missing native plan diagnostic")
		}
	}
}

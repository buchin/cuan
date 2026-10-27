package server

import (
	"context"
	"github.com/buchin/cuan/internal/task"
	"net/http/httptest"
	"testing"
)

func TestTaskPushRequiresLiveBoundAttempt(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	created := workspaceTask(t, call)
	ctx := context.Background()
	attempt, e := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if e != nil {
		t.Fatal(e)
	}
	s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	_, _, token, cleanup, e := s.issueTaskAgent(ctx, created.ID, attempt.ID, "agent_lifecycle")
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup()
	push := func(id, credential string) int {
		r := httptest.NewRequest("POST", "/api/v1/task-agent/"+id+"/push", nil)
		r.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w.Code
	}
	if got := push(created.ID, ""); got != 401 {
		t.Fatal("push lacks authentication", got)
	}
	if got := push("other-task", token); got != 403 {
		t.Fatal("push escaped task", got)
	}
	if got := push(created.ID, token); got != 409 {
		t.Fatal("task without Git repo unexpectedly pushed", got)
	}
	s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed})
	if got := push(created.ID, token); got != 409 {
		t.Fatal("finished attempt retained push access", got)
	}
}

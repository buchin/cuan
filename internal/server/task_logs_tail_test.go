package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

func TestTaskLogTailPreservesAttemptBindingAndOrder(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	d, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two"} {
		if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: id, TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"one", "two"} {
		if err := s.taskSvc.AppendAttemptLogs(ctx, id, []task.LogRecord{{Stream: "system", Line: "first"}, {Stream: "system", Line: "second"}, {Stream: "system", Line: "last"}}, 100); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		query, taskID string
		code          int
		first         string
	}{
		{"tail=true&limit=2", d.ID, 200, "second"},
		{"after_id=0&limit=2", d.ID, 200, "first"},
		{"tail=true&after_id=1", d.ID, 400, ""},
		{"tail=bogus", d.ID, 400, ""},
		{"tail=true", "foreign", 404, ""},
	} {
		t.Run(test.query+test.taskID, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/logs?"+test.query, nil)
			route := chi.NewRouteContext()
			route.URLParams.Add("id", test.taskID)
			route.URLParams.Add("attemptId", "one")
			req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, route))
			recorder := httptest.NewRecorder()
			s.handleGetAttemptLogs(recorder, req)
			if recorder.Code != test.code {
				t.Fatal(recorder.Code, recorder.Body.String())
			}
			if test.code == 200 {
				var body struct {
					Logs []*task.LogRecord `json:"logs"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if len(body.Logs) != 2 || body.Logs[0].Line != test.first || body.Logs[0].AttemptID != "one" || body.Logs[0].ID >= body.Logs[1].ID {
					t.Fatal(body.Logs)
				}
			}
		})
	}
}

package server

import (
	"context"
	"github.com/buchin/cuan/internal/task"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunnerReportReadRequiresBoundActiveAttemptAndOmitsInstructions(t *testing.T) {
	s, token := setupTestChatServer(t)
	ctx := context.Background()
	d, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Report recovery", Instructions: "private-environment-do-not-return"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_dev"})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	if _, err = s.taskSvc.UpdateTask(ctx, d.ID, "runner", task.UpdateTaskRequest{Status: &doing}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}

	summary, note := "Delivered guide", "Read and counted the guide"
	if _, err = s.taskSvc.ReportAgent(ctx, d.ID, a.ID, "agent", task.AgentReport{State: "review", Summary: &summary, CompleteNote: &note}); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/runner/attempts/" + a.ID + "/report?task_id=" + d.ID + "&runner_id="
	for _, tc := range []struct {
		runner, auth string
		code         int
	}{{"local_terminal", token, 200}, {"runner-b", token, 409}, {"local_terminal", "", 401}} {
		req := httptest.NewRequest("GET", path+tc.runner, nil)
		if tc.auth != "" {
			req.Header.Set("Authorization", "Bearer "+tc.auth)
		}
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		if rec.Code != tc.code {
			t.Fatalf("status=%d expected=%d", rec.Code, tc.code)
		}
		if strings.Contains(rec.Body.String(), "private-environment-do-not-return") || strings.Contains(rec.Body.String(), "instructions") {
			t.Fatal("private task scope leaked")
		}
		if tc.code == 200 && !strings.Contains(rec.Body.String(), note) {
			t.Fatal("native evidence absent")
		}
	}
	if _, err = s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", path+"local_terminal", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != 409 {
		t.Fatal("finished attempt retained runner access")
	}
}

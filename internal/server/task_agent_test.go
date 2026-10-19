package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "task" {
		os.Exit(taskcli.Run(os.Args[2:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

const fixtureReviewReport = "\n\"$CUAN_TASK_EXECUTABLE\" task report --summary 'Fixture implementation complete' --complete-note 'Verified fixture output and behavior' --state review\n"

func prepareTaskReporter(t *testing.T, s *Server) {
	t.Helper()
	api := httptest.NewServer(s.Router())
	s.taskAgentURL = api.URL
	t.Cleanup(api.Close)
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	file := filepath.Join(bin, "agy")
	data, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, append(data, []byte(fixtureReviewReport)...), 0700); e != nil {
		t.Fatal(e)
	}
}
func TestTaskAgentScopedReportAndReviewGate(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	created := workspaceTask(t, call)
	ctx := context.Background()
	a, e := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if e != nil {
		t.Fatal(e)
	}
	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	env, _, token, cleanup, e := s.issueTaskAgent(ctx, created.ID, a.ID, "agent_lifecycle")
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup()
	var cap taskcli.Capability
	raw, _ := os.ReadFile(strings.TrimPrefix(env[0], "CUAN_TASK_CAPABILITY_FILE="))
	json.Unmarshal(raw, &cap)
	invoke := func(method, id, body, credential string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/task-agent/"+id, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	if w := invoke("GET", created.ID, "", token); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := invoke("GET", "different_task", "", token); w.Code != 403 {
		t.Fatal("cross-task access", w.Code)
	}
	if w := invoke("GET", created.ID, "", "owner-token"); w.Code != 401 {
		t.Fatal("unscoped credential", w.Code)
	}
	for _, body := range []string{`{"instructions":"overwritten"}`, `{"status":"done"}`, `{"state":"done"}`, `{"state":"review"}`} {
		if w := invoke("POST", created.ID, body, token); w.Code != 400 {
			t.Fatal("unsafe or incomplete report accepted", body, w.Code)
		}
	}
	w := invoke("POST", created.ID, `{"title":"Refined by agent","summary":"Implemented fixture","progress":"Checking output"}`, token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = invoke("POST", created.ID, `{"state":"review","complete_note":"Verified actual output","dod_checklist":[{"text":"Verify the result","completed":false}]}`, token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if detail.Status != "doing" || detail.Instructions != created.Instructions {
		t.Fatal("report approved active work or changed human instructions")
	}
	zero := 0
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusSucceeded, ExitCode: &zero})
	if e = s.taskSvc.ReconcileLocalTask(ctx, created.ID, a.ID); e != nil {
		t.Fatal(e)
	}
	detail, _ = s.taskSvc.GetTaskDetail(ctx, created.ID)
	if detail.Status != "review" || detail.Summary != "Implemented fixture" {
		t.Fatal(detail.Status)
	}
	if w = invoke("POST", created.ID, `{"progress":"late"}`, token); w.Code != 409 {
		t.Fatal("late report accepted", w.Code)
	}
	cleanup()
	if w = invoke("GET", created.ID, "", token); w.Code != 401 {
		t.Fatal("revoked access", w.Code)
	}
}
func TestTaskAgentExitWithoutReportIsBlocked(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	c := workspaceTask(t, call)
	w := call("POST", "/api/v1/tasks/"+c.ID+"/run-agent", `{"agent_id":"agent_lifecycle","request_id":"90000000-0000-4000-8000-000000000001"}`)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	d := waitWorkspaceTask(t, s, c.ID, "succeeded")
	if d.Status != "blocked" || !strings.Contains(d.LatestAttempt.ErrorMessage, "No review-ready result") {
		t.Fatal("process exit claimed a result", d.Status, d.LatestAttempt.ErrorMessage)
	}
}

func TestTaskAgentFailureAndBlockerDoNotBecomeReview(t *testing.T) {
	for _, c := range []struct {
		name, state string
		exit        int
		attempt     string
	}{{"failed_after_report", "review", 7, "failed"}, {"reported_blocker", "blocked", 0, "succeeded"}} {
		t.Run(c.name, func(t *testing.T) {
			s, _, call, _ := inboxFixture(t)
			api := httptest.NewServer(s.Router())
			defer api.Close()
			s.taskAgentURL = api.URL
			script := "#!/bin/sh\n\"$CUAN_TASK_EXECUTABLE\" task report --summary 'Actual fixture result' --complete-note 'Concrete fixture evidence' --state " + c.state + "\nexit " + fmt.Sprint(c.exit) + "\n"
			if e := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); e != nil {
				t.Fatal(e)
			}
			created := workspaceTask(t, call)
			w := call("POST", "/api/v1/tasks/"+created.ID+"/run-agent", `{"agent_id":"agent_lifecycle","request_id":"90000000-0000-4000-8000-000000000002"}`)
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			d := waitWorkspaceTask(t, s, created.ID, c.attempt)
			if d.Status != "blocked" || d.CompleteNote == "" {
				t.Fatal("unsuccessful work became review", d.Status)
			}
		})
	}
}

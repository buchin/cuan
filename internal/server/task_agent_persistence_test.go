package server

import (
	"context"
	"crypto/sha256"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestTaskReportingGrantSurvivesServerMemoryLoss(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	created := workspaceTask(t, call)
	a, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	if _, err = s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing}); err != nil {
		t.Fatal(err)
	}
	_, _, token, cleanup, err := s.issueTaskAgent(ctx, created.ID, a.ID, "agent_lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	digest := sha256.Sum256([]byte(token))
	s.taskAgentGrants.Delete(digest)
	invoke := func(id, credential string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		return w
	}
	if w := invoke(created.ID, token); w.Code != 200 {
		t.Fatal("issued grant lost after restart", w.Code)
	}
	if w := invoke("another-task", token); w.Code != 403 {
		t.Fatal("cross-task access", w.Code)
	}
	// A copied ciphertext cannot authorize a different bearer token.
	forged := strings.Repeat("f", 64)
	other := sha256.Sum256([]byte(forged))
	raw, _ := os.ReadFile(taskGrantPath(digest))
	os.WriteFile(taskGrantPath(other), raw, 0600)
	defer os.Remove(taskGrantPath(other))
	if w := invoke(created.ID, forged); w.Code != 401 {
		t.Fatal("grant ciphertext transplanted", w.Code)
	}
	req := httptest.NewRequest("POST", "/api/v1/task-agent/"+created.ID, strings.NewReader(`{"state":"review","summary":"Recovered report","complete_note":"Verified saved output after server restart","dod_checklist":[{"text":"Verify the result","completed":true,"evidence":"Saved fixture output observed after restart"}]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	out := httptest.NewRecorder()
	s.Router().ServeHTTP(out, req)
	if out.Code != 200 {
		t.Fatal("final report rejected after restart", out.Code, out.Body.String())
	}
	latest, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if latest.Summary != "Recovered report" {
		t.Fatal("report not persisted")
	}
	// Cleanup revokes both memory and durable grant; unknown tokens stay rejected.
	cleanup()
	if w := invoke(created.ID, token); w.Code != 401 {
		t.Fatal("cleanup did not revoke durable grant", w.Code)
	}
}

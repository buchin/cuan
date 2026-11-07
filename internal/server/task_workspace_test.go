package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
	"github.com/gorilla/websocket"
)

func workspaceTask(t *testing.T, call func(string, string, string) *httptest.ResponseRecorder) task.Task {
	t.Helper()
	w := call("POST", "/api/v1/tasks", `{"request_id":"20000000-0000-4000-8000-000000000001","title":"Task workspace fixture","instructions":"Keep the human task intact","dod_checklist":[{"text":"Verify the result","completed":false}]}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result task.Task
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func waitWorkspaceTask(t *testing.T, s *Server, id, state string) *task.TaskDetailResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		detail, err := s.taskSvc.GetTaskDetail(context.Background(), id)
		if err == nil && detail.LatestAttempt != nil && detail.LatestAttempt.Status == state && detail.Status != "doing" {
			if s.terminalManager != nil && detail.ProjectID != "" {
				if active, _ := s.terminalManager.ProjectExecutionActive(detail.ProjectID); active {
					time.Sleep(10 * time.Millisecond)
					continue
				}
			}
			return detail
		}
		if time.Now().After(deadline) {
			diagnostic, _ := json.MarshalIndent(detail, "", "  ")
			t.Fatalf("expected terminal state %s; got %s %v", state, diagnostic, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func TestTaskWorkspaceLaunchReviewRevisionAndReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow workspace PTY test in short mode")
	}
	s, _, call, _ := inboxFixture(t)
	prepareTaskReporter(t, s)
	created := workspaceTask(t, call)
	path := "/api/v1/tasks/" + created.ID
	// Creating from the same form is atomic, even if the previous response was lost.
	again := call("POST", "/api/v1/tasks", `{"request_id":"20000000-0000-4000-8000-000000000001","title":"Task workspace fixture","instructions":"Keep the human task intact","dod_checklist":[{"text":"Verify the result","completed":false}]}`)
	if again.Code != 201 || !strings.Contains(again.Body.String(), created.ID) {
		t.Fatal("create retry", again.Code, again.Body.String())
	}
	changed := call("POST", "/api/v1/tasks", `{"request_id":"20000000-0000-4000-8000-000000000001","title":"Changed title"}`)
	if changed.Code != 409 {
		t.Fatal("changed create retry", changed.Code)
	}
	noSession := call("GET", path+"/terminal", "")
	if noSession.Code != 404 || len(s.terminalManager.ActiveSessions()) != 0 {
		t.Fatal("passive terminal spawned a shell", noSession.Code)
	}
	payload := `{"request_id":"30000000-0000-4000-8000-000000000001","agent_id":"agent_lifecycle"}`
	first := call("POST", path+"/run-agent", payload)
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body.String())
	}
	if deleted := call("DELETE", path, ""); deleted.Code != 409 {
		t.Fatal("active task deletion was allowed", deleted.Code)
	}
	detail := waitWorkspaceTask(t, s, created.ID, "succeeded")
	if detail.Status != "review" {
		t.Fatal("exit zero auto-approved task")
	}
	firstID := detail.LatestAttempt.ID
	replay := call("POST", path+"/run-agent", payload)
	if replay.Code != 200 || !strings.Contains(replay.Body.String(), `"replayed":true`) || !strings.Contains(replay.Body.String(), firstID) {
		t.Fatal("completed request replay", replay.Code, replay.Body.String())
	}
	if len(s.terminalManager.GetTaskHistory(created.ID)) != 1 {
		t.Fatal("completed request launched twice")
	}
	if revision := call("POST", path+"/run-agent", `{"request_id":"30000000-0000-4000-8000-000000000002","agent_id":"agent_lifecycle"}`); revision.Code != 400 {
		t.Fatal("review revision lacked concrete feedback", revision.Code)
	}
	revision := call("POST", path+"/run-agent", `{"request_id":"30000000-0000-4000-8000-000000000002","agent_id":"agent_lifecycle","revision":"Fix the mobile layout and verify at390px"}`)
	if revision.Code != 200 {
		t.Fatal(revision.Code, revision.Body.String())
	}
	detail = waitWorkspaceTask(t, s, created.ID, "succeeded")
	if len(detail.Attempts) != 2 || detail.Instructions != created.Instructions {
		t.Fatal("revision changed human task or queued ghost attempt", detail)
	}
	found := false
	for _, a := range detail.Activities {
		if a.ActivityType == "comment" && strings.Contains(a.Message, "Fix the mobile layout") {
			found = true
		}
	}
	if !found {
		t.Fatal("revision feedback not retained")
	}
	rejected := call("PUT", path, `{"status":"done","complete_note":"Verified mobile layout"}`)
	if rejected.Code != 400 {
		t.Fatal("unchecked result approved", rejected.Code)
	}
	approved := call("PUT", path, `{"status":"done","summary":"Improved mobile layout","complete_note":"Verified the actual result at390px","dod_checklist":[{"text":"Verify the result","completed":true}]}`)
	if approved.Code != 200 {
		t.Fatal(approved.Code, approved.Body.String())
	}
	detail, _ = s.taskSvc.GetTaskDetail(context.Background(), created.ID)
	if detail.Status != "done" || detail.CompleteNote == "" || detail.Summary == "" || detail.Instructions != created.Instructions {
		t.Fatal("review fields not persisted", detail)
	}
	if run := call("POST", path+"/run-agent", `{"request_id":"30000000-0000-4000-8000-000000000003","agent_id":"agent_lifecycle"}`); run.Code != 409 {
		t.Fatal("completed task restarted without reopening", run.Code)
	}
}
func TestTaskWorkspaceTerminalAttachInputReconnectAndSavedOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow workspace PTY test in short mode")
	}
	s, token, call, _ := inboxFixture(t)
	prepareTaskReporter(t, s)
	// A real local PTY fixture, never a provider invocation.
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nprintf 'READY\\n'\nread -r answer\nprintf 'ACK:%s\\n' \"$answer\"\n"+fixtureReviewReport), 0700); err != nil {
		t.Fatal(err)
	}
	created := workspaceTask(t, call)
	path := "/api/v1/tasks/" + created.ID
	response := call("POST", path+"/run-agent", `{"request_id":"40000000-0000-4000-8000-000000000001","agent_id":"agent_lifecycle"}`)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var run struct {
		SessionID string           `json:"session_id"`
		Attempt   task.TaskAttempt `json:"attempt"`
	}
	json.Unmarshal(response.Body.Bytes(), &run)
	server := httptest.NewServer(s.Router())
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + path + "/terminal?attach=1&mode=watch&session_id=" + run.SessionID
	dialer := websocket.Dialer{Subprotocols: []string{"cuan-terminal", "token." + token}, HandshakeTimeout: 2 * time.Second}
	foreign := http.Header{"Origin": []string{"https://untrusted.example"}}
	if conn, res, err := dialer.Dial(endpoint, foreign); err == nil {
		conn.Close()
		t.Fatal("foreign origin accepted")
	} else if res.StatusCode != 403 {
		t.Fatal("wrong origin rejection", res.StatusCode)
	}
	if conn, res, err := dialer.Dial(strings.Replace(endpoint, run.SessionID, "wrong-session", 1), nil); err == nil {
		conn.Close()
		t.Fatal("wrong session attached")
	} else if res.StatusCode != 409 {
		t.Fatal("wrong session response", res.StatusCode)
	}
	connect := func() *websocket.Conn {
		c, _, err := dialer.Dial(endpoint, nil)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	readControl := func(c *websocket.Conn, kind string) map[string]any {
		t.Helper()
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			typ, data, err := c.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			if typ != websocket.TextMessage {
				continue
			}
			var value map[string]any
			if json.Unmarshal(data, &value) == nil && value["type"] == kind {
				return value
			}
		}
	}
	c := connect()
	if message := readControl(c, "connected"); message["session_id"] != run.SessionID {
		t.Fatal("wrong handshake")
	}
	c.WriteJSON(map[string]any{"type": "stdin", "data": "must-not-reach-process\n"})
	if message := readControl(c, "error"); !strings.Contains(message["message"].(string), "watch mode") {
		t.Fatal(message)
	}
	c.Close()
	time.Sleep(25 * time.Millisecond)
	s.terminalManager.CleanIdleSessions(time.Nanosecond)
	if _, ok := s.terminalManager.GetSession(created.ID); !ok {
		t.Fatal("view disconnect stopped active work")
	}
	c = connect()
	defer c.Close()
	readControl(c, "connected")
	c.WriteJSON(map[string]any{"type": "mode", "mode": "interactive"})
	if message := readControl(c, "status"); message["mode"] != "interactive" {
		t.Fatal("input mode unconfirmed")
	}
	c.WriteJSON(map[string]any{"type": "stdin", "data": "confirmed-input\n"})
	waitWorkspaceTask(t, s, created.ID, "succeeded")
	output := call("GET", path+"/attempts/"+run.Attempt.ID+"/logs", "")
	if output.Code != 200 || !strings.Contains(output.Body.String(), "ACK:confirmed-input") || strings.Contains(output.Body.String(), "must-not-reach-process") {
		t.Fatal("saved output/input isolation", output.Code, output.Body.String())
	}
	if passive := call("GET", path+"/terminal", ""); passive.Code != 404 {
		t.Fatal("finished session replaced by shell", passive.Code)
	}
	if len(s.terminalManager.GetTaskHistory(created.ID)) != 1 {
		t.Fatal("reconnect launched duplicate agent")
	}
}

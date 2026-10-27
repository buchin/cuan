package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxAgentTopicsIsolationAndReplay(t *testing.T) {
	s, _, call, home := inboxFixture(t)
	ctx := context.Background()
	unauthorized := httptest.NewRecorder()
	s.Router().ServeHTTP(unauthorized, httptest.NewRequest("POST", "/api/v1/chat/conversations", strings.NewReader(`{}`)))
	if unauthorized.Code != 401 {
		t.Fatal(unauthorized.Code)
	}
	create := func(id string) ChatContact {
		t.Helper()
		raw, _ := json.Marshal(map[string]string{"request_id": id, "agent_id": "agent_lifecycle"})
		w := call("POST", "/api/v1/chat/conversations", string(raw))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var c ChatContact
		json.Unmarshal(w.Body.Bytes(), &c)
		return c
	}
	firstID, secondID := db.GenerateUUID(), db.GenerateUUID()
	first, second := create(firstID), create(secondID)
	if first.ConversationID == second.ConversationID || create(firstID).ConversationID != firstID || first.AgentID != "agent_lifecycle" {
		t.Fatal(first, second)
	}
	items, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(items) != 0 || len(s.terminalManager.ActiveSessions()) != 0 {
		t.Fatal("opening a topic executed work")
	}
	send := func(room, text string) {
		t.Helper()
		id := db.GenerateUUID()
		raw, _ := json.Marshal(map[string]string{"request_id": id, "message": text})
		w := call("POST", "/api/v1/chat/conversations/"+room+"/messages", string(raw))
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		waitInboxState(t, s, room, id, "completed")
	}
	send(firstID, "FIRST_TOPIC_PRIVATE_SENTINEL")
	send(secondID, "SECOND_TOPIC_INDEPENDENT_SENTINEL")
	prompt, _ := os.ReadFile(filepath.Join(home, "prompts"))
	parts := strings.Split(string(prompt), "---TURN---")
	last := parts[len(parts)-1]
	if !strings.Contains(last, "SECOND_TOPIC_INDEPENDENT_SENTINEL") || strings.Contains(last, "FIRST_TOPIC_PRIVATE_SENTINEL") {
		t.Fatal("topic context mixed", last)
	}
	w := call("GET", "/api/v1/chat/contacts", "")
	var list struct{ Contacts []ChatContact }
	json.Unmarshal(w.Body.Bytes(), &list)
	found := 0
	for _, c := range list.Contacts {
		if c.ConversationID == firstID || c.ConversationID == secondID {
			found++
			if c.AgentID != "agent_lifecycle" || c.Name == "New conversation" {
				t.Fatal("topic metadata missing", c)
			}
		}
	}
	if found != 2 {
		t.Fatal("topics not listed", list)
	}
	resolved := s.resolveConversationContext(ctx, secondID, "")
	if resolved.RoomID != secondID || resolved.TargetAgentID != "agent_lifecycle" {
		t.Fatal(resolved)
	}
	legacy := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	if legacy.RoomID == firstID || legacy.RoomID == secondID {
		t.Fatal("legacy alias moved into a topic")
	}
	msgs, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, firstID)
	for _, m := range msgs {
		if strings.Contains(m.Content, "SECOND_TOPIC") {
			t.Fatal("cross-topic message leakage")
		}
	}
	items, _ = s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(items) != 0 {
		t.Fatal("ordinary topic chat created tasks")
	}
	for _, body := range []string{`{"agent_id":"agent_lifecycle","request_id":"bad"}`, `{"agent_id":"missing","request_id":"` + db.GenerateUUID() + `"}`} {
		if code := call("POST", "/api/v1/chat/conversations", body).Code; code < 400 {
			t.Fatal("invalid topic accepted", code)
		}
	}
}

func TestNewTasksUseDefaultProject(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	p, err := s.projectSvc.Get(context.Background(), "default")
	if err != nil || p == nil {
		t.Fatal("fresh installation has no Default", err)
	}
	body := `{"title":"General work","instructions":"Save general work without a custom project","request_id":"c235ae61-42e7-44c4-b2b1-980f2e2c54a7"}`
	w := call("POST", "/api/v1/tasks", body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var created task.Task
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.ProjectID != "default" || len(created.ProjectIDs) != 1 || created.ProjectIDs[0] != "default" {
		t.Fatal(created)
	}
	replay := call("POST", "/api/v1/tasks", body)
	if replay.Code != 201 {
		t.Fatal("default assignment broke idempotency", replay.Code, replay.Body.String())
	}
}

package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestStudioGoalAcceptanceAndReplay(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	id := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{"goal": "Hello, help me think through my next goal", "request_id": id})
	w := call("POST", "/api/v1/studio/goals", string(body))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil || accepted.ConversationID == "" {
		t.Fatal(err, w.Body.String())
	}
	waitInboxState(t, s, accepted.ConversationID, id, "completed")
	replay := call("POST", "/api/v1/studio/goals", string(body))
	if replay.Code != 202 || replay.Body.String() != w.Body.String() {
		t.Fatal(replay.Code, replay.Body.String())
	}
	changed, _ := json.Marshal(map[string]string{"goal": "A different goal", "request_id": id})
	if conflict := call("POST", "/api/v1/studio/goals", string(changed)); conflict.Code != 409 {
		t.Fatal(conflict.Code, conflict.Body.String())
	}
	items, _ := s.taskSvc.ListTasks(context.Background(), task.TaskFilter{})
	if len(items) != 0 || len(s.terminalManager.ActiveSessions()) != 0 {
		t.Fatal("ordinary Studio goal started task execution")
	}
	room, err := s.projectStudioRepo.GetRoomByID(context.Background(), accepted.ConversationID)
	if err != nil || roomAgentID(room.Topic) != "agent_lifecycle" {
		t.Fatal(room, err)
	}
	for _, invalid := range []string{`{}`, `{"goal":"hello","request_id":"bad"}`, `{"goal":"","request_id":"` + db.GenerateUUID() + `"}`} {
		if response := call("POST", "/api/v1/studio/goals", invalid); response.Code != 400 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	unauth := httptest.NewRecorder()
	s.Router().ServeHTTP(unauth, httptest.NewRequest("POST", "/api/v1/studio/goals", strings.NewReader(string(body))))
	if unauth.Code != 401 {
		t.Fatal(unauth.Code)
	}
	s.agentRepo = nil
	if response := call("POST", "/api/v1/studio/goals", string(body)); response.Code != 503 {
		t.Fatal(response.Code)
	}
}

package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestCuanStudioUsesConfiguredIdentityOverLegacyPick(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	err := s.agentRepo.CreateAgent(context.Background(), &db.Agent{ID: db.AgentIDCuan, Name: "@teman", Role: "Assistant", Harness: "agy", Provider: "agy", Enabled: true, Status: db.AgentStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []string{"", "agent_sm", "agent_lifecycle"} {
		id := db.GenerateUUID()
		raw, _ := json.Marshal(map[string]string{"goal": "Hello", "request_id": id, "agent_id": legacy})
		response := call("POST", "/api/v1/studio/goals", string(raw))
		if response.Code != 202 {
			t.Fatal(response.Code, response.Body.String())
		}
		var result struct {
			ConversationID string `json:"conversation_id"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &result)
		room, err := s.projectStudioRepo.GetRoomByID(context.Background(), result.ConversationID)
		if err != nil || room.Topic != "agent-topic:"+db.AgentIDCuan {
			t.Fatal("legacy routing leaked", room, err)
		}
		waitInboxState(t, s, result.ConversationID, id, "completed")
		replay := call("POST", "/api/v1/studio/goals", string(raw))
		if replay.Code != 202 {
			t.Fatal("replay changed identity", replay.Body.String())
		}
	}
}

func TestCuanReviewIsNotWorking(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	id := db.GenerateUUID()
	response := call("POST", "/api/v1/chat/conversations", `{"agent_id":"agent_lifecycle","request_id":"`+id+`"}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	err := s.taskSvc.Store().CreateTask(context.Background(), &task.Task{ID: "review-task", SourceRoomID: id, ProjectID: "default", Title: "Review result", Instructions: "Review", Status: task.TaskStatusReview})
	if err != nil {
		t.Fatal(err)
	}
	var contacts struct {
		Contacts []ChatContact `json:"contacts"`
	}
	response = call("GET", "/api/v1/chat/contacts", "")
	_ = json.Unmarshal(response.Body.Bytes(), &contacts)
	found := false
	for _, contact := range contacts.Contacts {
		if contact.ConversationID == id {
			found = true
			if contact.ActiveTasks != 0 || contact.ReviewTasks != 1 {
				t.Fatalf("incorrect counts %+v", contact)
			}
		}
	}
	if !found {
		t.Fatal("missing topic")
	}
	var detail struct {
		Contact ChatContact `json:"contact"`
		Active  int         `json:"active_tasks"`
		Review  int         `json:"review_tasks"`
	}
	response = call("GET", "/api/v1/chat/conversations/"+id+"/messages", "")
	_ = json.Unmarshal(response.Body.Bytes(), &detail)
	if detail.Active != 0 || detail.Review != 1 || detail.Contact.ReviewTasks != 1 {
		t.Fatal("detail disagrees with contacts", response.Body.String())
	}
	for _, state := range []string{"backlog", "review", "canceled", "cancelled", "done", "queued", "blocked"} {
		if inboxTaskIsWorking(state) {
			t.Fatal("inactive work counted", state)
		}
	}
}

func TestCuanCapacityWaitIsDurableAndCancelledWithoutReplay(t *testing.T) {
	now := time.Now()
	q := db.InboxRequest{State: "queued", SafeError: inboxQuotaNoBackup, Steps: []db.InboxStep{{Kind: "finished", At: now}}}
	if !inboxCapacityWaiting(q, now.Add(10*time.Second)) || inboxCapacityWaiting(q, now.Add(time.Minute)) {
		t.Fatal("capacity retry delay ignored")
	}
	q.State = "cancelled"
	if inboxCapacityWaiting(q, now) {
		t.Fatal("cancelled request rescheduled")
	}
}

func TestCuanLegacyConversationKeepsHistoryButRoutesNewTurnsToPublicIdentity(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	id := db.GenerateUUID()
	response := call("POST", "/api/v1/chat/conversations", `{"agent_id":"agent_lifecycle","request_id":"`+id+`"}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if err := s.agentRepo.CreateAgent(context.Background(), &db.Agent{ID: db.AgentIDCuan, Name: "@teman", Harness: "agy", Provider: "agy", Enabled: true, Status: db.AgentStatusActive}); err != nil {
		t.Fatal(err)
	}
	resolved := s.resolveConversationContext(context.Background(), id, "")
	if resolved.RoomID != id || resolved.TargetAgentID != db.AgentIDCuan || resolved.TargetAgentName != "@teman" {
		t.Fatalf("legacy identity leaked: %+v", resolved)
	}
	room, _ := s.projectStudioRepo.GetRoomByID(context.Background(), id)
	if room.Topic != "agent-topic:agent_lifecycle" {
		t.Fatal("historical room binding rewritten")
	}
}

func TestCuanCapacityQueuesSameMessageAndCancellationStopsRecovery(t *testing.T) {
	s, token, _, primary, backup, _, _ := setupFailoverFixture(t)
	for _, p := range []*agent.ProviderAccount{primary, backup} {
		if err := agent.RecordProviderAccountQuotaWithDuration(p.OwnerID, p.ID, time.Hour, ""); err != nil {
			t.Fatal(err)
		}
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		result := httptest.NewRecorder()
		s.Router().ServeHTTP(result, request)
		return result
	}
	roomID, requestID := db.GenerateUUID(), db.GenerateUUID()
	if response := call("POST", "/api/v1/chat/conversations", `{"agent_id":"agent_cuan","request_id":"`+roomID+`"}`); response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	response := call("POST", "/api/v1/chat/conversations/"+roomID+"/messages", `{"message":"Hello","request_id":"`+requestID+`"}`)
	if response.Code != 202 {
		t.Fatal(response.Body.String())
	}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	deadline := time.Now().Add(3 * time.Second)
	for {
		requests, _ := store.ListInboxRequests(context.Background(), roomID)
		if len(requests) == 1 && requests[0].SafeError == inboxQuotaNoBackup && requests[0].State == "queued" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("message did not wait for capacity: %+v", requests)
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.drainInboxRoom(roomID)
	requests, _ := store.ListInboxRequests(context.Background(), roomID)
	if len(requests) != 1 {
		t.Fatal("capacity wait duplicated request")
	}
	if _, err := store.CancelInboxRequest(context.Background(), roomID, requestID); err != nil {
		t.Fatal(err)
	}
	s.drainInboxRoom(roomID)
	requests, _ = store.ListInboxRequests(context.Background(), roomID)
	if len(requests) != 1 || requests[0].State != "cancelled" {
		t.Fatal("cancelled capacity wait was replayed", requests)
	}
}

func TestCuanAssistantNameEditIsDurable(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	_ = s.agentRepo.CreateAgent(context.Background(), &db.Agent{ID: db.AgentIDCuan, Name: "@cuan", Harness: "agy", Provider: "agy", Enabled: true, Status: db.AgentStatusActive})
	response := call("PATCH", "/api/v1/agents/agent_cuan", `{"name":"@teman"}`)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	saved, _ := s.agentRepo.GetAgentByID(context.Background(), db.AgentIDCuan)
	if saved.Name != "@teman" {
		t.Fatal("assistant rename was not persisted")
	}
}

func TestCuanBrainSettingsOwnerAuthenticated(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	p, err := agent.CreateProviderAccount(owner.ID, "Original")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"label":"Project brain","model_id":"gemini-test"}`
	response := call("PATCH", "/api/v1/provider-accounts/"+p.ID, body)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Project brain") {
		t.Fatal(response.Code, response.Body.String())
	}
	saved, err := agent.GetProviderAccount(owner.ID, p.ID)
	if err != nil || saved.ModelID != "gemini-test" {
		t.Fatal("brain preference not saved")
	}
	unauth := httptest.NewRecorder()
	s.Router().ServeHTTP(unauth, httptest.NewRequest("PATCH", "/api/v1/provider-accounts/"+p.ID, strings.NewReader(body)))
	if unauth.Code != 401 {
		t.Fatal("unauthenticated edit accepted", unauth.Code)
	}
}

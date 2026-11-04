package server

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/db"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInboxRetryAPIUsesOriginalAndChecksOwnership(t *testing.T) {
	s, token := setupTestChatServer(t)
	s.inboxStopping.Store(true)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	ctx := context.Background()
	now := time.Now().UTC()
	room := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Fixture", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_fixture", RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "attachment-snapshot", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err := store.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: room, AuthorType: "user", Content: "original message", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	q.State = "failed"
	q.SafeError = "The chat sandbox is unavailable."
	if err := store.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/chat/conversations/" + room + "/requests/" + q.RequestID + "/retry"
	call := func(path, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader("{}"))
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		return w
	}
	if w := call(path, ""); w.Code != 401 {
		t.Fatal("unauthenticated retry", w.Code)
	}
	if w := call(strings.Replace(path, room, db.GenerateUUID(), 1), token); w.Code != 404 {
		t.Fatal("cross-room retry", w.Code)
	}
	for i := 0; i < 3; i++ {
		w := call(path, token)
		if (i == 0 && w.Code != 202) || (i > 0 && w.Code != 200) {
			t.Fatal(w.Code, w.Body.String())
		}
		var body struct{ Request db.InboxRequest }
		json.Unmarshal(w.Body.Bytes(), &body)
		if body.Request.MessageID != q.MessageID || body.Request.RequestID != db.InboxRetryID(q.RequestID) {
			t.Fatal("correlation changed")
		}
	}
	all, _ := store.ListInboxRequests(ctx, room)
	if len(all) != 2 {
		t.Fatal("duplicate retry")
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, room)
	if len(messages) != 1 {
		t.Fatal("duplicate user message")
	}
	childPath := strings.Replace(path, q.RequestID, db.InboxRetryID(q.RequestID), 1)
	if w := call(childPath, token); w.Code != 409 {
		t.Fatal("active retry duplicated", w.Code)
	}
}

func TestInboxRetryCompletesOnceWithOriginalContext(t *testing.T) {
	s, token, call, home := inboxFixture(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	w := call("GET", "/api/v1/chat/conversations/agent_lifecycle/messages", "")
	var conversation struct {
		RoomID string `json:"room_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &conversation); err != nil || conversation.RoomID == "" {
		t.Fatal(w.Body.String())
	}
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: conversation.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "same", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if _, _, err := store.AcceptInboxRequest(context.Background(), q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "ORIGINAL_INTENT_FIXTURE", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	q.State = "failed"
	q.SafeError = "The chat sandbox is unavailable."
	if err := store.FinishInboxRequest(context.Background(), q, nil); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/chat/conversations/" + q.RoomID + "/requests/" + q.RequestID + "/retry"
	for i := 0; i < 3; i++ {
		w = call("POST", path, "{}")
		if w.Code != 200 && w.Code != 202 {
			t.Fatal(w.Body.String())
		}
	}
	waitInboxState(t, s, q.RoomID, db.InboxRetryID(q.RequestID), "completed")
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(context.Background(), q.RoomID)
	if len(messages) != 2 {
		t.Fatalf("want original user and one reply, got %d", len(messages))
	}
	prompt, err := os.ReadFile(filepath.Join(home, "prompts"))
	if err != nil || strings.Count(string(prompt), "---TURN---") != 1 || !strings.Contains(string(prompt), "ORIGINAL_INTENT_FIXTURE") {
		t.Fatal("retry lost context or duplicated provider execution", err)
	}
}

func TestInboxUncertainRetryReevaluatesOldMessageOnce(t *testing.T) {
	s, token, call, home := inboxFixture(t)
	ctx := context.Background()
	owner, _ := s.authSvc.ValidateToken(ctx, token)
	conversation := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: conversation.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "retained-original", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if _, _, err := store.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "OLD_ORIGINAL_QUESTION", CreatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: q.RoomID, AuthorType: "agent", Content: "Later context", CreatedAt: now.Add(time.Duration(i) * time.Millisecond)}); err != nil {
			t.Fatal(err)
		}
	}
	store.AppendInboxStep(ctx, q.RoomID, q.RequestID, "runtime_tool_started", "Uncertain previous action")
	q.State = "failed"
	q.SafeError = "The provider reached a usage limit after tool activity that cannot be safely repeated."
	if err := store.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/chat/conversations/" + q.RoomID + "/requests/" + q.RequestID + "/retry"
	for i := 0; i < 3; i++ {
		w := call("POST", path, "{}")
		if w.Code != 200 && w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	waitInboxState(t, s, q.RoomID, db.InboxRetryID(q.RequestID), "completed")
	rows, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, q.RoomID)
	if len(rows) != 2 || !db.InboxRetryNeedsReconciliation(rows[1]) {
		t.Fatal("reconciliation was duplicated or lost", len(rows))
	}
	prompt, err := os.ReadFile(filepath.Join(home, "prompts"))
	if err != nil || strings.Count(string(prompt), "---TURN---") != 1 || !strings.Contains(string(prompt), "OLD_ORIGINAL_QUESTION") || !strings.Contains(string(prompt), inboxRetryReconcileInstruction) {
		t.Fatal("fresh original-context evaluation missing", err)
	}
}

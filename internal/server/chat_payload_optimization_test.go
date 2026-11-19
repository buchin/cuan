package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
)

func TestChatMessagesPayloadOptimization_PrunesOldCompletedSteps(t *testing.T) {
	srv, token := setupTestChatServer(t)

	repo := cuanDB.NewMemoryProjectStudioRepository()
	srv.SetProjectStudioRepository(repo)

	roomID := "550e8400-e29b-41d4-a716-446655440000"
	msgID1 := "11111111-1111-1111-1111-111111111111"
	msgID2 := "22222222-2222-2222-2222-222222222222"

	// Create room
	err := repo.CreateRoom(context.Background(), &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Test Room",
		Topic:     "agent-topic:general",
	})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Create messages
	_ = repo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         msgID1,
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorName: "agent_cuan",
		Content:    "Hello 1",
	})
	_ = repo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         msgID2,
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorName: "agent_cuan",
		Content:    "Hello 2",
	})

	heavySteps := []cuanDB.InboxStep{
		{Seq: 1, Kind: "accepted", Text: "Message received.", At: time.Now()},
		{Seq: 2, Kind: "provider", Text: "Very heavy provider debug information text...", At: time.Now()},
	}

	// 1 active request for msgID2, 1 completed request for msgID1 (visible), 1 completed request for old non-visible msg
	_, _, _ = repo.AcceptInboxRequest(context.Background(), cuanDB.InboxRequest{
		RoomID:    roomID,
		RequestID: "req-1",
		MessageID: "old-non-visible-msg",
		State:     "completed",
		Steps:     heavySteps,
		OwnerID:   "admin",
		AgentID:   "agent_cuan",
	}, &cuanDB.ProjectMessage{ID: "m-old", RoomID: roomID, AuthorType: "user", Content: "old"})

	_, _, _ = repo.AcceptInboxRequest(context.Background(), cuanDB.InboxRequest{
		RoomID:    roomID,
		RequestID: "req-2",
		MessageID: msgID1,
		State:     "completed",
		Steps:     heavySteps,
		OwnerID:   "admin",
		AgentID:   "agent_cuan",
	}, &cuanDB.ProjectMessage{ID: "m-1", RoomID: roomID, AuthorType: "user", Content: "1"})

	_, _, _ = repo.AcceptInboxRequest(context.Background(), cuanDB.InboxRequest{
		RoomID:    roomID,
		RequestID: "req-3",
		MessageID: msgID2,
		State:     "running",
		Steps:     heavySteps,
		OwnerID:   "admin",
		AgentID:   "agent_cuan",
	}, &cuanDB.ProjectMessage{ID: "m-2", RoomID: roomID, AuthorType: "user", Content: "2"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+roomID+"/messages?limit=50", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Requests []inboxRecoveryDTO `json:"requests"`
		Messages []ChatMessageDTO   `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(res.Requests) != 3 {
		t.Fatalf("expected 3 requests metadata to be preserved, got %d", len(res.Requests))
	}

	var req1, req2, req3 *inboxRecoveryDTO
	for i := range res.Requests {
		switch res.Requests[i].RequestID {
		case "req-1":
			req1 = &res.Requests[i]
		case "req-2":
			req2 = &res.Requests[i]
		case "req-3":
			req3 = &res.Requests[i]
		}
	}

	if req1 == nil || len(req1.Steps) != 0 {
		t.Errorf("expected req-1 (old completed) to have pruned steps, got %d steps", len(req1.Steps))
	}
	if req2 == nil || len(req2.Steps) != 2 {
		t.Errorf("expected req-2 (visible completed) to preserve steps, got %d steps", len(req2.Steps))
	}
	if req3 == nil || len(req3.Steps) != 2 {
		t.Errorf("expected req-3 (running active) to preserve steps, got %d steps", len(req3.Steps))
	}
}

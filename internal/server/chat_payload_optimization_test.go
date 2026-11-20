package server

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestChatMessages_GetFormattedMessagesResultPaged(t *testing.T) {
	srv, _ := setupTestChatServer(t)
	repo := cuanDB.NewMemoryProjectStudioRepository()
	srv.SetProjectStudioRepository(repo)

	roomID := "660e8400-e29b-41d4-a716-446655440000"
	_ = repo.CreateRoom(context.Background(), &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Paging Room",
	})

	var msgIDs []string
	for i := 1; i <= 10; i++ {
		id := "00000000-0000-0000-0000-" + format12Digit(i)
		msgIDs = append(msgIDs, id)
		_ = repo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
			ID:         id,
			RoomID:     roomID,
			AuthorType: "user",
			Content:    "Message " + format12Digit(i),
		})
	}

	ctx := context.Background()

	// 1. Fetch latest page of 3
	page1, hasMore1, err := srv.getFormattedMessagesResultPaged(ctx, roomID, "", 3, "")
	if err != nil {
		t.Fatalf("page 1 failed: %v", err)
	}
	if len(page1) != 3 {
		t.Fatalf("expected 3 items in page 1, got %d", len(page1))
	}
	if !hasMore1 {
		t.Errorf("expected hasMore1 = true")
	}
	if page1[0].ID != 8 || page1[1].ID != 9 || page1[2].ID != 10 {
		t.Errorf("expected IDs 8, 9, 10, got %d, %d, %d", page1[0].ID, page1[1].ID, page1[2].ID)
	}
	if page1[0].UUID != msgIDs[7] || page1[2].UUID != msgIDs[9] {
		t.Errorf("mismatched UUIDs in page 1")
	}

	// 2. Fetch page before message 8
	page2, hasMore2, err := srv.getFormattedMessagesResultPaged(ctx, roomID, "", 3, msgIDs[7])
	if err != nil {
		t.Fatalf("page 2 failed: %v", err)
	}
	if len(page2) != 3 {
		t.Fatalf("expected 3 items in page 2, got %d", len(page2))
	}
	if !hasMore2 {
		t.Errorf("expected hasMore2 = true")
	}
	if page2[0].ID != 5 || page2[1].ID != 6 || page2[2].ID != 7 {
		t.Errorf("expected IDs 5, 6, 7, got %d, %d, %d", page2[0].ID, page2[1].ID, page2[2].ID)
	}

	// 3. Fetch page before message 2 (boundary reaching top)
	page3, hasMore3, err := srv.getFormattedMessagesResultPaged(ctx, roomID, "", 3, msgIDs[1])
	if err != nil {
		t.Fatalf("page 3 failed: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("expected 1 item in page 3, got %d", len(page3))
	}
	if hasMore3 {
		t.Errorf("expected hasMore3 = false when reaching start")
	}
	if page3[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", page3[0].ID)
	}

	// 4. Fetch all (limit = 0)
	pageAll, hasMoreAll, err := srv.getFormattedMessagesResultPaged(ctx, roomID, "", 0, "")
	if err != nil {
		t.Fatalf("page all failed: %v", err)
	}
	if len(pageAll) != 10 {
		t.Fatalf("expected 10 items in page all, got %d", len(pageAll))
	}
	if hasMoreAll {
		t.Errorf("expected hasMoreAll = false")
	}
	for i, m := range pageAll {
		if m.ID != int64(i+1) {
			t.Errorf("expected msg[%d].ID = %d, got %d", i, i+1, m.ID)
		}
	}
}

func format12Digit(n int) string {
	s := "000000000000" + fmt.Sprintf("%d", n)
	return s[len(s)-12:]
}

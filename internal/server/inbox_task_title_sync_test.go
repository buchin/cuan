package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestRoomTitleSyncOnTaskCreation(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()

	// 1. Create a new conversation
	roomID := db.GenerateUUID()
	w := call("POST", "/api/v1/chat/conversations", `{"agent_id":"agent_lifecycle","request_id":"`+roomID+`"}`)
	if w.Code != 200 {
		t.Fatalf("failed to create conversation: %s", w.Body.String())
	}

	room, err := s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if err != nil || room == nil {
		t.Fatalf("failed to load room: %v", err)
	}
	if room.Title != "New conversation" {
		t.Fatalf("expected initial title 'New conversation', got %q", room.Title)
	}

	// 2. User sends initial message
	msgID := db.GenerateUUID()
	userMsg := &db.ProjectMessage{
		ID:         msgID,
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "owner",
		AuthorName: "Owner",
		Content:    "Bantu buatkan backend auth",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, userMsg); err != nil {
		t.Fatalf("failed to create message: %v", err)
	}
	s.nameAgentTopic(ctx, roomID, userMsg.Content)

	room, _ = s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if room.Title != "Bantu buatkan backend auth" {
		t.Fatalf("expected initial message title 'Bantu buatkan backend auth', got %q", room.Title)
	}

	// 3. Create first task in this room (DoD 1: Pembuatan task baru otomatis memperbarui judul chat)
	t1, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		SourceRoomID: roomID,
		ProjectID:    "default",
		Title:        "Setup JWT Authentication",
		Instructions: "Setup JWT auth service",
	})
	if err != nil || t1 == nil {
		t.Fatalf("failed to create task 1: %v", err)
	}

	room, _ = s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if room.Title != "Setup JWT Authentication" {
		t.Fatalf("expected room title to sync to task 1 title 'Setup JWT Authentication', got %q", room.Title)
	}

	// 4. Create second task in the same room (Scope 3: Sinkronisasi task terbaru: mengikuti task paling baru)
	t2, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		SourceRoomID: roomID,
		ProjectID:    "default",
		Title:        "Add Refresh Token Mechanism",
		Instructions: "Implement refresh tokens",
	})
	if err != nil || t2 == nil {
		t.Fatalf("failed to create task 2: %v", err)
	}

	room, _ = s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if room.Title != "Add Refresh Token Mechanism" {
		t.Fatalf("expected room title to sync to latest task 2 title 'Add Refresh Token Mechanism', got %q", room.Title)
	}

	// 5. User manually renames the conversation (DoD 2: Judul manual tidak tertimpa otomatis)
	patchReq := `{"title":"Auth Service Production"}`
	w = call("PATCH", "/api/v1/chat/conversations/"+roomID, patchReq)
	if w.Code != 200 {
		t.Fatalf("failed to manually rename conversation: %s", w.Body.String())
	}

	room, _ = s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if room.Title != "Auth Service Production" {
		t.Fatalf("expected room title to be updated to 'Auth Service Production', got %q", room.Title)
	}

	// 6. Create third task in this room: verify manual title is preserved!
	t3, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		SourceRoomID: roomID,
		ProjectID:    "default",
		Title:        "Add Rate Limiting",
		Instructions: "Implement rate limiter",
	})
	if err != nil || t3 == nil {
		t.Fatalf("failed to create task 3: %v", err)
	}

	room, _ = s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if room.Title != "Auth Service Production" {
		t.Fatalf("expected manual title 'Auth Service Production' to be preserved, but got %q", room.Title)
	}
}

func TestRoomTitleSyncFromMessageTaskEndpoint(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()

	roomID := db.GenerateUUID()
	w := call("POST", "/api/v1/chat/conversations", `{"agent_id":"agent_lifecycle","request_id":"`+roomID+`"}`)
	if w.Code != 200 {
		t.Fatalf("failed to create conversation: %s", w.Body.String())
	}

	msgID := db.GenerateUUID()
	userMsg := &db.ProjectMessage{
		ID:         msgID,
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "owner",
		AuthorName: "Owner",
		Content:    "Tolong deploy worker",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, userMsg); err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	body := `{"title":"Deploy Worker Daemon","project_id":"default"}`
	w = call("POST", "/api/v1/chat/conversations/"+roomID+"/messages/"+msgID+"/task", body)
	if w.Code != 200 {
		t.Fatalf("failed to create task from message: %s", w.Body.String())
	}

	room, err := s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if err != nil || room == nil {
		t.Fatalf("failed to get room: %v", err)
	}
	if room.Title != "Deploy Worker Daemon" {
		t.Fatalf("expected room title 'Deploy Worker Daemon', got %q", room.Title)
	}
}

func TestInboxContactsAndStreamIncludesRoomTitle(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	ctx := context.Background()

	roomID := db.GenerateUUID()
	_ = call("POST", "/api/v1/chat/conversations", `{"agent_id":"agent_lifecycle","request_id":"`+roomID+`"}`)

	_, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		SourceRoomID: roomID,
		ProjectID:    "default",
		Title:        "Feature Reactive Title Sync",
		Instructions: "Verify reactivity",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Verify GET /api/v1/chat/contacts reports the updated title
	w := call("GET", "/api/v1/chat/contacts", "")
	if w.Code != 200 {
		t.Fatalf("contacts endpoint failed: %s", w.Body.String())
	}
	var contactsResp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &contactsResp); err != nil {
		t.Fatalf("failed to decode contacts: %v", err)
	}
	found := false
	for _, c := range contactsResp.Contacts {
		if c.ConversationID == roomID || c.ID == "topic_"+roomID {
			found = true
			if c.Name != "Feature Reactive Title Sync" {
				t.Fatalf("expected contact name 'Feature Reactive Title Sync', got %q", c.Name)
			}
		}
	}
	if !found {
		t.Fatalf("conversation %s not found in contacts list", roomID)
	}

	// Verify GET /api/v1/chat/conversations/{id}/events returns room_title in SSE snapshot
	req := httptest.NewRequest("GET", "/api/v1/chat/conversations/"+roomID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctxTimeout)

	s.Router().ServeHTTP(rec, req)

	respBody := rec.Body.String()
	if !strings.Contains(respBody, "Feature Reactive Title Sync") {
		t.Fatalf("expected SSE stream snapshot to contain room title 'Feature Reactive Title Sync', got: %s", respBody)
	}
	if !strings.Contains(respBody, `"room_title":`) {
		t.Fatalf("expected SSE stream data to contain room_title key, got: %s", respBody)
	}
}

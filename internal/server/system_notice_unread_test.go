package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestSystemLifecycleNoticeUnreadAndResolution(t *testing.T) {
	srv, token := setupTestChatServer(t)
	agentRepo := cuanDB.NewMemoryAgentRepository()
	srv.SetAgentRepository(agentRepo)

	// Create test agent
	err := agentRepo.CreateAgent(context.Background(), &cuanDB.Agent{
		ID:      "agent_scrum",
		Name:    "Scrum Master",
		Role:    "Scrum Master",
		Harness: "agy",
		Enabled: true,
		Status:  cuanDB.AgentStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var resp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(resp.Contacts))
	}
	roomID := resp.Contacts[0].ConversationID

	// 1. Send system lifecycle notice ("Sistem baru saja diperbarui...")
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "system",
		AuthorID:   "cuan_system_lifecycle",
		AuthorName: "Cuan",
		Content:    "Sistem baru saja diperbarui. Pekerjaan Anda tersimpan aman dan dilanjutkan secara otomatis...\n\n[View work](/tasks/task_123)",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	})

	// 2. Fetch contacts -> UnreadCount must still be 0!
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)

	var resp2 struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if resp2.Contacts[0].UnreadCount != 0 {
		t.Fatalf("expected 0 unread for system lifecycle notice, got %d", resp2.Contacts[0].UnreadCount)
	}
	if !resp2.Contacts[0].Resolved {
		t.Fatalf("expected conversation with only system lifecycle notice to be resolved, got resolved=%v", resp2.Contacts[0].Resolved)
	}
	if !resp2.Contacts[0].OnlySystemNotices {
		t.Fatalf("expected OnlySystemNotices=true, got %v", resp2.Contacts[0].OnlySystemNotices)
	}

	// 3. Verify message is still present in chat timeline
	reqMsgs := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+roomID+"/messages", nil)
	reqMsgs.Header.Set("Authorization", "Bearer "+token)
	recMsgs := httptest.NewRecorder()
	srv.Router().ServeHTTP(recMsgs, reqMsgs)

	var msgResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recMsgs.Body.Bytes(), &msgResp); err != nil {
		t.Fatal(err)
	}
	if len(msgResp.Messages) == 0 {
		t.Fatalf("expected system lifecycle notice to remain in timeline, got 0 messages")
	}

	// 4. Now add a conversational message, resolve the room, and send another system lifecycle notice
	// Conversation should remain resolved and not be reopened by the lifecycle notice.
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "user_1",
		AuthorName: "User",
		Content:    "Pekerjaan bagus, terima kasih.",
		CreatedAt:  time.Now().UTC().Add(time.Second),
		UpdatedAt:  time.Now().UTC().Add(time.Second),
	})

	now := time.Now().UTC().Add(2 * time.Second)
	if err := srv.projectStudioRepo.SetRoomResolved(context.Background(), roomID, &now); err != nil {
		t.Fatalf("failed to resolve room: %v", err)
	}

	// Verify room is resolved
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	rec3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec3, req3)
	var resp3 struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(rec3.Body.Bytes(), &resp3)
	if !resp3.Contacts[0].Resolved {
		t.Fatalf("expected room to be resolved, got %v", resp3.Contacts[0].Resolved)
	}

	// Now a new system lifecycle notice arrives after the resolution timestamp
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "system",
		AuthorID:   "cuan_system_lifecycle",
		AuthorName: "Cuan",
		Content:    "Sistem baru saja diperbarui. Pekerjaan Anda tersimpan aman dan dilanjutkan secara otomatis...",
		CreatedAt:  now.Add(time.Second),
		UpdatedAt:  now.Add(time.Second),
	})

	// Room must REMAIN resolved and unread count must remain 0!
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	rec4 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec4, req4)
	var resp4 struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(rec4.Body.Bytes(), &resp4)
	if !resp4.Contacts[0].Resolved {
		t.Fatalf("expected room to remain resolved after system lifecycle notice, got %v", resp4.Contacts[0].Resolved)
	}
	if resp4.Contacts[0].UnreadCount != 0 {
		t.Fatalf("expected 0 unread count after system lifecycle notice, got %d", resp4.Contacts[0].UnreadCount)
	}
	// Preview should keep the conversational message instead of system update spam
	if resp4.Contacts[0].LastMessage != "Pekerjaan bagus, terima kasih." {
		t.Fatalf("expected preview to retain user message, got %q", resp4.Contacts[0].LastMessage)
	}
}

func TestSystemNoticeWithUnfinishedTaskStaysUnresolved(t *testing.T) {
	srv, token := setupTestChatServer(t)
	agentRepo := cuanDB.NewMemoryAgentRepository()
	srv.SetAgentRepository(agentRepo)
	if err := agentRepo.CreateAgent(context.Background(), &cuanDB.Agent{
		ID:      cuanDB.AgentIDCuan,
		Name:    "Cuan",
		Role:    "Coordinator",
		Harness: "agy",
		Enabled: true,
		Status:  cuanDB.AgentStatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	roomID := cuanDB.GenerateUUID()
	_ = srv.projectStudioRepo.CreateRoom(context.Background(), &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Beckthe98",
		Topic:     "agent-topic:agent_cuan",
	})

	// Create an unfinished task in review status
	created, err := srv.taskSvc.CreateTask(context.Background(), "system", task.CreateTaskRequest{
		ProjectID:    "default",
		SourceRoomID: roomID,
		Title:        "Unfinished task",
		Instructions: "Investigate issue",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	reviewStatus := task.TaskStatusReview
	if _, err := srv.taskSvc.UpdateTask(context.Background(), created.ID, "system", task.UpdateTaskRequest{Status: &reviewStatus}); err != nil {
		t.Fatalf("update task status: %v", err)
	}

	// Recent messages are all system lifecycle/review notices
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
			ID:         cuanDB.GenerateUUID(),
			RoomID:     roomID,
			AuthorType: "system",
			AuthorID:   "cuan_work_review",
			AuthorName: "Cuan",
			Content:    "Review the result for [Unfinished task](/tasks/" + created.ID + ").",
			CreatedAt:  now.Add(time.Duration(i) * time.Second),
			UpdatedAt:  now.Add(time.Duration(i) * time.Second),
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	var found *ChatContact
	for i := range resp.Contacts {
		if resp.Contacts[i].ConversationID == roomID {
			found = &resp.Contacts[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected room %s in contacts", roomID)
	}
	if found.OnlySystemNotices {
		t.Fatalf("expected OnlySystemNotices=false when room has unfinished tasks")
	}
	if found.Resolved {
		t.Fatalf("expected Resolved=false when room has unfinished tasks")
	}
}

func TestCustomerRoomWithTasksAndInternalRecentMessagesVisible(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// Initialize visitor chat session
	initBody, _ := json.Marshal(map[string]any{
		"guest_uuid": "guest-beckthe-visible-1",
		"product":    "dracin",
		"name":       "Beckthe98",
		"email":      "beckthe98@example.com",
		"message":    "dracin masih gak bisa di play videonya",
	})
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(initBody))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)
	if recInit.Code != http.StatusOK {
		t.Fatalf("session init failed: %d", recInit.Code)
	}
	var initResp struct {
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
	}
	_ = json.Unmarshal(recInit.Body.Bytes(), &initResp)
	roomID := initResp.Session.UUID

	// Create an unfinished task in the customer room
	created, err := srv.taskSvc.CreateTask(context.Background(), "system", task.CreateTaskRequest{
		ProjectID:    "default",
		SourceRoomID: roomID,
		Title:        "Investigate player",
		Instructions: "Investigate",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	reviewStatus := task.TaskStatusReview
	if _, err := srv.taskSvc.UpdateTask(context.Background(), created.ID, "system", task.UpdateTaskRequest{Status: &reviewStatus}); err != nil {
		t.Fatalf("update task: %v", err)
	}

	// Saturate the 5-message recent window with internal review messages
	now := time.Now().UTC()
	for i := 0; i < 6; i++ {
		_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
			ID:         cuanDB.GenerateUUID(),
			RoomID:     roomID,
			AuthorType: "system",
			AuthorID:   "cuan_work_review",
			AuthorName: "Cuan",
			Content:    "Review the result for [Investigate player](/tasks/" + created.ID + ").",
			CreatedAt:  now.Add(time.Duration(i+1) * time.Second),
			UpdatedAt:  now.Add(time.Duration(i+1) * time.Second),
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?view=list", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	var found *ChatContact
	for i := range resp.Contacts {
		if resp.Contacts[i].ConversationID == roomID {
			found = &resp.Contacts[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected customer room %s to remain visible in /api/v1/chat/contacts", roomID)
	}
	if !found.HasMessages || found.OnlySystemNotices || found.Resolved {
		t.Fatalf("expected HasMessages=true, OnlySystemNotices=false, Resolved=false, got %+v", *found)
	}
}

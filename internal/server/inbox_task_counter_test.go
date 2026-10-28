package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxTaskCounter_ContactsAndConversationsPayload(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// Create an active agent in agentRepo
	if srv.agentRepo != nil {
		_ = srv.agentRepo.CreateAgent(context.Background(), &db.Agent{
			ID:       "agent_dev",
			Name:     "@dev",
			Harness:  "agy",
			Provider: "agy",
			Enabled:  true,
			Status:   db.AgentStatusActive,
		})
	}

	// 1. Initial contacts check: each contact should have task_count and active_tasks initialized to 0
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/v1/chat/contacts, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Contacts      []ChatContact `json:"contacts"`
		Conversations []ChatContact `json:"conversations"`
		Total         int           `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal contacts response: %v", err)
	}

	if len(resp.Contacts) == 0 {
		t.Fatalf("expected at least 1 agent contact")
	}
	if len(resp.Conversations) != len(resp.Contacts) {
		t.Fatalf("expected conversations list to mirror contacts, got %d vs %d", len(resp.Conversations), len(resp.Contacts))
	}

	firstContact := resp.Contacts[0]
	targetRoomID := firstContact.ConversationID
	if targetRoomID == "" {
		t.Fatalf("expected contact to have non-empty ConversationID")
	}

	// 2. Test /api/inbox/conversations endpoint
	reqConv := httptest.NewRequest(http.MethodGet, "/api/inbox/conversations", nil)
	reqConv.Header.Set("Authorization", "Bearer "+token)
	recConv := httptest.NewRecorder()
	srv.Router().ServeHTTP(recConv, reqConv)

	if recConv.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/inbox/conversations, got %d: %s", recConv.Code, recConv.Body.String())
	}

	var convResp struct {
		Conversations []ChatContact `json:"conversations"`
		Contacts      []ChatContact `json:"contacts"`
		Total         int           `json:"total"`
	}
	if err := json.Unmarshal(recConv.Body.Bytes(), &convResp); err != nil {
		t.Fatalf("failed to unmarshal /api/inbox/conversations response: %v", err)
	}
	if len(convResp.Conversations) == 0 {
		t.Fatalf("expected conversations to be returned from /api/inbox/conversations")
	}

	// 3. Create an active task linked to targetRoomID
	createdTask, err := srv.taskSvc.CreateTask(context.Background(), "owner", task.CreateTaskRequest{
		SourceRoomID: targetRoomID,
		ProjectID:    "default",
		Title:        "Investigate inbox task badge counter",
		Instructions: "Ensure task counter displays on conversation items",
		Status:       task.TaskStatusBacklog,
	})
	if err != nil {
		t.Fatalf("failed to create linked task: %v", err)
	}

	// 4. Verify /api/v1/chat/contacts now reports task_count=1 and active_tasks=1
	recAfter := httptest.NewRecorder()
	reqAfter := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts", nil)
	reqAfter.Header.Set("Authorization", "Bearer "+token)
	srv.Router().ServeHTTP(recAfter, reqAfter)

	var respAfter struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(recAfter.Body.Bytes(), &respAfter)

	var targetContact *ChatContact
	for i := range respAfter.Contacts {
		if respAfter.Contacts[i].ConversationID == targetRoomID {
			targetContact = &respAfter.Contacts[i]
			break
		}
	}
	if targetContact == nil {
		t.Fatalf("target contact with ConversationID %s not found", targetRoomID)
	}
	if targetContact.TaskCount != 1 {
		t.Errorf("expected task_count to be 1, got %d", targetContact.TaskCount)
	}
	if targetContact.ActiveTasks != 1 {
		t.Errorf("expected active_tasks to be 1, got %d", targetContact.ActiveTasks)
	}

	// 5. Test conversation messages endpoint payload (/api/v1/chat/conversations/{id}/messages)
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+targetRoomID+"/messages", nil)
	reqDetail.Header.Set("Authorization", "Bearer "+token)
	recDetail := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDetail, reqDetail)

	if recDetail.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for conversation messages, got %d: %s", recDetail.Code, recDetail.Body.String())
	}
	var detailResp struct {
		TaskCount   int         `json:"task_count"`
		ActiveTasks int         `json:"active_tasks"`
		Contact     ChatContact `json:"contact"`
		Tasks       []any       `json:"tasks"`
	}
	_ = json.Unmarshal(recDetail.Body.Bytes(), &detailResp)
	if detailResp.TaskCount != 1 {
		t.Errorf("expected conversation detail task_count=1, got %d", detailResp.TaskCount)
	}
	if detailResp.ActiveTasks != 1 {
		t.Errorf("expected conversation detail active_tasks=1, got %d", detailResp.ActiveTasks)
	}
	if detailResp.Contact.TaskCount != 1 || detailResp.Contact.ActiveTasks != 1 {
		t.Errorf("expected contact in detail to have task_count=1, active_tasks=1, got %d, %d",
			detailResp.Contact.TaskCount, detailResp.Contact.ActiveTasks)
	}

	// 6. Complete the task: backlog -> review -> done
	reviewStatus := task.TaskStatusReview
	completeNote := "Verified task completion evidence"
	_, err = srv.taskSvc.UpdateTask(context.Background(), createdTask.ID, "owner", task.UpdateTaskRequest{
		Status:       &reviewStatus,
		CompleteNote: &completeNote,
	})
	if err != nil {
		t.Fatalf("failed to move task to review: %v", err)
	}

	doneStatus := task.TaskStatusDone
	_, err = srv.taskSvc.UpdateTask(context.Background(), createdTask.ID, "owner", task.UpdateTaskRequest{
		Status:       &doneStatus,
		CompleteNote: &completeNote,
	})
	if err != nil {
		t.Fatalf("failed to mark task as done: %v", err)
	}

	// 7. Verify active_tasks becomes 0 while task_count remains 1
	recDone := httptest.NewRecorder()
	reqDone := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts", nil)
	reqDone.Header.Set("Authorization", "Bearer "+token)
	srv.Router().ServeHTTP(recDone, reqDone)

	var respDone struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(recDone.Body.Bytes(), &respDone)
	for _, c := range respDone.Contacts {
		if c.ConversationID == targetRoomID {
			if c.TaskCount != 1 {
				t.Errorf("expected task_count to remain 1 after completion, got %d", c.TaskCount)
			}
			if c.ActiveTasks != 0 {
				t.Errorf("expected active_tasks to become 0 after completion, got %d", c.ActiveTasks)
			}
			if c.DoneTasks != 1 {
				t.Errorf("expected done_tasks to become 1 after completion, got %d", c.DoneTasks)
			}
			break
		}
	}
}

func TestInboxTaskCounter_ProjectStudioRepoFallback(t *testing.T) {
	srv, _ := setupTestInboxServer(t)

	// Admin login to get authToken
	loginBody := []byte(`{"username":"admin","password":"whosyourdaddy"}`)
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	srv.Router().ServeHTTP(recLogin, reqLogin)
	if recLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin login, got %d: %s", recLogin.Code, recLogin.Body.String())
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(recLogin.Body.Bytes(), &loginResp)
	authToken := loginResp.Token

	// Create room and task directly in projectStudioRepo
	roomID := db.GenerateUUID()
	_ = srv.projectStudioRepo.CreateRoom(context.Background(), &db.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Customer Query Room",
		Topic:     "Inquiry",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	_ = srv.projectStudioRepo.CreateTask(context.Background(), &db.ProjectTask{
		ID:        db.GenerateUUID(),
		ProjectID: "default",
		RoomID:    &roomID,
		Title:     "Direct Studio Task",
		Status:    "doing",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	// GET /api/v1/inbox
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inbox", nil)
	req.Header.Set("Authorization", "Bearer "+authToken)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/v1/inbox, got %d: %s", rec.Code, rec.Body.String())
	}

	var listResp struct {
		Threads []InboxThreadItem `json:"threads"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	var found *InboxThreadItem
	for i := range listResp.Threads {
		if listResp.Threads[i].ID == roomID {
			found = &listResp.Threads[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected to find room %s in inbox threads", roomID)
	}
	if found.TaskCount != 1 {
		t.Errorf("expected thread task_count=1, got %d", found.TaskCount)
	}
	if found.ActiveTasks != 1 {
		t.Errorf("expected thread active_tasks=1, got %d", found.ActiveTasks)
	}
}

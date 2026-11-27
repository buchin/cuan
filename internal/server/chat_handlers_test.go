package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
)

func setupTestChatServer(t *testing.T) (*Server, string) {
	cfg := &config.Config{
		Environment: "testing",
		ServerPort:  "8080",
		AuthSecret:  "test-secret-key-32-chars-long!",
	}

	authStore := auth.NewMemoryStore()
	authSvc := auth.NewService(authStore)
	if err := authSvc.AutoSeedDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("failed to auto seed admin: %v", err)
	}

	studioRepo := cuanDB.NewMemoryProjectStudioRepository()

	srv := NewServer(cfg, nil, authSvc)
	srv.SetProjectStudioRepository(studioRepo)

	// Authenticate admin to get token
	loginBody := []byte(`{"username":"admin","password":"whosyourdaddy"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)

	return srv, resp.Token
}

func TestChatHandlers_CompleteFlow(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Initial Contacts (Default Scrum Master & Antigravity Agent)
	reqContacts := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts", nil)
	reqContacts.Header.Set("Authorization", "Bearer "+token)
	recContacts := httptest.NewRecorder()
	srv.Router().ServeHTTP(recContacts, reqContacts)

	if recContacts.Code != http.StatusOK {
		t.Fatalf("expected 200 for contacts, got %d: %s", recContacts.Code, recContacts.Body.String())
	}

	var contactsResp struct {
		Contacts []ChatContact `json:"contacts"`
		Total    int           `json:"total"`
	}
	if err := json.Unmarshal(recContacts.Body.Bytes(), &contactsResp); err != nil {
		t.Fatalf("failed to unmarshal contacts: %v", err)
	}

	for _, c := range contactsResp.Contacts {
		if c.Type != "agent" {
			t.Fatalf("expected no visitor contacts initially: %+v", c)
		}
	}

	// 2. Visitor Live Chat Initialization (POST /api/chat/session)
	visitorInitPayload := map[string]interface{}{
		"guest_uuid":   "guest-uuid-1234",
		"product":      "shuriken",
		"name":         "Budi Santoso",
		"email":        "budi@example.com",
		"current_url":  "https://shuriken.dojo.cc/checkout",
		"page_title":   "Shuriken Checkout",
		"referrer_url": "https://google.com",
		"message":      "Halo, apakah lisensi shuriken support multi-domain?",
	}
	initBody, _ := json.Marshal(visitorInitPayload)
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(initBody))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)

	if recInit.Code != http.StatusOK {
		t.Fatalf("expected 200 for visitor chat session init, got %d: %s", recInit.Code, recInit.Body.String())
	}

	var initResp struct {
		OK      bool `json:"ok"`
		Session struct {
			UUID      string `json:"uuid"`
			GuestUUID string `json:"guest_uuid"`
			Product   string `json:"product"`
			Name      string `json:"name"`
		} `json:"session"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recInit.Body.Bytes(), &initResp); err != nil || !initResp.OK {
		t.Fatalf("invalid init session response: %v", err)
	}
	sessionUUID := initResp.Session.UUID
	if sessionUUID == "" {
		t.Fatalf("expected non-empty session UUID")
	}
	if len(initResp.Messages) != 1 || initResp.Messages[0].Message != "Halo, apakah lisensi shuriken support multi-domain?" {
		t.Fatalf("expected initial message in session, got %+v", initResp.Messages)
	}

	// Check CORS header
	if cors := recInit.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Fatalf("expected CORS *, got %s", cors)
	}

	// 3. Visitor Page Telemetry (POST /api/chat/sessions/{uuid}/page)
	telemetryPayload := map[string]interface{}{
		"current_url": "https://shuriken.dojo.cc/faq",
		"page_title":  "Shuriken FAQ",
		"product":     "shuriken",
	}
	telemetryBody, _ := json.Marshal(telemetryPayload)
	reqTel := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionUUID+"/page", bytes.NewReader(telemetryBody))
	reqTel.Header.Set("Content-Type", "application/json")
	recTel := httptest.NewRecorder()
	srv.Router().ServeHTTP(recTel, reqTel)
	if recTel.Code != http.StatusOK {
		t.Fatalf("expected 200 for page telemetry, got %d", recTel.Code)
	}

	// 4. Contacts List Now Shows Customer Contact
	reqContacts2 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=customers", nil)
	reqContacts2.Header.Set("Authorization", "Bearer "+token)
	recContacts2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(recContacts2, reqContacts2)
	if recContacts2.Code != http.StatusOK {
		t.Fatalf("expected 200 for customer contacts, got %d", recContacts2.Code)
	}
	var custContactsResp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(recContacts2.Body.Bytes(), &custContactsResp)
	if len(custContactsResp.Contacts) == 0 {
		t.Fatalf("expected at least 1 customer contact, got 0")
	}
	custContact := custContactsResp.Contacts[0]
	if custContact.Name != "Budi Santoso" || custContact.Product != "shuriken" {
		t.Fatalf("unexpected customer contact info: %+v", custContact)
	}
	if !custContact.HasMessages || custContact.MessageCount != 1 || !custContact.IsOnline {
		t.Fatalf("expected HasMessages=true, MessageCount=1, IsOnline=true, got %+v", custContact)
	}
	if custContact.PageTitle != "Shuriken FAQ" {
		t.Fatalf("expected PageTitle 'Shuriken FAQ', got %s", custContact.PageTitle)
	}

	// 5. Operator Replies from Direct Messenger Hub (POST /api/v1/chat/conversations/{id}/messages)
	operatorReplyPayload := map[string]interface{}{
		"message":     "Halo Mas Budi! Betul, lisensi Shuriken Lifetime mencakup pemakaian multi-domain.",
		"author_type": "user",
		"author_name": "You",
	}
	replyBody, _ := json.Marshal(operatorReplyPayload)
	reqReply := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+sessionUUID+"/messages", bytes.NewReader(replyBody))
	reqReply.Header.Set("Content-Type", "application/json")
	reqReply.Header.Set("Authorization", "Bearer "+token)
	recReply := httptest.NewRecorder()
	srv.Router().ServeHTTP(recReply, reqReply)
	if recReply.Code != http.StatusOK {
		t.Fatalf("expected 200 for operator reply, got %d: %s", recReply.Code, recReply.Body.String())
	}

	// 5b. Verify Conversation Messages for Inbox UI have operator message marked as operator
	reqConvMsgs := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+sessionUUID+"/messages", nil)
	reqConvMsgs.Header.Set("Authorization", "Bearer "+token)
	recConvMsgs := httptest.NewRecorder()
	srv.Router().ServeHTTP(recConvMsgs, reqConvMsgs)
	if recConvMsgs.Code != http.StatusOK {
		t.Fatalf("expected 200 for conversation messages, got %d", recConvMsgs.Code)
	}
	var convMsgsResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recConvMsgs.Body.Bytes(), &convMsgsResp); err != nil {
		t.Fatalf("failed to decode conv msgs: %v", err)
	}
	if len(convMsgsResp.Messages) < 2 {
		t.Fatalf("expected at least 2 messages in conv, got %d", len(convMsgsResp.Messages))
	}
	visitorMsg := convMsgsResp.Messages[0]
	if visitorMsg.SenderType != "visitor" {
		t.Fatalf("expected visitor sender_type 'visitor', got '%s'", visitorMsg.SenderType)
	}
	operatorMsg := convMsgsResp.Messages[1]
	if operatorMsg.SenderType != "operator" {
		t.Fatalf("expected operator sender_type 'operator', got '%s'", operatorMsg.SenderType)
	}
	if operatorMsg.AuthorID != "operator_user" {
		t.Fatalf("expected operator author_id 'operator_user', got '%s'", operatorMsg.AuthorID)
	}
	if operatorMsg.AuthorName != "You" {
		t.Fatalf("expected operator author_name 'You', got '%s'", operatorMsg.AuthorName)
	}

	// 6. Visitor Polls New Messages (GET /api/chat/messages?session_uuid=...&after_id=1)
	reqPoll := httptest.NewRequest(http.MethodGet, "/api/chat/messages?session_uuid="+sessionUUID+"&after_id=1", nil)
	recPoll := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPoll, reqPoll)
	if recPoll.Code != http.StatusOK {
		t.Fatalf("expected 200 for visitor polling, got %d", recPoll.Code)
	}
	var pollResp struct {
		OK       bool             `json:"ok"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recPoll.Body.Bytes(), &pollResp); err != nil || !pollResp.OK {
		t.Fatalf("invalid poll response: %v", err)
	}
	if len(pollResp.Messages) != 1 {
		t.Fatalf("expected 1 new message for visitor, got %d", len(pollResp.Messages))
	}
	if pollResp.Messages[0].SenderType != "operator" || !strings.Contains(pollResp.Messages[0].Message, "multi-domain") {
		t.Fatalf("unexpected operator message in visitor poll: %+v", pollResp.Messages[0])
	}
	if pollResp.Messages[0].AuthorID != "operator_user" {
		t.Fatalf("expected poll message author_id 'operator_user', got '%s'", pollResp.Messages[0].AuthorID)
	}

	// 8. Test /chat/widget.js Serving
	reqWidget := httptest.NewRequest(http.MethodGet, "/chat/widget.js", nil)
	recWidget := httptest.NewRecorder()
	srv.Router().ServeHTTP(recWidget, reqWidget)
	if recWidget.Code != http.StatusOK {
		t.Fatalf("expected 200 for /chat/widget.js, got %d", recWidget.Code)
	}
	if ct := recWidget.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("expected javascript content type, got %s", ct)
	}
	if cors := recWidget.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Fatalf("expected CORS *, got %s", cors)
	}
}

func TestChatHandlers_NoSyntheticAgentReplies(t *testing.T) {
	srv, token := setupTestChatServer(t)
	for _, id := range []string{defaultScrumMasterRoomID, defaultAntigravityRoomID, "@unknown"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+id+"/messages", strings.NewReader(`{"message":"Halo @unknown"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected saved user message, got %d", rec.Code)
		}
		var response struct {
			AgentReply *ChatMessageDTO `json:"agent_reply"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.AgentReply != nil {
			t.Fatal("unregistered agent fabricated reply")
		}
	}
}

func TestChatHandlers_DynamicRoles(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// Register a BYO agent "@dev"
	agentRepo := cuanDB.NewMemoryAgentRepository()
	srv.SetAgentRepository(agentRepo)

	createdAgent := &cuanDB.Agent{
		ID:           cuanDB.GenerateUUID(),
		Name:         "@dev",
		Harness:      "agy",
		ConnectionID: "conn_test_dev",
		Enabled:      true,
		Provider:     "agy",
		Status:       cuanDB.AgentStatusActive,
	}
	if err := agentRepo.CreateAgent(context.Background(), createdAgent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// 1. Test Context Classifier directly
	tests := []struct {
		input string
		want  ChatContextRole
	}{
		{"Halo apa kabar @dev?", RoleFriendly},
		{"Selamat pagi tim!", RoleFriendly},
		{"Terima kasih atas bantuannya", RoleFriendly},
		{"Tolong evaluasi sprint 23 dan review backlog", RoleScrumMaster},
		{"Bagaimana DoD untuk user story ini?", RoleScrumMaster},
		{"Mari jadwalkan sprint planning dan koordinasi tim", RoleScrumMaster},
		{"Tolong fix bug crash di endpoint api pembayaran", RoleDeveloper},
		{"Bisa tolong buatkan unit test dan refactor query database?", RoleDeveloper},
		{"Siapkan script migration untuk tabel baru", RoleDeveloper},
	}

	for _, tt := range tests {
		got := ClassifyMessageContext(tt.input)
		if got != tt.want {
			t.Errorf("ClassifyMessageContext(%q) = %v; want %v", tt.input, got, tt.want)
		}
	}

	// An unavailable provider must not produce canned persona text.
	for _, message := range []string{"@dev evaluasi sprint dan review", "Tolong fix bug database", "Halo @dev"} {
		body, _ := json.Marshal(map[string]string{"message": message})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+createdAgent.ID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected provider failure, got %d: %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "agent_reply") {
			t.Fatal("failure fabricated an agent reply")
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+createdAgent.ID+"/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	var history struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Messages) != 3 {
		t.Fatalf("expected three saved user messages, got %d", len(history.Messages))
	}
	for _, message := range history.Messages {
		if message.AuthorType == "agent" {
			t.Fatal("unavailable provider reply persisted")
		}
	}
}

func TestS25InboxOnlyRegisteredAgents(t *testing.T) {
	srv, token := setupTestChatServer(t)
	repo := cuanDB.NewMemoryAgentRepository()
	srv.SetAgentRepository(repo)
	if err := repo.CreateAgent(context.Background(), &cuanDB.Agent{ID: "agent_real", Name: "@real", Harness: "agy", Enabled: true, Status: cuanDB.AgentStatusActive}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	var response struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Contacts) != 1 || response.Contacts[0].Name != "@real" {
		t.Fatalf("unexpected contacts: %+v", response.Contacts)
	}
	resolved := srv.resolveConversationContext(context.Background(), response.Contacts[0].ConversationID, "")
	if !resolved.IsAgent || resolved.TargetAgentID != "agent_real" {
		t.Fatal("registered Inbox agent not resolved")
	}
	if err := repo.DeleteAgent(context.Background(), "agent_real"); err != nil {
		t.Fatal(err)
	}
	resolved = srv.resolveConversationContext(context.Background(), response.Contacts[0].ConversationID, "")
	if resolved.IsAgent {
		t.Fatal("deleted agent still active through its old Inbox URL")
	}
}

func TestChatHandlers_Pagination(t *testing.T) {
	srv, token := setupTestChatServer(t)
	roomID := cuanDB.GenerateUUID()

	// 1. Send 5 messages into this room
	for i := 1; i <= 5; i++ {
		body := fmt.Sprintf(`{"message":"Message number %d"}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("failed to post message %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}

	// 2. Fetch with limit=2 (should return messages 4 and 5, has_more=true)
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+roomID+"/messages?limit=2", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	rec1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("failed to get messages: %d %s", rec1.Code, rec1.Body.String())
	}
	var page1 struct {
		Messages []ChatMessageDTO `json:"messages"`
		HasMore  bool             `json:"has_more"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &page1); err != nil {
		t.Fatal(err)
	}
	if len(page1.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(page1.Messages))
	}
	if !page1.HasMore {
		t.Fatalf("expected has_more=true for page 1")
	}
	if page1.Messages[0].Message != "Message number 4" || page1.Messages[1].Message != "Message number 5" {
		t.Fatalf("unexpected page 1 messages: %+v", page1.Messages)
	}

	// 3. Fetch earlier 2 with before_id = message 4's UUID (should return messages 2 and 3, has_more=true)
	oldestUUID := page1.Messages[0].UUID
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+roomID+"/messages?limit=2&before_id="+oldestUUID, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)
	var page2 struct {
		Messages []ChatMessageDTO `json:"messages"`
		HasMore  bool             `json:"has_more"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Messages) != 2 {
		t.Fatalf("expected 2 messages in page 2, got %d", len(page2.Messages))
	}
	if !page2.HasMore {
		t.Fatalf("expected has_more=true for page 2")
	}
	if page2.Messages[0].Message != "Message number 2" || page2.Messages[1].Message != "Message number 3" {
		t.Fatalf("unexpected page 2 messages: %+v", page2.Messages)
	}

	// 4. Fetch earliest with before_id = message 2's UUID (should return message 1, has_more=false)
	oldestUUID2 := page2.Messages[0].UUID
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+roomID+"/messages?limit=2&before_id="+oldestUUID2, nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	rec3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec3, req3)
	var page3 struct {
		Messages []ChatMessageDTO `json:"messages"`
		HasMore  bool             `json:"has_more"`
	}
	if err := json.Unmarshal(rec3.Body.Bytes(), &page3); err != nil {
		t.Fatal(err)
	}
	if len(page3.Messages) != 1 {
		t.Fatalf("expected 1 message in page 3, got %d", len(page3.Messages))
	}
	if page3.HasMore {
		t.Fatalf("expected has_more=false for page 3")
	}
	if page3.Messages[0].Message != "Message number 1" {
		t.Fatalf("unexpected page 3 messages: %+v", page3.Messages)
	}

	// 5. Fetch all messages (limit=all)
	reqAll := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+roomID+"/messages?limit=all", nil)
	reqAll.Header.Set("Authorization", "Bearer "+token)
	recAll := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAll, reqAll)
	var pageAll struct {
		Messages []ChatMessageDTO `json:"messages"`
		HasMore  bool             `json:"has_more"`
	}
	if err := json.Unmarshal(recAll.Body.Bytes(), &pageAll); err != nil {
		t.Fatal(err)
	}
	if len(pageAll.Messages) != 5 || pageAll.HasMore {
		t.Fatalf("expected 5 messages and has_more=false for limit=all, got %d (has_more=%v)", len(pageAll.Messages), pageAll.HasMore)
	}
}

func TestInboxUnreadCountAndSystemEvents(t *testing.T) {
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

	// 1. Initial list contacts -> UnreadCount should be 0
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
	if resp.Contacts[0].UnreadCount != 0 {
		t.Fatalf("expected 0 unread initially, got %d", resp.Contacts[0].UnreadCount)
	}

	roomID := resp.Contacts[0].ConversationID

	// 2. Add an incoming Agent message and a System event message
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   "agent_scrum",
		AuthorName: "Scrum Master",
		Content:    "Halo, sprint baru telah dimulai.",
	})
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "system",
		AuthorID:   "system",
		AuthorName: "System Alert",
		Content:    "⚠️ System Notification: Build deployed successfully.",
	})

	// 3. Fetch contacts again -> UnreadCount should now be 2
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)

	var resp2 struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if resp2.Contacts[0].UnreadCount != 2 {
		t.Fatalf("expected 2 unread (1 agent + 1 system), got %d", resp2.Contacts[0].UnreadCount)
	}

	// 4. User replies -> UnreadCount should reset to 0
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "admin",
		AuthorName: "admin",
		Content:    "Baik, saya monitor.",
	})

	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	rec3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec3, req3)

	var resp3 struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(rec3.Body.Bytes(), &resp3)
	if resp3.Contacts[0].UnreadCount != 0 {
		t.Fatalf("expected 0 unread after user message, got %d", resp3.Contacts[0].UnreadCount)
	}

	// 5. Subsequent System message -> UnreadCount becomes 1
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "system",
		AuthorID:   "system",
		AuthorName: "System Alert",
		Content:    "ℹ️ Task execution completed.",
	})

	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	rec4 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec4, req4)

	var resp4 struct {
		Contacts []ChatContact `json:"contacts"`
	}
	_ = json.Unmarshal(rec4.Body.Bytes(), &resp4)
	if resp4.Contacts[0].UnreadCount != 1 {
		t.Fatalf("expected 1 unread after new system event, got %d", resp4.Contacts[0].UnreadCount)
	}
}

func TestChatHandlers_WhisperMentionAndDraftReply(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Visitor initializes chat session
	sessionPayload := map[string]string{
		"guest_uuid": cuanDB.GenerateUUID(),
		"name":       "Visitor Joko",
		"email":      "joko@example.com",
		"product":    "Dojo Store",
		"message":    "Min, tombol checkout error 500 saat diklik.",
	}
	bSession, _ := json.Marshal(sessionPayload)
	reqSession := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(bSession))
	recSession := httptest.NewRecorder()
	srv.Router().ServeHTTP(recSession, reqSession)
	if recSession.Code != http.StatusOK {
		t.Fatalf("expected 200 for visitor session, got %d", recSession.Code)
	}

	var sessResp struct {
		OK      bool             `json:"ok"`
		Session *LiveChatSession `json:"session"`
	}
	_ = json.Unmarshal(recSession.Body.Bytes(), &sessResp)
	sessionUUID := sessResp.Session.UUID

	// 2. Operator sends internal whisper mention: @team
	whisperPayload := map[string]interface{}{
		"message":     "@team tolong bantu fix error 500 di tombol checkout ini",
		"author_type": "user",
		"author_name": "Support Lead",
	}
	bWhisper, _ := json.Marshal(whisperPayload)
	reqWhisper := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+sessionUUID+"/messages", bytes.NewReader(bWhisper))
	reqWhisper.Header.Set("Authorization", "Bearer "+token)
	reqWhisper.Header.Set("Content-Type", "application/json")
	recWhisper := httptest.NewRecorder()
	srv.Router().ServeHTTP(recWhisper, reqWhisper)
	if recWhisper.Code != http.StatusOK && recWhisper.Code != http.StatusAccepted && recWhisper.Code != http.StatusBadGateway {
		t.Fatalf("expected 200, 202 or 502 for whisper message, got %d: %s", recWhisper.Code, recWhisper.Body.String())
	}

	// 3. Operator fetches internal conversation messages
	reqInternal := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+sessionUUID+"/messages", nil)
	reqInternal.Header.Set("Authorization", "Bearer "+token)
	recInternal := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInternal, reqInternal)
	if recInternal.Code != http.StatusOK {
		t.Fatalf("expected 200 for internal messages, got %d", recInternal.Code)
	}
	var internalResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	_ = json.Unmarshal(recInternal.Body.Bytes(), &internalResp)

	var foundWhisper *ChatMessageDTO
	for _, m := range internalResp.Messages {
		if strings.Contains(m.Message, "@team") {
			mCopy := m
			foundWhisper = &mCopy
			break
		}
	}
	if foundWhisper == nil {
		t.Fatalf("expected whisper message with @team in internal messages")
	}
	if !foundWhisper.IsInternal {
		t.Fatalf("expected whisper message to be marked is_internal=true")
	}
	if foundWhisper.MentionHandle != "@team" {
		t.Fatalf("expected mention_handle to be @team, got: %s", foundWhisper.MentionHandle)
	}

	// 4. Visitor polls messages: MUST NOT receive the whisper
	reqVisPoll := httptest.NewRequest(http.MethodGet, "/api/chat/messages?session_uuid="+sessionUUID+"&after_id=1", nil)
	recVisPoll := httptest.NewRecorder()
	srv.Router().ServeHTTP(recVisPoll, reqVisPoll)
	if recVisPoll.Code != http.StatusOK {
		t.Fatalf("expected 200 for visitor polling, got %d", recVisPoll.Code)
	}
	var visPollResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	_ = json.Unmarshal(recVisPoll.Body.Bytes(), &visPollResp)
	for _, m := range visPollResp.Messages {
		if strings.Contains(m.Message, "@team") || m.IsInternal {
			t.Fatalf("visitor received internal whisper message: %+v", m)
		}
	}

	// 5. Test ready-to-use customer draft reply extraction
	agentReplyContent := `Here is a proposed plan for your request:

**Problem Statement:**
Fix checkout 500 error

**Execution plan & definition of done:**
1. Investigate error
2. Fix checkout handler
3. Run test

**Draf Balasan untuk Pengunjung:**
> "Halo Kak, terima kasih sudah menunggu. Kendala checkout sudah kami perbaiki dan sudah live kembali. Silakan dicoba ya kak! 🙏"

**Recommended worker agent:** antigravity-flash.`

	draft := extractCustomerReplyDraft(agentReplyContent)
	if !strings.Contains(draft, "Kendala checkout sudah kami perbaiki") {
		t.Fatalf("expected draft to be extracted, got: %q", draft)
	}

	// Inject SM message with draft
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     sessionUUID,
		AuthorType: "agent",
		AuthorID:   "agent_sm",
		AuthorName: "Scrum Master",
		Content:    agentReplyContent,
	})

	// Fetch internal messages again to verify ReplyDraft field
	recInternal2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInternal2, reqInternal)
	_ = json.Unmarshal(recInternal2.Body.Bytes(), &internalResp)
	var foundAgentReply *ChatMessageDTO
	for _, m := range internalResp.Messages {
		if m.AuthorType == "agent" && strings.Contains(m.Message, "Here is a proposed plan") {
			mCopy := m
			foundAgentReply = &mCopy
			break
		}
	}
	if foundAgentReply == nil {
		t.Fatalf("expected agent reply in internal messages")
	}
	if !foundAgentReply.IsInternal {
		t.Fatalf("expected agent reply in customer room to be is_internal=true")
	}
	if foundAgentReply.ReplyDraft == "" || !strings.Contains(foundAgentReply.ReplyDraft, "Kendala checkout sudah kami perbaiki") {
		t.Fatalf("expected foundAgentReply.ReplyDraft to be populated, got: %q", foundAgentReply.ReplyDraft)
	}

	// Visitor MUST NOT receive the agent's internal DoD reply
	recVisPoll2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(recVisPoll2, reqVisPoll)
	_ = json.Unmarshal(recVisPoll2.Body.Bytes(), &visPollResp)
	for _, m := range visPollResp.Messages {
		if m.AuthorType == "agent" || m.IsInternal {
			t.Fatalf("visitor received agent internal reply: %+v", m)
		}
	}

	// 6. Operator uses the draft to reply publicly to visitor
	pubReplyPayload := map[string]interface{}{
		"message":     foundAgentReply.ReplyDraft,
		"author_type": "user",
		"author_name": "Support Lead",
	}
	bPub, _ := json.Marshal(pubReplyPayload)
	reqPub := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+sessionUUID+"/messages", bytes.NewReader(bPub))
	reqPub.Header.Set("Authorization", "Bearer "+token)
	reqPub.Header.Set("Content-Type", "application/json")
	recPub := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPub, reqPub)

	// Visitor polls again: MUST receive the public reply!
	recVisPoll3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(recVisPoll3, reqVisPoll)
	_ = json.Unmarshal(recVisPoll3.Body.Bytes(), &visPollResp)
	var visitorReceivedReply bool
	for _, m := range visPollResp.Messages {
		if strings.Contains(m.Message, "Kendala checkout sudah kami perbaiki") {
			visitorReceivedReply = true
			if m.IsInternal {
				t.Fatalf("visitor public reply must not be internal")
			}
		}
	}
	if !visitorReceivedReply {
		t.Fatalf("expected visitor to receive public reply drafted by Scrum Master")
	}
}

func TestVisitorChatSession_FiltersInternalMessages(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Visitor initializes live chat session
	guestUUID := "guest-whisper-test-1"
	visitorInitPayload := map[string]interface{}{
		"guest_uuid": guestUUID,
		"product":    "shuriken",
		"name":       "Test Visitor",
		"email":      "visitor@example.com",
		"message":    "Halo, mau tanya harga",
	}
	initBody, _ := json.Marshal(visitorInitPayload)
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(initBody))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)

	if recInit.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recInit.Code, recInit.Body.String())
	}

	var initResp struct {
		OK      bool `json:"ok"`
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recInit.Body.Bytes(), &initResp); err != nil || !initResp.OK {
		t.Fatalf("failed to decode response: %v", err)
	}
	sessionUUID := initResp.Session.UUID

	ctx := context.Background()
	now := time.Now().UTC()

	// 2. Staff writes internal messages to the room:
	// a. Whisper message with @team mention
	if err := srv.projectStudioRepo.CreateMessage(ctx, &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     sessionUUID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		AuthorName: "Owner",
		Content:    "@team Mohon jangan berikan diskon lebih dari 10%",
		CreatedAt:  now.Add(1 * time.Second),
		UpdatedAt:  now.Add(1 * time.Second),
	}); err != nil {
		t.Fatalf("failed to create whisper message: %v", err)
	}

	// b. Internal bot / agent response
	if err := srv.projectStudioRepo.CreateMessage(ctx, &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     sessionUUID,
		AuthorType: "agent",
		AuthorID:   "agent_sm",
		AuthorName: "Scrum Master",
		Content:    "Rahasia dapur sistem: planning sprint aktif",
		CreatedAt:  now.Add(2 * time.Second),
		UpdatedAt:  now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("failed to create agent message: %v", err)
	}

	// c. Internal system note
	if err := srv.projectStudioRepo.CreateMessage(ctx, &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     sessionUUID,
		AuthorType: "system",
		AuthorID:   "system_notifier",
		AuthorName: "System",
		Content:    "Internal note: Session assigned to Level 2 support",
		CreatedAt:  now.Add(3 * time.Second),
		UpdatedAt:  now.Add(3 * time.Second),
	}); err != nil {
		t.Fatalf("failed to create system message: %v", err)
	}

	// 3. Staff writes a public reply to the visitor
	if err := srv.projectStudioRepo.CreateMessage(ctx, &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     sessionUUID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		AuthorName: "Support Agent",
		Content:    "Halo! Harga lisensi kami Rp 500.000",
		CreatedAt:  now.Add(4 * time.Second),
		UpdatedAt:  now.Add(4 * time.Second),
	}); err != nil {
		t.Fatalf("failed to create public message: %v", err)
	}

	// 4. Verify internal conversation endpoint DOES show the internal messages
	reqInternal := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+sessionUUID+"/messages", nil)
	reqInternal.Header.Set("Authorization", "Bearer "+token)
	recInternal := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInternal, reqInternal)
	if recInternal.Code != http.StatusOK {
		t.Fatalf("failed to get internal messages: %d", recInternal.Code)
	}
	var internalResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	_ = json.Unmarshal(recInternal.Body.Bytes(), &internalResp)
	var hasWhisperInternal, hasAgentInternal, hasSystemInternal bool
	for _, m := range internalResp.Messages {
		if strings.Contains(m.Content, "Mohon jangan berikan diskon") && m.IsInternal {
			hasWhisperInternal = true
		}
		if strings.Contains(m.Content, "Rahasia dapur sistem") && m.IsInternal {
			hasAgentInternal = true
		}
		if strings.Contains(m.Content, "Level 2 support") && m.IsInternal {
			hasSystemInternal = true
		}
	}
	if !hasWhisperInternal || !hasAgentInternal || !hasSystemInternal {
		t.Fatalf("expected internal endpoint to contain all internal messages (whisper=%v agent=%v system=%v)",
			hasWhisperInternal, hasAgentInternal, hasSystemInternal)
	}

	// 5. Visitor refreshes widget / re-initializes session: POST /api/chat/session
	refreshPayload := map[string]interface{}{
		"guest_uuid":   guestUUID,
		"session_uuid": sessionUUID,
		"product":      "shuriken",
	}
	bRefresh, _ := json.Marshal(refreshPayload)
	reqRefresh := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(bRefresh))
	reqRefresh.Header.Set("Content-Type", "application/json")
	recRefresh := httptest.NewRecorder()
	srv.Router().ServeHTTP(recRefresh, reqRefresh)

	if recRefresh.Code != http.StatusOK {
		t.Fatalf("expected 200 on session refresh, got %d: %s", recRefresh.Code, recRefresh.Body.String())
	}

	var refreshResp struct {
		OK       bool             `json:"ok"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recRefresh.Body.Bytes(), &refreshResp); err != nil {
		t.Fatalf("failed to unmarshal refresh response: %v", err)
	}

	if len(refreshResp.Messages) == 0 {
		t.Fatalf("expected visitor messages to be returned on refresh")
	}

	for _, m := range refreshResp.Messages {
		if m.IsInternal {
			t.Fatalf("LEAK DETECTED: visitor session received internal message: ID=%d Content=%q Author=%s IsInternal=%v",
				m.ID, m.Content, m.AuthorName, m.IsInternal)
		}
		if strings.Contains(m.Content, "Mohon jangan berikan diskon") ||
			strings.Contains(m.Content, "Rahasia dapur") ||
			strings.Contains(m.Content, "Level 2 support") {
			t.Fatalf("LEAK DETECTED: visitor session received internal message text: %q", m.Content)
		}
	}

	// Verify the visitor receives both their own message and the public operator reply
	var foundVisitorMsg, foundPublicReply bool
	for _, m := range refreshResp.Messages {
		if strings.Contains(m.Content, "mau tanya harga") {
			foundVisitorMsg = true
		}
		if strings.Contains(m.Content, "Rp 500.000") {
			foundPublicReply = true
		}
	}
	if !foundVisitorMsg {
		t.Fatalf("expected visitor message in refresh response")
	}
	if !foundPublicReply {
		t.Fatalf("expected public reply in refresh response")
	}
}

func TestChatHandlers_CustomerEmail(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Initialize visitor chat session with email
	visitorInitPayload := map[string]interface{}{
		"guest_uuid":  "guest-uuid-email-test",
		"product":     "shuriken",
		"name":        "Siti Rahma",
		"email":       "siti.rahma@example.com",
		"current_url": "https://shuriken.dojo.cc/pricing",
		"page_title":  "Pricing Page",
		"message":     "Halo, saya ingin menanyakan paket enterprise.",
	}
	initBody, _ := json.Marshal(visitorInitPayload)
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(initBody))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)
	if recInit.Code != http.StatusOK {
		t.Fatalf("expected 200 for visitor chat session init, got %d: %s", recInit.Code, recInit.Body.String())
	}

	var initResp struct {
		OK      bool `json:"ok"`
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
	}
	if err := json.Unmarshal(recInit.Body.Bytes(), &initResp); err != nil || !initResp.OK {
		t.Fatalf("failed to decode init response: %v", err)
	}
	sessionUUID := initResp.Session.UUID

	// 2. Fetch contacts list -> verify email is present in ChatContact
	reqContacts := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=customers", nil)
	reqContacts.Header.Set("Authorization", "Bearer "+token)
	recContacts := httptest.NewRecorder()
	srv.Router().ServeHTTP(recContacts, reqContacts)
	if recContacts.Code != http.StatusOK {
		t.Fatalf("expected 200 for contacts, got %d: %s", recContacts.Code, recContacts.Body.String())
	}
	var contactsResp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(recContacts.Body.Bytes(), &contactsResp); err != nil {
		t.Fatalf("failed to decode contacts response: %v", err)
	}
	var found *ChatContact
	for _, c := range contactsResp.Contacts {
		if c.ConversationID == sessionUUID {
			found = &c
			break
		}
	}
	if found == nil {
		t.Fatalf("expected contact with conversationID %s in contacts list, got %+v", sessionUUID, contactsResp.Contacts)
	}
	if found.Email != "siti.rahma@example.com" {
		t.Fatalf("expected Email 'siti.rahma@example.com', got '%s'", found.Email)
	}
	if found.Name != "Siti Rahma" {
		t.Fatalf("expected Name 'Siti Rahma', got '%s'", found.Name)
	}

	// 3. Search contacts by email query
	reqSearch := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?search=siti.rahma@example.com", nil)
	reqSearch.Header.Set("Authorization", "Bearer "+token)
	recSearch := httptest.NewRecorder()
	srv.Router().ServeHTTP(recSearch, reqSearch)
	if recSearch.Code != http.StatusOK {
		t.Fatalf("expected 200 for search contacts, got %d", recSearch.Code)
	}
	var searchResp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(recSearch.Body.Bytes(), &searchResp); err != nil || len(searchResp.Contacts) == 0 {
		t.Fatalf("expected at least 1 contact matching email search, got %d", len(searchResp.Contacts))
	}
	if searchResp.Contacts[0].Email != "siti.rahma@example.com" {
		t.Fatalf("expected searched contact email 'siti.rahma@example.com', got '%s'", searchResp.Contacts[0].Email)
	}

	// 4. Fetch conversation messages -> verify contact.email is returned
	reqConv := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+sessionUUID+"/messages", nil)
	reqConv.Header.Set("Authorization", "Bearer "+token)
	recConv := httptest.NewRecorder()
	srv.Router().ServeHTTP(recConv, reqConv)
	if recConv.Code != http.StatusOK {
		t.Fatalf("expected 200 for conversation messages, got %d: %s", recConv.Code, recConv.Body.String())
	}
	var convResp struct {
		Contact  ChatContact      `json:"contact"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recConv.Body.Bytes(), &convResp); err != nil {
		t.Fatalf("failed to decode conversation messages: %v", err)
	}
	if convResp.Contact.Email != "siti.rahma@example.com" {
		t.Fatalf("expected conv contact Email 'siti.rahma@example.com', got '%s'", convResp.Contact.Email)
	}
	if convResp.Contact.Name != "Siti Rahma" {
		t.Fatalf("expected conv contact Name 'Siti Rahma', got '%s'", convResp.Contact.Name)
	}
}

func TestChat_SingleAgentCuanMentionAndContacts(t *testing.T) {
	srv, token := setupTestChatServer(t)
	repo := cuanDB.NewMemoryAgentRepository()
	srv.SetAgentRepository(repo)

	// Ensure built-in agents (@cuan, @sm, @dev, @support)
	if err := cuanDB.EnsureBuiltinAgents(context.Background(), repo); err != nil {
		t.Fatal(err)
	}

	// Explicitly start a conversation; listing must not create one.
	srv.resolveConversationContext(context.Background(), cuanDB.AgentIDCuan, "")

	// 1. GET /api/v1/chat/contacts?type=agents -> verify ONLY @cuan is returned (internal legacy bots are hidden)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/contacts?type=agents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Contacts) != 1 {
		t.Fatalf("expected exactly 1 contact (@cuan), got %d: %+v", len(resp.Contacts), resp.Contacts)
	}
	if resp.Contacts[0].Name != "@cuan" {
		t.Fatalf("expected contact name '@cuan', got '%s'", resp.Contacts[0].Name)
	}

	// 2. Test mention resolution for @cuan
	ctx := context.Background()
	resCuan := srv.resolveConversationContext(ctx, "", "@cuan tolong cek laporan ini")
	if !resCuan.IsAgent || resCuan.TargetAgentID != cuanDB.AgentIDCuan {
		t.Fatalf("expected mention @cuan to resolve to agent_cuan, got %+v", resCuan)
	}
	if resCuan.TargetHarness != "agy" {
		t.Fatalf("expected TargetHarness 'agy', got %q", resCuan.TargetHarness)
	}

	// 3. Test legacy alias mentions (@team, @sm, @dev, @support) resolve to agent_cuan
	for _, mention := range []string{"@team halo", "@sm sprint", "@dev build", "@support help"} {
		resLegacy := srv.resolveConversationContext(ctx, "", mention)
		if !resLegacy.IsAgent || resLegacy.TargetAgentID != cuanDB.AgentIDCuan {
			t.Fatalf("expected legacy mention %q to resolve to agent_cuan, got %+v", mention, resLegacy)
		}
		if resLegacy.TargetHarness != "agy" {
			t.Fatalf("expected legacy mention %q TargetHarness 'agy', got %q", mention, resLegacy.TargetHarness)
		}
	}

	// 4. Test mention in existing room with empty topic (customer/visitor room)
	custRoomID := cuanDB.GenerateUUID()
	_ = srv.projectStudioRepo.CreateRoom(ctx, &cuanDB.ProjectRoom{
		ID:        custRoomID,
		ProjectID: "default",
		Title:     "Customer Visitor #123",
		Topic:     "",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	resInRoom := srv.resolveConversationContext(ctx, custRoomID, "@cuan aktifkan akses user ini dan bikinkan respon balasan")
	if !resInRoom.IsAgent || resInRoom.TargetAgentID != cuanDB.AgentIDCuan {
		t.Fatalf("expected mention @cuan in customer room to resolve to agent_cuan, got %+v", resInRoom)
	}
	if resInRoom.RoomID != custRoomID {
		t.Fatalf("expected RoomID to be preserved as %s, got %s", custRoomID, resInRoom.RoomID)
	}
}

func TestCuanAgent_LoadBalancerRouting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)
	repo := cuanDB.NewMemoryAgentRepository()
	srv.SetAgentRepository(repo)

	if err := cuanDB.EnsureBuiltinAgents(context.Background(), repo); err != nil {
		t.Fatal(err)
	}

	// Verify @cuan has normalized harness "agy" even if provider is "google"
	cuanAgent, err := repo.GetAgentByID(context.Background(), cuanDB.AgentIDCuan)
	if err != nil {
		t.Fatal(err)
	}
	if cuanAgent.Harness != "agy" {
		t.Fatalf("expected @cuan Harness to be 'agy', got %q", cuanAgent.Harness)
	}

	// Create an active provider account owned by the user
	ctx := context.Background()
	owner, err := srv.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}

	acct, err := agent.CreateProviderAccount(owner.ID, "Google Workspace (@sm)")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := agent.ProviderAccountHome(owner.ID, acct.ID)
	_ = os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
	_ = os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-credential"), 0600)

	body := `{"name":"@primary","handle":"@primary","provider":"agy","account_id":"` + acct.ID + `"}`
	r := httptest.NewRequest("POST", "/api/v1/agents/connect-cli", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, r)
	if w.Code != 201 && w.Code != 200 {
		t.Fatalf("connect-cli failed: %d %s", w.Code, w.Body.String())
	}

	metaPath := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", acct.ID, "account.json")
	metaData, _ := os.ReadFile(metaPath)
	var meta map[string]any
	_ = json.Unmarshal(metaData, &meta)
	meta["state"] = "verified"
	metaData, _ = json.Marshal(meta)
	_ = os.WriteFile(metaPath, metaData, 0600)

	// Test providerRoutes for @cuan facade: it MUST discover and route to the active connected account
	routes := srv.providerRoutes(ctx, owner.ID, cuanAgent, "", false)
	if len(routes) == 0 {
		t.Fatalf("expected @cuan to route through active provider account in load balancer pool, got 0 routes")
	}

	// Verify route properties
	foundSMRoute := false
	for _, r := range routes {
		if r.accountID == acct.ID {
			foundSMRoute = true
			break
		}
	}
	if !foundSMRoute {
		t.Fatalf("expected routes to contain account %s, got: %+v", acct.ID, routes)
	}

	// Test leaseProviderRoute: @cuan must be allowed to lease this route
	release, err := srv.leaseProviderRoute(ctx, owner.ID, cuanAgent, routes[0])
	if err != nil {
		t.Fatalf("expected leaseProviderRoute to succeed for @cuan facade, got error: %v", err)
	}
	if release != nil {
		release()
	}
}

func TestHandleUpdateAndDeleteInboxMessage(t *testing.T) {
	srv, token := setupTestChatServer(t)
	ctx := context.Background()

	roomID := cuanDB.GenerateUUID()
	if err := srv.projectStudioRepo.CreateRoom(ctx, &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Test Edit Delete Room",
		Topic:     "agent:agent_cuan",
	}); err != nil {
		t.Fatalf("create room: %v", err)
	}

	userMsgID := cuanDB.GenerateUUID()
	agentMsgID := cuanDB.GenerateUUID()
	sysMsgID := cuanDB.GenerateUUID()
	oldTime := time.Now().UTC().Add(-2 * time.Minute)

	for _, m := range []*cuanDB.ProjectMessage{
		{
			ID:         userMsgID,
			RoomID:     roomID,
			AuthorType: "user",
			AuthorID:   "operator_user",
			AuthorName: "Owner",
			Content:    "Original user message",
			CreatedAt:  oldTime,
			UpdatedAt:  oldTime,
		},
		{
			ID:         agentMsgID,
			RoomID:     roomID,
			AuthorType: "agent",
			AuthorID:   "agent_cuan",
			AuthorName: "Cuan",
			Content:    "Agent reply",
			CreatedAt:  oldTime,
			UpdatedAt:  oldTime,
		},
		{
			ID:         sysMsgID,
			RoomID:     roomID,
			AuthorType: "system",
			AuthorID:   "cuan_system_lifecycle",
			AuthorName: "System",
			Content:    "System notice",
			CreatedAt:  oldTime,
			UpdatedAt:  oldTime,
		},
	} {
		if err := srv.projectStudioRepo.CreateMessage(ctx, m); err != nil {
			t.Fatalf("create message %s: %v", m.ID, err)
		}
	}
	// Reset user message CreatedAt back to 2 minutes ago so UpdatedAt - CreatedAt > 1s after PATCH
	if fetched, err := srv.projectStudioRepo.GetMessageByID(ctx, userMsgID); err == nil {
		if memRepo, ok := srv.projectStudioRepo.(*cuanDB.MemoryProjectStudioRepository); ok {
			_ = memRepo
			fetched.CreatedAt = oldTime
		}
	}

	// 1. Forbidden on agent and system messages
	for _, forbiddenID := range []string{agentMsgID, sysMsgID} {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/chat/conversations/"+roomID+"/messages/"+forbiddenID, strings.NewReader(`{"content":"hijack"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for PATCH on %s, got %d: %s", forbiddenID, rec.Code, rec.Body.String())
		}

		reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/conversations/"+roomID+"/messages/"+forbiddenID, nil)
		reqDel.Header.Set("Authorization", "Bearer "+token)
		recDel := httptest.NewRecorder()
		srv.Router().ServeHTTP(recDel, reqDel)
		if recDel.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for DELETE on %s, got %d: %s", forbiddenID, recDel.Code, recDel.Body.String())
		}
	}

	// 2. Empty content rejected on PATCH
	reqEmpty := httptest.NewRequest(http.MethodPatch, "/api/v1/chat/conversations/"+roomID+"/messages/"+userMsgID, strings.NewReader(`{"content":"   "}`))
	reqEmpty.Header.Set("Authorization", "Bearer "+token)
	recEmpty := httptest.NewRecorder()
	srv.Router().ServeHTTP(recEmpty, reqEmpty)
	if recEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty PATCH, got %d", recEmpty.Code)
	}

	// 3. Valid PATCH on user message updates content and sets edited=true (even when edited <= 1s after creation)
	immediateMsgID := cuanDB.GenerateUUID()
	if err := srv.projectStudioRepo.CreateMessage(ctx, &cuanDB.ProjectMessage{
		ID:         immediateMsgID,
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		AuthorName: "Owner",
		Content:    "Immediate message",
	}); err != nil {
		t.Fatalf("create immediate message: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	reqImmediatePatch := httptest.NewRequest(http.MethodPatch, "/api/v1/chat/conversations/"+roomID+"/messages/"+immediateMsgID, strings.NewReader(`{"content":"Immediate edited"}`))
	reqImmediatePatch.Header.Set("Authorization", "Bearer "+token)
	recImmediatePatch := httptest.NewRecorder()
	srv.Router().ServeHTTP(recImmediatePatch, reqImmediatePatch)
	if recImmediatePatch.Code != http.StatusOK {
		t.Fatalf("expected 200 for immediate PATCH, got %d: %s", recImmediatePatch.Code, recImmediatePatch.Body.String())
	}
	var immediateDTO ChatMessageDTO
	if err := json.Unmarshal(recImmediatePatch.Body.Bytes(), &immediateDTO); err != nil {
		t.Fatalf("unmarshal immediate PATCH response: %v", err)
	}
	if !immediateDTO.Edited {
		t.Fatalf("expected Edited=true for immediate edit within <=1s, got false")
	}

	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/chat/conversations/"+roomID+"/messages/"+userMsgID, strings.NewReader(`{"content":"Updated user message"}`))
	reqPatch.Header.Set("Authorization", "Bearer "+token)
	recPatch := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPatch, reqPatch)
	if recPatch.Code != http.StatusOK {
		t.Fatalf("expected 200 for PATCH, got %d: %s", recPatch.Code, recPatch.Body.String())
	}
	var updatedDTO ChatMessageDTO
	if err := json.Unmarshal(recPatch.Body.Bytes(), &updatedDTO); err != nil {
		t.Fatalf("unmarshal PATCH response: %v", err)
	}
	if updatedDTO.Content != "Updated user message" {
		t.Fatalf("expected updated content, got %q", updatedDTO.Content)
	}

	// Verify persisted in GET messages
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+roomID+"/messages", nil)
	reqGet.Header.Set("Authorization", "Bearer "+token)
	recGet := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK || !strings.Contains(recGet.Body.String(), "Updated user message") {
		t.Fatalf("expected updated message in GET response, got %d: %s", recGet.Code, recGet.Body.String())
	}

	// 4. Valid DELETE on user message removes it permanently
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/conversations/"+roomID+"/messages/"+userMsgID, nil)
	reqDel.Header.Set("Authorization", "Bearer "+token)
	recDel := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 for DELETE, got %d: %s", recDel.Code, recDel.Body.String())
	}

	reqGetAfter := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+roomID+"/messages", nil)
	reqGetAfter.Header.Set("Authorization", "Bearer "+token)
	recGetAfter := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGetAfter, reqGetAfter)
	if strings.Contains(recGetAfter.Body.String(), userMsgID) || strings.Contains(recGetAfter.Body.String(), "Updated user message") {
		t.Fatalf("expected deleted message to be absent after DELETE, got: %s", recGetAfter.Body.String())
	}
}

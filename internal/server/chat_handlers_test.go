package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	if len(contactsResp.Contacts) != 0 {
		t.Fatalf("expected no fabricated agent contacts: %+v", contactsResp.Contacts)
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

	// 5. Operator Replies from Direct Messenger Hub (POST /api/v1/chat/conversations/{id}/messages)
	operatorReplyPayload := map[string]interface{}{
		"message":     "Halo Mas Budi! Betul, lisensi Shuriken Lifetime mencakup pemakaian multi-domain.",
		"author_type": "agent",
		"author_name": "Dojo Support",
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
		if m.AuthorType == "agent" && strings.Contains(m.Message, "Draf Balasan") {
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

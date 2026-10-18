package server

import (
	"bytes"
	"context"
	"encoding/json"
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

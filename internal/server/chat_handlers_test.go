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

	if len(contactsResp.Contacts) < 2 {
		t.Fatalf("expected at least 2 default agent contacts, got %d", len(contactsResp.Contacts))
	}
	if contactsResp.Contacts[0].Name != "Scrum Master" {
		t.Fatalf("expected first contact to be Scrum Master, got %s", contactsResp.Contacts[0].Name)
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

	// 7. Test Scrum Master Direct Chat with Automated Triage & 1-Click Action Card
	smMsgPayload := map[string]interface{}{
		"message": "Tolong perbaiki responsive modal pembayaran di mobile",
	}
	smMsgBody, _ := json.Marshal(smMsgPayload)
	reqSM := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+defaultScrumMasterRoomID+"/messages", bytes.NewReader(smMsgBody))
	reqSM.Header.Set("Content-Type", "application/json")
	reqSM.Header.Set("Authorization", "Bearer "+token)
	recSM := httptest.NewRecorder()
	srv.Router().ServeHTTP(recSM, reqSM)
	if recSM.Code != http.StatusOK {
		t.Fatalf("expected 200 for SM message, got %d", recSM.Code)
	}
	var smResp struct {
		Message    ChatMessageDTO  `json:"message"`
		AgentReply *ChatMessageDTO `json:"agent_reply"`
	}
	_ = json.Unmarshal(recSM.Body.Bytes(), &smResp)
	if smResp.AgentReply == nil {
		t.Fatalf("expected automated agent reply from Scrum Master")
	}
	if smResp.AgentReply.ActionCard == nil || len(smResp.AgentReply.ActionCard.DoDChecklist) == 0 {
		t.Fatalf("expected ActionCard with DoD checklist from Scrum Master")
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

func TestChatHandlers_ScrumMasterConversationalGreetings(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// Test Cases for Conversational Greetings / Questions (Should have NO ActionCard)
	casualInputs := []string{
		"Apa kabar",
		"Halo",
		"Pagi",
		"Bagaimana sprint?",
		"Siapa kamu?",
	}

	for _, input := range casualInputs {
		payload := map[string]interface{}{"message": input}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+defaultScrumMasterRoomID+"/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for casual message '%s', got %d", input, rec.Code)
		}

		var resp struct {
			Message    ChatMessageDTO  `json:"message"`
			AgentReply *ChatMessageDTO `json:"agent_reply"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response for '%s': %v", input, err)
		}

		if resp.AgentReply == nil {
			t.Fatalf("expected agent reply for '%s'", input)
		}

		// Verify NO ActionCard for casual conversation
		if resp.AgentReply.ActionCard != nil {
			t.Fatalf("expected NO ActionCard for casual greeting '%s', but got: %+v", input, resp.AgentReply.ActionCard)
		}

		// Verify it does NOT contain rigid problem statement template
		if strings.Contains(resp.AgentReply.Message, "Problem Statement: "+input) || strings.Contains(resp.AgentReply.Message, "Mulai Kerjakan") {
			t.Fatalf("expected natural response without rigid template for '%s', got: %s", input, resp.AgentReply.Message)
		}
	}

	// Test Case for Technical Task Instruction (MUST have ActionCard & DoD)
	taskMsg := "Tolong buatkan fitur export CSV transaksi"
	taskPayload := map[string]interface{}{"message": taskMsg}
	taskBody, _ := json.Marshal(taskPayload)
	reqTask := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+defaultScrumMasterRoomID+"/messages", bytes.NewReader(taskBody))
	reqTask.Header.Set("Content-Type", "application/json")
	reqTask.Header.Set("Authorization", "Bearer "+token)
	recTask := httptest.NewRecorder()
	srv.Router().ServeHTTP(recTask, reqTask)

	if recTask.Code != http.StatusOK {
		t.Fatalf("expected 200 for task message, got %d", recTask.Code)
	}

	var taskResp struct {
		Message    ChatMessageDTO  `json:"message"`
		AgentReply *ChatMessageDTO `json:"agent_reply"`
	}
	if err := json.Unmarshal(recTask.Body.Bytes(), &taskResp); err != nil {
		t.Fatalf("failed to decode task response: %v", err)
	}

	if taskResp.AgentReply == nil || taskResp.AgentReply.ActionCard == nil {
		t.Fatalf("expected ActionCard for technical task message, got nil")
	}
	if !strings.Contains(taskResp.AgentReply.ActionCard.TaskTitle, "export CSV") {
		t.Fatalf("expected TaskTitle to reflect task, got: %s", taskResp.AgentReply.ActionCard.TaskTitle)
	}
	if len(taskResp.AgentReply.ActionCard.DoDChecklist) == 0 {
		t.Fatalf("expected DoDChecklist for technical task")
	}
}

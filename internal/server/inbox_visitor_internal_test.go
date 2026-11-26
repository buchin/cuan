package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
)

func TestVisitorMessagesNeverInternal(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Initialize visitor chat session
	guestUUID := "guest-visitor-github-user-1"
	visitorInitPayload := map[string]interface{}{
		"guest_uuid": guestUUID,
		"product":    "shuriken",
		"name":       "Test Visitor",
		"email":      "visitor@example.com",
		"message":    "Halo, mau tanya integrasi GitHub",
	}
	initBody, _ := json.Marshal(visitorInitPayload)
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(initBody))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)

	if recInit.Code != http.StatusOK {
		t.Fatalf("expected 200 on session init, got %d: %s", recInit.Code, recInit.Body.String())
	}

	var initResp struct {
		OK      bool `json:"ok"`
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recInit.Body.Bytes(), &initResp); err != nil || !initResp.OK {
		t.Fatalf("failed to decode session init response: %v", err)
	}
	sessionUUID := initResp.Session.UUID

	// 2. Visitor sends a reply specifying their GitHub username (like in customer issue: "username github : gvinum")
	// and another reply with an @mention syntax: "@gvinum" or "@cuan"
	visitorReplies := []string{
		"username github : gvinum",
		"@gvinum mohon di cek",
		"@cuan tolong konfirmasi ya",
	}

	for _, replyText := range visitorReplies {
		sendPayload := map[string]interface{}{
			"session_uuid": sessionUUID,
			"guest_uuid":   guestUUID,
			"message":      replyText,
		}
		sendBody, _ := json.Marshal(sendPayload)
		reqSend := httptest.NewRequest(http.MethodPost, "/api/chat/messages", bytes.NewReader(sendBody))
		reqSend.Header.Set("Content-Type", "application/json")
		recSend := httptest.NewRecorder()
		srv.Router().ServeHTTP(recSend, reqSend)

		if recSend.Code != http.StatusOK {
			t.Fatalf("expected 200 on visitor send, got %d: %s", recSend.Code, recSend.Body.String())
		}

		var sendResp struct {
			OK      bool           `json:"ok"`
			Message ChatMessageDTO `json:"message"`
		}
		if err := json.Unmarshal(recSend.Body.Bytes(), &sendResp); err != nil || !sendResp.OK {
			t.Fatalf("failed to decode visitor send response: %v", err)
		}

		if sendResp.Message.IsInternal {
			t.Fatalf("REGRESSION: Visitor message %q was wrongly flagged as IsInternal=true in send response", replyText)
		}
		if sendResp.Message.SenderType != "visitor" {
			t.Fatalf("expected SenderType='visitor', got %q for message %q", sendResp.Message.SenderType, replyText)
		}
	}

	// 3. Operator creates an actual internal whisper note to test separation
	ctx := context.Background()
	now := time.Now().UTC()
	if err := srv.projectStudioRepo.CreateMessage(ctx, &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     sessionUUID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		AuthorName: "Owner",
		Content:    "@cuan Note internal: visitor ini akun VIP, berikan diskon",
		CreatedAt:  now.Add(10 * time.Second),
		UpdatedAt:  now.Add(10 * time.Second),
	}); err != nil {
		t.Fatalf("failed to create operator whisper: %v", err)
	}

	// 4. Operator fetches room messages via internal conversation endpoint
	reqInternal := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+sessionUUID+"/messages", nil)
	reqInternal.Header.Set("Authorization", "Bearer "+token)
	recInternal := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInternal, reqInternal)

	if recInternal.Code != http.StatusOK {
		t.Fatalf("failed to fetch internal messages: %d", recInternal.Code)
	}

	var internalResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recInternal.Body.Bytes(), &internalResp); err != nil {
		t.Fatalf("failed to parse internal messages: %v", err)
	}

	var foundGithubVisitorMsg, foundOperatorWhisper bool
	for _, m := range internalResp.Messages {
		if strings.Contains(m.Content, "gvinum") {
			foundGithubVisitorMsg = true
			if m.IsInternal {
				t.Fatalf("REGRESSION: Visitor message containing github username %q has IsInternal=true in conversation API", m.Content)
			}
			if m.SenderType != "visitor" {
				t.Fatalf("expected SenderType='visitor', got %q for message %q", m.SenderType, m.Content)
			}
		}
		if strings.Contains(m.Content, "Note internal: visitor ini akun VIP") {
			foundOperatorWhisper = true
			if !m.IsInternal {
				t.Fatalf("expected operator whisper to have IsInternal=true")
			}
		}
	}

	if !foundGithubVisitorMsg {
		t.Fatalf("expected visitor github message to be present in conversation messages")
	}
	if !foundOperatorWhisper {
		t.Fatalf("expected operator whisper to be present in conversation messages")
	}

	// 5. Visitor polls messages via public visitor endpoint: GET /api/chat/messages?session_uuid=...
	reqPoll := httptest.NewRequest(http.MethodGet, "/api/chat/messages?session_uuid="+sessionUUID, nil)
	recPoll := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPoll, reqPoll)

	if recPoll.Code != http.StatusOK {
		t.Fatalf("failed to poll visitor messages: %d", recPoll.Code)
	}

	var pollResp struct {
		OK       bool             `json:"ok"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recPoll.Body.Bytes(), &pollResp); err != nil || !pollResp.OK {
		t.Fatalf("failed to parse visitor poll response: %v", err)
	}

	var visitorPollFoundGithub bool
	for _, m := range pollResp.Messages {
		if strings.Contains(m.Content, "gvinum") {
			visitorPollFoundGithub = true
			if m.IsInternal {
				t.Fatalf("LEAK: Visitor message marked IsInternal=true in visitor poll response: %q", m.Content)
			}
		}
		if strings.Contains(m.Content, "Note internal: visitor ini akun VIP") {
			t.Fatalf("LEAK DETECTED: Operator internal whisper leaked to visitor poll endpoint: %q", m.Content)
		}
	}
	if !visitorPollFoundGithub {
		t.Fatalf("expected visitor to receive their github reply in visitor poll endpoint")
	}
}

func TestFormatConversationHistoryContext_VisitorRole(t *testing.T) {
	now := time.Now()
	messages := []cuanDB.ProjectMessage{
		{
			ID:         "msg-1",
			AuthorType: "user",
			AuthorID:   "guest-12345",
			AuthorName: "Visitor #12345",
			Content:    "username github : gvinum",
			CreatedAt:  now.Add(-2 * time.Minute),
		},
		{
			ID:         "msg-2",
			AuthorType: "user",
			AuthorID:   "operator_user",
			AuthorName: "Owner",
			Content:    "Halo gvinum, akses sudah diberikan",
			CreatedAt:  now.Add(-1 * time.Minute),
		},
		{
			ID:         "msg-3",
			AuthorType: "user",
			AuthorID:   "guest-12345",
			AuthorName: "Visitor #12345",
			Content:    "Terima kasih!",
			CreatedAt:  now,
		},
	}

	result := formatConversationHistoryContext(messages, "msg-3", 10)

	if !strings.Contains(result, "**Visitor**: username github : gvinum") {
		t.Fatalf("expected visitor turn to be labeled '**Visitor**:', got:\n%s", result)
	}
	if !strings.Contains(result, "**User**: Halo gvinum, akses sudah diberikan") {
		t.Fatalf("expected operator turn to be labeled '**User**:', got:\n%s", result)
	}
	if !strings.Contains(result, "**Visitor**: Terima kasih!") {
		t.Fatalf("expected visitor turn to be labeled '**Visitor**:', got:\n%s", result)
	}
}

func TestCustomerMessageVisibility_WhisperWithAttachmentAndDelete(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// 1. Initialize visitor chat session
	initBody, _ := json.Marshal(map[string]any{
		"guest_uuid": "guest-whisper-attachment-1",
		"product":    "cuan",
		"name":       "Test Visitor",
		"email":      "visitor@example.com",
		"message":    "Halo min",
	})
	reqInit := httptest.NewRequest(http.MethodPost, "/api/chat/session", bytes.NewReader(initBody))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInit, reqInit)
	if recInit.Code != http.StatusOK {
		t.Fatalf("expected 200 on session init, got %d: %s", recInit.Code, recInit.Body.String())
	}
	var initResp struct {
		OK      bool `json:"ok"`
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
	}
	if err := json.Unmarshal(recInit.Body.Bytes(), &initResp); err != nil || !initResp.OK {
		t.Fatalf("failed to decode session init response: %v", err)
	}
	sessionUUID := initResp.Session.UUID

	// 2. Operator creates an internal whisper message with an attachment (CUAN_MESSAGE_V1 envelope)
	ctx := context.Background()
	now := time.Now().UTC()
	whisperID := cuanDB.GenerateUUID()
	attID := cuanDB.GenerateUUID()
	whisperContent := encodeAttachmentMessage("@cuan ini creds prod rahasia", []string{attID})
	if err := srv.projectStudioRepo.CreateMessage(ctx, &cuanDB.ProjectMessage{
		ID:         whisperID,
		RoomID:     sessionUUID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		AuthorName: "You",
		Content:    whisperContent,
		CreatedAt:  now.Add(5 * time.Second),
		UpdatedAt:  now.Add(5 * time.Second),
	}); err != nil {
		t.Fatalf("failed to create whisper with attachment: %v", err)
	}

	// 3. Internal conversation endpoint must mark it IsInternal=true
	reqInternal := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+sessionUUID+"/messages", nil)
	reqInternal.Header.Set("Authorization", "Bearer "+token)
	recInternal := httptest.NewRecorder()
	srv.Router().ServeHTTP(recInternal, reqInternal)
	if recInternal.Code != http.StatusOK {
		t.Fatalf("failed to fetch internal messages: %d", recInternal.Code)
	}
	var internalResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recInternal.Body.Bytes(), &internalResp); err != nil {
		t.Fatalf("failed to parse internal messages: %v", err)
	}
	foundInternal := false
	for _, m := range internalResp.Messages {
		if m.UUID == whisperID {
			foundInternal = true
			if !m.IsInternal {
				t.Fatalf("LEAK: whisper with attachment was not marked IsInternal=true")
			}
		}
	}
	if !foundInternal {
		t.Fatalf("expected whisper with attachment to appear in internal conversation endpoint")
	}

	// 4. Visitor poll endpoint MUST NOT return the whisper with attachment
	reqPoll := httptest.NewRequest(http.MethodGet, "/api/chat/messages?session_uuid="+sessionUUID, nil)
	recPoll := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPoll, reqPoll)
	if recPoll.Code != http.StatusOK {
		t.Fatalf("failed to poll visitor messages: %d", recPoll.Code)
	}
	var pollResp struct {
		OK       bool             `json:"ok"`
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recPoll.Body.Bytes(), &pollResp); err != nil || !pollResp.OK {
		t.Fatalf("failed to parse visitor poll response: %v", err)
	}
	for _, m := range pollResp.Messages {
		if m.UUID == whisperID || strings.Contains(m.Content, "creds prod rahasia") {
			t.Fatalf("LEAK DETECTED: whisper with attachment leaked to visitor poll endpoint: %+v", m)
		}
	}

	// 5. Delete message via DELETE /api/messages/{id}
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/messages/"+whisperID, nil)
	reqDel.Header.Set("Authorization", "Bearer "+token)
	recDel := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting message, got %d: %s", recDel.Code, recDel.Body.String())
	}

	// 6. Deleted message must no longer appear in internal conversation endpoint
	reqAfter := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/"+sessionUUID+"/messages", nil)
	reqAfter.Header.Set("Authorization", "Bearer "+token)
	recAfter := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf("failed to fetch internal messages after delete: %d", recAfter.Code)
	}
	var afterResp struct {
		Messages []ChatMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(recAfter.Body.Bytes(), &afterResp); err != nil {
		t.Fatalf("failed to parse internal messages after delete: %v", err)
	}
	for _, m := range afterResp.Messages {
		if m.UUID == whisperID {
			t.Fatalf("expected deleted message %s to be removed from conversation", whisperID)
		}
	}
}

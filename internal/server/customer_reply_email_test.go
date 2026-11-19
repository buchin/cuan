package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/resend"
)

type mockCustomerEmailResendClient struct {
	mu           sync.Mutex
	validateErr  error
	sendErr      error
	sentEmails   []resend.EmailRequest
	validateRuns int
}

func (m *mockCustomerEmailResendClient) ValidateAPIKey(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validateRuns++
	return m.validateErr
}

func (m *mockCustomerEmailResendClient) SendEmail(ctx context.Context, req resend.EmailRequest) (*resend.EmailResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	m.sentEmails = append(m.sentEmails, req)
	return &resend.EmailResponse{ID: "re_mock_test_123"}, nil
}

func (m *mockCustomerEmailResendClient) getSentEmails() []resend.EmailRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]resend.EmailRequest, len(m.sentEmails))
	copy(copied, m.sentEmails)
	return copied
}

func TestSendCustomerReplyEmail_Unauthorized(t *testing.T) {
	srv, _ := setupTestChatServer(t)
	roomID := db.GenerateUUID()
	msgID := db.GenerateUUID()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+msgID+"/email", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without auth, got %d", rec.Code)
	}
}

func TestSendCustomerReplyEmail_InvalidUUID(t *testing.T) {
	srv, token := setupTestChatServer(t)
	roomID := db.GenerateUUID()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/invalid-uuid/email", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid UUID, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendCustomerReplyEmail_ValidationAndSafetyGuards(t *testing.T) {
	srv, token := setupTestChatServer(t)
	roomID := db.GenerateUUID()
	msgID := db.GenerateUUID()

	// Seed message in studio repo
	ctx := context.Background()
	_ = srv.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "default", Title: "Customer Room"})
	_ = srv.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         msgID,
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   db.AgentIDCuan,
		Content:    "```cuan-customer-reply\n{\"draft\":\"Halo ini draft balasan.\"}\n```",
	})

	call := func(payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+msgID+"/email", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		return rec
	}

	// 1. Missing recipient
	rec := call(`{"draft":"Halo"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "penerima") {
		t.Fatalf("expected 400 for missing recipient, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. CRLF injection in recipient
	rec = call(`{"recipient":"test@example.com\r\nBcc: evil@domain.com","draft":"Halo"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for CRLF in recipient, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Invalid email format
	rec = call(`{"recipient":"not-an-email","draft":"Halo"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid email, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. CRLF in subject
	rec = call(`{"recipient":"customer@example.com","subject":"Subjek\r\nInjeksi","draft":"Halo"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for CRLF in subject, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendCustomerReplyEmail_SuccessAndIdempotency(t *testing.T) {
	srv, token := setupTestChatServer(t)
	roomID := db.GenerateUUID()
	msgID := db.GenerateUUID()

	mock := &mockCustomerEmailResendClient{}
	srv.SetResendClient(mock)

	ctx := context.Background()
	_ = srv.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "default", Title: "Customer Room"})
	_ = srv.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         msgID,
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   db.AgentIDCuan,
		Content:    "Halo, kendala sudah kami cek.\n```cuan-customer-reply\n{\"draft\":\"Halo sob, kendala login sudah diperbaiki.\"}\n```",
	})

	payload := `{"recipient":"afukwong85@gmail.com","subject":"Update Akun Nerd"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+msgID+"/email", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on successful send, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp sendCustomerReplyEmailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Status != "success" || resp.DeliveryStatus != "sent" || resp.EmailID != "re_mock_test_123" {
		t.Fatalf("unexpected response payload: %+v", resp)
	}

	sent := mock.getSentEmails()
	if len(sent) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(sent))
	}
	if sent[0].To[0] != "afukwong85@gmail.com" {
		t.Fatalf("expected recipient afukwong85@gmail.com, got %s", sent[0].To[0])
	}
	if sent[0].Subject != "Update Akun Nerd" {
		t.Fatalf("expected subject 'Update Akun Nerd', got %s", sent[0].Subject)
	}
	if !strings.Contains(sent[0].Text, "kendala login sudah diperbaiki") {
		t.Fatalf("expected draft content extracted from cuan-customer-reply marker, got: %s", sent[0].Text)
	}

	// 2. Test Idempotency: Send the same message ID again
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+msgID+"/email", strings.NewReader(payload))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on idempotent retry, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var resp2 sendCustomerReplyEmailResponse
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if !resp2.AlreadySent {
		t.Fatalf("expected already_sent=true on idempotent retry, got %+v", resp2)
	}

	// Provider should NOT be called again
	if len(mock.getSentEmails()) != 1 {
		t.Fatalf("expected provider not called again on idempotent send, got %d sends", len(mock.getSentEmails()))
	}
}

func TestSendCustomerReplyEmail_ProviderFailure(t *testing.T) {
	srv, token := setupTestChatServer(t)
	roomID := db.GenerateUUID()
	msgID := db.GenerateUUID()

	mock := &mockCustomerEmailResendClient{
		sendErr: errors.New("Resend API rate limit exceeded"),
	}
	srv.SetResendClient(mock)

	ctx := context.Background()
	_ = srv.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "default", Title: "Customer Room"})
	_ = srv.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         msgID,
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   db.AgentIDCuan,
		Content:    "```cuan-customer-reply\n{\"draft\":\"Pesan draf.\"}\n```",
	})

	payload := `{"recipient":"afukwong86@gmail.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/messages/"+msgID+"/email", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway on provider failure, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "rate limit exceeded") {
		t.Fatalf("expected error message to contain provider failure details, got: %s", rec.Body.String())
	}
}

func TestSendConversationEmail_DirectRoute(t *testing.T) {
	srv, token := setupTestChatServer(t)
	roomID := db.GenerateUUID()

	mock := &mockCustomerEmailResendClient{}
	srv.SetResendClient(mock)

	ctx := context.Background()
	_ = srv.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "default", Title: "Customer Room"})

	payload := `{"recipient":"afukwong85@gmail.com","content":"Halo ini pengiriman langsung","subject":"Info Penting"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/conversations/"+roomID+"/send-email", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on send-email route, got %d: %s", rec.Code, rec.Body.String())
	}

	sent := mock.getSentEmails()
	if len(sent) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(sent))
	}
	if sent[0].To[0] != "afukwong85@gmail.com" || sent[0].Text != "Halo ini pengiriman langsung" {
		t.Fatalf("unexpected sent email: %+v", sent[0])
	}
}

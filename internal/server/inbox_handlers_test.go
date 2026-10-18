package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"context"

	"github.com/buchin/cuan/internal/db"
)

func setupTestInboxServer(t *testing.T) (*Server, db.ProjectStudioRepository) {
	s := newTestServerWithMemoryDB(t)
	if s.AuthService() != nil {
		_ = s.AuthService().AutoSeedDefaultAdmin(context.Background())
	}
	return s, s.ProjectStudioRepository()
}

func TestInboxHandlers_CompleteFlow(t *testing.T) {
	s, _ := setupTestInboxServer(t)

	// Admin Login to get JWT auth token
	loginBody := []byte(`{"username":"admin","password":"whosyourdaddy"}`)
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	s.Router().ServeHTTP(recLogin, reqLogin)
	if recLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for admin login, got %d: %s", recLogin.Code, recLogin.Body.String())
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(recLogin.Body.Bytes(), &loginResp)
	authToken := loginResp.Token

	// 1. Initial GET /api/v1/inbox (Empty)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/inbox", nil)
	req.Header.Set("Authorization", "Bearer "+authToken)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var listResp struct {
		Threads []InboxThreadItem `json:"threads"`
		Total   int               `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Total != 0 {
		t.Fatalf("expected 0 threads initially, got %d", listResp.Total)
	}

	// 2. Create new thread via POST /api/v1/inbox
	createPayload := CreateInboxThreadRequest{
		Title:   "Fix Safari Mobile login padding",
		Message: "Hi SM, login button on iOS Safari has inconsistent padding.",
	}
	body, _ := json.Marshal(createPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/inbox", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Thread   db.ProjectRoomDTO     `json:"thread"`
		Messages []db.ProjectMessageDTO `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}

	threadID := createResp.Thread.ID
	if threadID == "" {
		t.Fatalf("expected non-empty thread id")
	}
	if len(createResp.Messages) != 2 {
		t.Fatalf("expected 2 messages (user + SM triage), got %d", len(createResp.Messages))
	}
	if createResp.Messages[0].AuthorType != "user" {
		t.Errorf("expected first message to be user, got %s", createResp.Messages[0].AuthorType)
	}
	if createResp.Messages[1].AuthorType != "agent" || createResp.Messages[1].AuthorName != "Scrum Master" {
		t.Errorf("expected second message to be agent Scrum Master, got %s / %s",
			createResp.Messages[1].AuthorType, createResp.Messages[1].AuthorName)
	}

	// 3. Get thread detail via GET /api/v1/inbox/{id}
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/inbox/%s", threadID), nil)
	req.Header.Set("Authorization", "Bearer "+authToken)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var detailResp struct {
		Thread   InboxThreadItem        `json:"thread"`
		Messages []db.ProjectMessageDTO `json:"messages"`
		Tasks    []db.ProjectTaskDTO    `json:"tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("failed to decode detail response: %v", err)
	}
	if detailResp.Thread.Status != "unread" {
		t.Errorf("expected initial status unread, got %s", detailResp.Thread.Status)
	}
	if len(detailResp.Messages) != 2 {
		t.Errorf("expected 2 messages in detail, got %d", len(detailResp.Messages))
	}

	// 4. Post reply message via POST /api/v1/inbox/{id}/messages
	replyPayload := CreateInboxMessageRequest{
		Content:    "Looks good. Please proceed with Antigravity Flash.",
		AuthorType: "user",
		AuthorName: "Product Owner",
	}
	body, _ = json.Marshal(replyPayload)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/inbox/%s/messages", threadID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on reply, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. 1-Click Approval via POST /api/v1/inbox/{id}/approve
	approvePayload := ApproveInboxThreadRequest{
		Title:    "Fix Safari Mobile login padding",
		Assignee: "antigravity-flash",
	}
	body, _ = json.Marshal(approvePayload)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/inbox/%s/approve", threadID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on approve, got %d: %s", rec.Code, rec.Body.String())
	}

	var approveResp struct {
		Status  string            `json:"status"`
		Task    db.ProjectTaskDTO `json:"task"`
		Message db.ProjectMessageDTO `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &approveResp); err != nil {
		t.Fatalf("failed to decode approve response: %v", err)
	}
	if approveResp.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", approveResp.Status)
	}
	if approveResp.Task.Status != "doing" {
		t.Errorf("expected task status doing, got %s", approveResp.Task.Status)
	}

	// 6. Verify status update in thread list filter
	req = httptest.NewRequest(http.MethodGet, "/api/v1/inbox?status=in_progress", nil)
	req.Header.Set("Authorization", "Bearer "+authToken)
	rec = httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var filteredResp struct {
		Threads []InboxThreadItem `json:"threads"`
		Total   int               `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &filteredResp)
	if filteredResp.Total != 1 {
		t.Errorf("expected 1 in_progress thread, got %d", filteredResp.Total)
	}
}

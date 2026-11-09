package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	_ "modernc.org/sqlite"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/resend"
)

type mockResendClient struct {
	mu           sync.Mutex
	validateErr  error
	sendErr      error
	sentEmails   []resend.EmailRequest
	validateRuns int
}

func (m *mockResendClient) ValidateAPIKey(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validateRuns++
	return m.validateErr
}

func (m *mockResendClient) SendEmail(ctx context.Context, req resend.EmailRequest) (*resend.EmailResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	m.sentEmails = append(m.sentEmails, req)
	return &resend.EmailResponse{ID: "re_mock_email_123"}, nil
}

func (m *mockResendClient) getSentEmails() []resend.EmailRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]resend.EmailRequest, len(m.sentEmails))
	copy(copied, m.sentEmails)
	return copied
}

func TestResendSettingsPersistenceAndMasking(t *testing.T) {
	srv, token := setupTestChatServer(t)
	srv.cfg.WorkspacesDir = filepath.Join(t.TempDir(), "workspaces")

	chatIdleDetectorMu.Lock()
	chatIdleHistory = make(map[string]chatIdleNotificationRecord)
	chatIdleTimers = make(map[string]*time.Timer)
	chatIdleDetectorMu.Unlock()

	// 1. Initial settings should be empty / unconfigured
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/resend", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status   string            `json:"status"`
		Settings ResendSettingsDTO `json:"settings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}

	if resp.Settings.HasAPIKey {
		t.Fatalf("expected has_api_key = false initially")
	}

	// 2. Save new Resend settings
	enabled := true
	savePayload := map[string]any{
		"api_key":                "re_1234567890abcdef",
		"from_email":             "notif@cuan.store",
		"from_name":              "Cuan Toko Saya",
		"notification_email":     "owner@example.com",
		"idle_threshold_minutes": 5,
		"enabled":                &enabled,
	}
	rawSave, _ := json.Marshal(savePayload)
	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/resend", bytes.NewReader(rawSave))
	saveReq.Header.Set("Authorization", "Bearer "+token)
	saveReq.Header.Set("Content-Type", "application/json")
	wSave := httptest.NewRecorder()
	srv.Router().ServeHTTP(wSave, saveReq)

	if wSave.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wSave.Code, wSave.Body.String())
	}

	var saveResp struct {
		Status   string            `json:"status"`
		Message  string            `json:"message"`
		Settings ResendSettingsDTO `json:"settings"`
	}
	if err := json.Unmarshal(wSave.Body.Bytes(), &saveResp); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	if !saveResp.Settings.HasAPIKey {
		t.Errorf("expected has_api_key = true")
	}
	if !strings.Contains(saveResp.Settings.MaskedAPIKey, "*") {
		t.Errorf("expected masked API key with asterisks, got %q", saveResp.Settings.MaskedAPIKey)
	}
	if strings.Contains(wSave.Body.String(), "re_1234567890abcdef") {
		t.Errorf("plain api key leaked in HTTP response body!")
	}
	if saveResp.Settings.IdleThresholdMinutes != 5 {
		t.Errorf("expected idle_threshold_minutes = 5, got %d", saveResp.Settings.IdleThresholdMinutes)
	}
	if saveResp.Settings.NotificationEmail != "owner@example.com" {
		t.Errorf("expected notification_email = owner@example.com, got %s", saveResp.Settings.NotificationEmail)
	}
}

func TestResendAPIKeyValidationEndpoint(t *testing.T) {
	srv, token := setupTestChatServer(t)

	mock := &mockResendClient{validateErr: nil}
	srv.SetResendClient(mock)

	// Test valid API key validation
	valReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/resend/validate", strings.NewReader(`{"api_key":"re_validkey123"}`))
	valReq.Header.Set("Authorization", "Bearer "+token)
	valReq.Header.Set("Content-Type", "application/json")
	wVal := httptest.NewRecorder()
	srv.Router().ServeHTTP(wVal, valReq)

	if wVal.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wVal.Code, wVal.Body.String())
	}
	if !strings.Contains(wVal.Body.String(), `"valid":true`) {
		t.Fatalf("expected valid: true, got %s", wVal.Body.String())
	}

	// Test invalid API key validation
	mock.validateErr = errors.New("unauthorized 401")
	valReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/settings/resend/validate", strings.NewReader(`{"api_key":"re_badkey"}`))
	valReq2.Header.Set("Authorization", "Bearer "+token)
	valReq2.Header.Set("Content-Type", "application/json")
	wVal2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(wVal2, valReq2)

	if wVal2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wVal2.Code)
	}
	if !strings.Contains(wVal2.Body.String(), `"valid":false`) {
		t.Fatalf("expected valid: false, got %s", wVal2.Body.String())
	}
}

func TestResendTestEmailEndpoint(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// Save settings first
	_ = srv.SaveResendSettings(context.Background(), &ResendSettings{
		APIKey:            "re_dummy_for_test",
		FromEmail:         "noreply@cuan.store",
		FromName:          "Cuan Store",
		NotificationEmail: "admin@cuan.store",
		Enabled:           true,
	})

	mock := &mockResendClient{}
	srv.SetResendClient(mock)

	testReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/resend/test", strings.NewReader(`{"recipient_email":"admin@cuan.store"}`))
	testReq.Header.Set("Authorization", "Bearer "+token)
	testReq.Header.Set("Content-Type", "application/json")
	wTest := httptest.NewRecorder()
	srv.Router().ServeHTTP(wTest, testReq)

	if wTest.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wTest.Code, wTest.Body.String())
	}

	sent := mock.getSentEmails()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent test email, got %d", len(sent))
	}
	if sent[0].To[0] != "admin@cuan.store" {
		t.Errorf("expected recipient admin@cuan.store, got %s", sent[0].To[0])
	}
	if !strings.Contains(sent[0].Subject, "Email pengujian Cuan") {
		t.Errorf("expected subject to contain test message, got %s", sent[0].Subject)
	}
}

func TestChatIdleDetectionFlow(t *testing.T) {
	srv, token := setupTestChatServer(t)
	srv.cfg.WorkspacesDir = filepath.Join(t.TempDir(), "workspaces")

	chatIdleDetectorMu.Lock()
	chatIdleHistory = make(map[string]chatIdleNotificationRecord)
	chatIdleTimers = make(map[string]*time.Timer)
	chatIdleDetectorMu.Unlock()

	// Configure Resend with 5-minute threshold
	_ = srv.SaveResendSettings(context.Background(), &ResendSettings{
		APIKey:               "re_active_key",
		FromEmail:            "support@cuan.store",
		FromName:             "Cuan Support",
		NotificationEmail:    "operator@cuan.store",
		IdleThresholdMinutes: 5,
		Enabled:              true,
	})

	mock := &mockResendClient{}
	srv.SetResendClient(mock)

	roomID := cuanDB.GenerateUUID()
	now := time.Now().UTC()

	// Create room
	_ = srv.projectStudioRepo.CreateRoom(context.Background(), &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Pelanggan Budi",
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
	})

	// 1. Message created just 1 minute ago (timeSince < 5m threshold) -> SHOULD NOT SEND
	recentMsg := &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "user",
		AuthorName: "Budi",
		Content:    "Halo apakah stok ready?",
		CreatedAt:  now.Add(-1 * time.Minute),
		UpdatedAt:  now.Add(-1 * time.Minute),
	}
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), recentMsg)

	sent, err := srv.TriggerRoomIdleNotification(context.Background(), roomID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent {
		t.Errorf("expected not to trigger idle email for recent message (<5m)")
	}
	if len(mock.getSentEmails()) != 0 {
		t.Errorf("no email should be sent yet")
	}

	// 2. Room with idle message older than 5m threshold -> SHOULD SEND
	roomID2 := cuanDB.GenerateUUID()
	_ = srv.projectStudioRepo.CreateRoom(context.Background(), &cuanDB.ProjectRoom{
		ID:        roomID2,
		ProjectID: "default",
		Title:     "Pelanggan Budi",
		CreatedAt: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-10 * time.Minute),
	})
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID2,
		AuthorType: "user",
		AuthorName: "Budi",
		Content:    "Tolong dibalas ya min",
		CreatedAt:  now.Add(-6 * time.Minute),
		UpdatedAt:  now.Add(-6 * time.Minute),
	})

	sent2, err := srv.TriggerRoomIdleNotification(context.Background(), roomID2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sent2 {
		t.Errorf("expected idle email to be triggered for idle message (>5m)")
	}

	sentEmails := mock.getSentEmails()
	if len(sentEmails) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sentEmails))
	}
	if sentEmails[0].To[0] != "operator@cuan.store" {
		t.Errorf("expected recipient operator@cuan.store, got %s", sentEmails[0].To[0])
	}
	if !strings.Contains(sentEmails[0].Subject, "Pesan belum dibaca dari Budi") {
		t.Errorf("subject expected to mention customer name Budi: %s", sentEmails[0].Subject)
	}
	if !strings.Contains(sentEmails[0].HTML, "Tolong dibalas ya min") {
		t.Errorf("HTML transcript expected to include unread message body")
	}

	// A restart loses the memory cache; the persisted send must still suppress replay.
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.Exec(`CREATE TABLE resend_email_logs(room_id TEXT, status TEXT, sent_at TIMESTAMP, resend_email_id TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`CREATE TABLE chat_message_emails(message_id TEXT, email_id TEXT, recipient_type TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO resend_email_logs VALUES(?, 'sent', ?, 'legacy')`, roomID2, now.Add(-6*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	srv.db = database
	chatIdleDetectorMu.Lock()
	delete(chatIdleHistory, roomID2)
	chatIdleDetectorMu.Unlock()
	if replayed, err := srv.TriggerRoomIdleNotification(context.Background(), roomID2); err != nil || replayed {
		t.Fatalf("persisted same-message notification replayed after restart: sent=%v err=%v", replayed, err)
	}
	if len(mock.getSentEmails()) != 1 {
		t.Fatal("restart sent duplicate email")
	}
	// Exact message receipts remain authoritative even after legacy log retention.
	messages, err := srv.projectStudioRepo.ListMessagesByRoomID(context.Background(), roomID2)
	if err != nil || len(messages) != 1 {
		t.Fatal("missing customer fixture", err)
	}
	_, err = database.Exec(`DELETE FROM resend_email_logs`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO chat_message_emails VALUES(?, 'sent-email', 'operator')`, messages[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if sent, err := srv.TriggerRoomIdleNotification(context.Background(), roomID2); err != nil || sent {
		t.Fatalf("exact persisted receipt replayed: sent=%v err=%v", sent, err)
	}
	_, _ = database.Exec(`DELETE FROM chat_message_emails`)
	// Database errors must fail closed, rather than replaying historical email.
	_, _ = database.Exec(`DROP TABLE resend_email_logs`)
	if sent, err := srv.TriggerRoomIdleNotification(context.Background(), roomID2); err == nil || sent {
		t.Fatal("missing durable history must not permit sending")
	}
	srv.db = nil
	chatIdleDetectorMu.Lock()
	chatIdleHistory[roomID2] = chatIdleNotificationRecord{MessageID: "fixture", NotifiedAt: time.Now()}
	chatIdleDetectorMu.Unlock()

	// 3. Immediately trigger again -> Anti-inbox bombing cooldown should prevent 2nd email
	sent3, err := srv.TriggerRoomIdleNotification(context.Background(), roomID2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent3 {
		t.Errorf("anti-inbox bombing cooldown should prevent second notification")
	}
	if len(mock.getSentEmails()) != 1 {
		t.Errorf("still only 1 email should have been sent due to cooldown")
	}

	// 4. Operator sends reply -> idle timer & answered status resets
	operatorMsg := &cuanDB.ProjectMessage{
		ID:         cuanDB.GenerateUUID(),
		RoomID:     roomID2,
		AuthorType: "user",
		AuthorID:   "operator_user",
		AuthorName: "Owner",
		Content:    "Halo Budi, ready ya silahkan diorder!",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	_ = srv.projectStudioRepo.CreateMessage(context.Background(), operatorMsg)

	// Reset history cooldown for room to test answered state
	chatIdleDetectorMu.Lock()
	delete(chatIdleHistory, roomID2)
	chatIdleDetectorMu.Unlock()

	sent4, err := srv.TriggerRoomIdleNotification(context.Background(), roomID2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent4 {
		t.Errorf("expected not to trigger email since operator has replied")
	}

	// 5. Check trigger via public endpoint POST /api/chat/idle/check
	scanReq := httptest.NewRequest(http.MethodPost, "/api/chat/idle/check", nil)
	scanReq.Header.Set("Authorization", "Bearer "+token)
	wScan := httptest.NewRecorder()
	srv.Router().ServeHTTP(wScan, scanReq)

	if wScan.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wScan.Code, wScan.Body.String())
	}
}

func TestChatIdleSchedulerJobRegistered(t *testing.T) {
	srv, _ := setupTestChatServer(t)

	if srv.scheduler == nil {
		t.Fatalf("server scheduler is nil")
	}

	job, exists := srv.scheduler.GetJob("chat_idle_email_digest")
	if !exists || job == nil {
		t.Fatalf("job chat_idle_email_digest is not registered in scheduler")
	}
	if !job.Enabled {
		t.Errorf("expected chat_idle_email_digest job to be enabled")
	}
	if job.Interval != 1*time.Minute {
		t.Errorf("expected interval 1m, got %v", job.Interval)
	}
}

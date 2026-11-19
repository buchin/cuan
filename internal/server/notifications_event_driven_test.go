package server

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type pushPayloadSpyClient struct {
	mu       sync.Mutex
	calls    int
	requests []*http.Request
	status   int
}

func (c *pushPayloadSpyClient) RoundTrip(r *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.requests = append(c.requests, r)
	if r.Header.Get("Content-Encoding") != "aes128gcm" || !strings.Contains(r.Header.Get("Authorization"), "vapid") {
		panic("missing Web Push encryption/signature")
	}
	status := c.status
	if status == 0 {
		status = http.StatusCreated
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func (c *pushPayloadSpyClient) CallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func fixturePushSub(t *testing.T, endpoint string) webpush.Subscription {
	t.Helper()
	_, x, y, e := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	authKey := make([]byte, 16)
	rand.Read(authKey)
	return webpush.Subscription{
		Endpoint: endpoint,
		Keys: webpush.Keys{
			P256dh: base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y)),
			Auth:   base64.RawURLEncoding.EncodeToString(authKey),
		},
	}
}

func setupStaffPushTest(t *testing.T) (*Server, string, string, string, string) {
	t.Helper()
	s, ownerToken, _, _ := inboxFixture(t)
	ctx := context.Background()

	// 1. Get Owner user
	owner, err := s.authSvc.ValidateToken(ctx, ownerToken)
	if err != nil {
		t.Fatalf("validate owner token: %v", err)
	}

	// 2. Invite a staff Member
	inviteResp, err := s.authSvc.CreateInvite(ctx, owner.ID, "member@dropsugar.local", auth.RoleMember)
	if err != nil {
		t.Fatalf("create staff invite: %v", err)
	}

	// 3. Member accepts invitation
	acceptResp, err := s.authSvc.AcceptInvite(ctx, inviteResp.Token, "Siti Admin", "memberSecretPass123")
	if err != nil {
		t.Fatalf("accept staff invite: %v", err)
	}
	memberToken := acceptResp.Token
	memberID := acceptResp.User.ID

	// 4. Register push subscriptions for Owner
	ownerSub := fixturePushSub(t, "https://fcm.googleapis.com/fcm/send/owner-device-1")
	ownerPrefs := defaultOwnerPreferences()
	ownerPrefs.PrivateKey, ownerPrefs.PublicKey, _ = webpush.GenerateVAPIDKeys()
	ownerPrefs.PushSubject = "mailto:owner@dropsugar.local"
	ownerPrefs.Subscriptions = []ownerPushSubscription{
		{Subscription: ownerSub, Since: time.Now().Add(-1 * time.Hour), Sent: map[string]bool{}},
	}
	if err := writeOwnerPreferences(owner.ID, ownerPrefs); err != nil {
		t.Fatalf("write owner preferences: %v", err)
	}

	// 5. Register push subscriptions for Member
	memberSub := fixturePushSub(t, "https://fcm.googleapis.com/fcm/send/member-device-1")
	memberPrefs := defaultOwnerPreferences()
	memberPrefs.PrivateKey, memberPrefs.PublicKey, _ = webpush.GenerateVAPIDKeys()
	memberPrefs.PushSubject = "mailto:member@dropsugar.local"
	memberPrefs.Subscriptions = []ownerPushSubscription{
		{Subscription: memberSub, Since: time.Now().Add(-1 * time.Hour), Sent: map[string]bool{}},
	}
	if err := writeOwnerPreferences(memberID, memberPrefs); err != nil {
		t.Fatalf("write member preferences: %v", err)
	}

	return s, owner.ID, ownerToken, memberID, memberToken
}

// TestEventDrivenPush_VisitorLiveChatMessage verifies:
// 1. Instant event-driven push dispatch upon visitor live chat message.
// 2. Multi-user broadcast to all active staff (Owner and Member).
// 3. Dynamic payload: title (Visitor Name), body (sanitized snippet), href (/inbox/<room_id>), tag (customer:<room_id>:<msg_id>).
// 4. Background ticker does not duplicate already-dispatched push notifications.
func TestEventDrivenPush_VisitorLiveChatMessage(t *testing.T) {
	s, ownerID, _, memberID, _ := setupStaffPushTest(t)
	ctx := context.Background()

	spy := &pushPayloadSpyClient{status: 201}
	prevHTTPClient := notificationHTTPClient
	notificationHTTPClient = &http.Client{Transport: spy}
	defer func() { notificationHTTPClient = prevHTTPClient }()

	visitorName := "Budi Hartono"
	messageText := "Halo Kak, apakah sepatu lari ukuran 42 warna hitam masih ready stock?"

	// 1. Initialize visitor live chat session via POST /api/chat/session
	initPayload, _ := json.Marshal(map[string]any{
		"guest_uuid": "guest-visitor-123",
		"product":    "cuan-store",
		"name":       visitorName,
		"email":      "budi@example.com",
	})
	reqInit := httptest.NewRequest("POST", "/api/chat/session", strings.NewReader(string(initPayload)))
	reqInit.Header.Set("Content-Type", "application/json")
	recInit := httptest.NewRecorder()
	s.Router().ServeHTTP(recInit, reqInit)

	if recInit.Code != http.StatusOK {
		t.Fatalf("expected 200 from chat session init, got %d: %s", recInit.Code, recInit.Body.String())
	}
	var initResp struct {
		Session struct {
			UUID string `json:"uuid"`
		} `json:"session"`
	}
	if err := json.Unmarshal(recInit.Body.Bytes(), &initResp); err != nil || initResp.Session.UUID == "" {
		t.Fatalf("expected valid session UUID, got: %s", recInit.Body.String())
	}
	roomUUID := initResp.Session.UUID

	// 2. Simulate visitor sending a message via POST /api/chat/messages
	reqBody, _ := json.Marshal(map[string]any{
		"session_uuid": roomUUID,
		"message":      messageText,
		"author_name":  visitorName,
	})
	r := httptest.NewRequest("POST", "/api/chat/messages", strings.NewReader(string(reqBody)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 from chat messages, got %d: %s", w.Code, w.Body.String())
	}

	// 1. Assert: Push was sent immediately (without waiting for 30s background ticker)
	// Both Owner and Member should receive 1 push each -> total 2 calls
	if spy.CallCount() != 2 {
		t.Fatalf("expected 2 instant push calls (1 for owner, 1 for member), got %d", spy.CallCount())
	}

	// 2. Verify payload attributes on owner & member preferences
	ownerPrefs, _ := readOwnerPreferences(ownerID)
	memberPrefs, _ := readOwnerPreferences(memberID)

	if len(ownerPrefs.Items) == 0 {
		t.Fatal("expected owner notification item saved")
	}
	if len(memberPrefs.Items) == 0 {
		t.Fatal("expected member notification item saved")
	}

	item := ownerPrefs.Items[0]
	if item.Kind != "customer" {
		t.Errorf("expected item kind 'customer', got %q", item.Kind)
	}
	if !strings.Contains(item.Title, visitorName) {
		t.Errorf("expected item title to contain %q, got %q", visitorName, item.Title)
	}
	if !strings.Contains(item.Href, roomUUID) {
		t.Errorf("expected item href to contain room %q, got %q", roomUUID, item.Href)
	}

	// Test payload generator directly to verify exact dynamic keys
	payload := buildPushPayload(item)
	expectedTitle := "Pesan Baru dari " + visitorName
	if payload["title"] != expectedTitle {
		t.Errorf("expected push payload title %q, got %q", expectedTitle, payload["title"])
	}
	if payload["body"] != messageText {
		t.Errorf("expected push payload body %q, got %q", messageText, payload["body"])
	}
	expectedHref := "/inbox/" + roomUUID
	if payload["href"] != expectedHref {
		t.Errorf("expected push payload href %q, got %q", expectedHref, payload["href"])
	}
	if !strings.HasPrefix(payload["tag"], "customer:"+roomUUID+":") {
		t.Errorf("expected tag prefix 'customer:%s:', got %q", roomUUID, payload["tag"])
	}

	// 3. Fallback Ticker Reliability: Run 30s fallback ticker dispatcher
	// Verify that already-sent push is NOT duplicated!
	s.dispatchAllStaff(ctx)
	if spy.CallCount() != 2 {
		t.Fatalf("expected no duplicated push calls after ticker sync, still got %d calls", spy.CallCount())
	}
}

// TestEventDrivenPush_AgentReplyAndApproval verifies:
// 1. Instant push when agent completes reply (title: "Asisten AI", tag: "agent:<room_id>:<msg_id>", href: "/inbox/<room_id>").
// 2. Instant push when agent proposes a plan requiring approval (cuan-approval).
func TestEventDrivenPush_AgentReplyAndApproval(t *testing.T) {
	s, ownerID, _, memberID, _ := setupStaffPushTest(t)
	ctx := context.Background()

	spy := &pushPayloadSpyClient{status: 201}
	prevHTTPClient := notificationHTTPClient
	notificationHTTPClient = &http.Client{Transport: spy}
	defer func() { notificationHTTPClient = prevHTTPClient }()

	roomID := "room-inbox-" + db.GenerateUUID()
	agentMsgID := db.GenerateUUID()
	agentContent := "Halo! Pesanan Anda sedang diproses oleh tim gudang dan akan dikirim sore ini."

	// 1. Trigger agent completed reply
	s.DispatchAgentUpdatePush(ctx, roomID, agentMsgID, "Asisten AI", agentContent, false)

	if spy.CallCount() != 2 {
		t.Fatalf("expected 2 instant pushes for agent reply (owner + member), got %d", spy.CallCount())
	}

	ownerPrefs, _ := readOwnerPreferences(ownerID)
	if len(ownerPrefs.Items) == 0 {
		t.Fatal("expected owner notification recorded")
	}
	agentItem := ownerPrefs.Items[0]
	payload := buildPushPayload(agentItem)

	if payload["title"] != "Asisten AI" {
		t.Errorf("expected title 'Asisten AI', got %q", payload["title"])
	}
	if !strings.Contains(payload["body"], "Pesanan Anda sedang diproses") {
		t.Errorf("expected body snippet from agent response, got %q", payload["body"])
	}
	if payload["href"] != "/inbox/"+roomID {
		t.Errorf("expected href '/inbox/%s', got %q", roomID, payload["href"])
	}
	expectedTag := "agent:" + roomID + ":" + agentMsgID
	if payload["tag"] != expectedTag {
		t.Errorf("expected tag %q, got %q", expectedTag, payload["tag"])
	}

	// 2. Trigger agent approval needed proposal
	approvalMsgID := db.GenerateUUID()
	approvalPlanText := "Rencana Penyesuaian Harga Diskon 10%\n```cuan-approval\n{\"title\":\"Konfirmasi diskon 10% produk sepatu\",\"description\":\"Terapkan promo diskon 10% untuk pelanggan VIP.\",\"project_id\":\"default\"}\n```"

	s.DispatchAgentUpdatePush(ctx, roomID, approvalMsgID, "Asisten AI", approvalPlanText, true)

	// Calls should increase by 2 (owner + member) -> total 4
	if spy.CallCount() != 4 {
		t.Fatalf("expected 4 total push calls after approval proposal, got %d", spy.CallCount())
	}

	ownerPrefs, _ = readOwnerPreferences(ownerID)
	approvalItem := ownerPrefs.Items[0]
	approvalPayload := buildPushPayload(approvalItem)

	if approvalPayload["title"] != "Asisten AI" {
		t.Errorf("expected title 'Asisten AI', got %q", approvalPayload["title"])
	}
	if !strings.Contains(approvalPayload["body"], "Konfirmasi diskon 10% produk sepatu") {
		t.Errorf("expected body to reflect approval plan title, got %q", approvalPayload["body"])
	}
	if approvalPayload["href"] != "/inbox/"+roomID {
		t.Errorf("expected href '/inbox/%s', got %q", roomID, approvalPayload["href"])
	}
	expectedApprovalTag := "agent:" + roomID + ":" + approvalMsgID
	if approvalPayload["tag"] != expectedApprovalTag {
		t.Errorf("expected tag %q, got %q", expectedApprovalTag, approvalPayload["tag"])
	}

	// Fallback ticker should not re-send
	s.dispatchAllStaff(ctx)
	if spy.CallCount() != 4 {
		t.Fatalf("expected no duplicates from background ticker, still %d calls", spy.CallCount())
	}

	_ = memberID
}

// TestEventDrivenPush_QuietHoursAndNotificationPreferences verifies:
// 1. Staff with disabled Customer preference does not receive push.
// 2. Staff with active quiet hours does not receive push during quiet hours.
func TestEventDrivenPush_QuietHoursAndNotificationPreferences(t *testing.T) {
	s, ownerID, _, memberID, _ := setupStaffPushTest(t)
	ctx := context.Background()

	spy := &pushPayloadSpyClient{status: 201}
	prevHTTPClient := notificationHTTPClient
	notificationHTTPClient = &http.Client{Transport: spy}
	defer func() { notificationHTTPClient = prevHTTPClient }()

	// Configure Member with quiet hours active right now
	memberPrefs, _ := readOwnerPreferences(memberID)
	memberPrefs.Notifications.Quiet = true
	nowHour := time.Now().UTC().Hour()
	// Set quiet hours spanning current hour
	memberPrefs.Notifications.Start = (nowHour + 23) % 24 // 1 hour ago
	memberPrefs.Notifications.End = (nowHour + 2) % 24   // 2 hours in future
	memberPrefs.Notifications.Timezone = "UTC"
	_ = writeOwnerPreferences(memberID, memberPrefs)

	// Trigger visitor message
	roomID := "room-quiet-" + db.GenerateUUID()
	msgID := db.GenerateUUID()
	s.DispatchCustomerMessagePush(ctx, roomID, msgID, "Pelanggan Malam", "Halo, apakah toko masih buka?")

	// Only Owner should receive the push! Member's device is in quiet hours.
	if spy.CallCount() != 1 {
		t.Fatalf("expected 1 push call (owner only, member skipped by quiet hours), got %d", spy.CallCount())
	}

	// But notification center item should still be saved for Member
	updatedMemberPrefs, _ := readOwnerPreferences(memberID)
	if len(updatedMemberPrefs.Items) == 0 {
		t.Fatal("expected member notification item recorded in notification center despite quiet hours")
	}

	// Now configure Owner with Customer notifications disabled
	ownerPrefs, _ := readOwnerPreferences(ownerID)
	ownerPrefs.Notifications.Customer = false
	_ = writeOwnerPreferences(ownerID, ownerPrefs)

	// Trigger another visitor message
	msgID2 := db.GenerateUUID()
	s.DispatchCustomerMessagePush(ctx, roomID, msgID2, "Pelanggan Siang", "Cek harga grosir")

	// Neither should receive push (Owner disabled Customer, Member in quiet hours)
	if spy.CallCount() != 1 {
		t.Fatalf("expected no new push call (owner disabled customer, member in quiet hours), got %d", spy.CallCount())
	}
}

// TestPushPayload_SanitizationAndMasking verifies:
// 1. Sensitive secrets in message content are masked before push dispatch.
// 2. Raw HTML tags are stripped.
// 3. Messages exceeding limit are cleanly truncated.
func TestPushPayload_SanitizationAndMasking(t *testing.T) {
	rawSecretMsg := "Halo admin, token rahasia saya adalah sk-1234567890abcdef1234567890abcdef dan password rahasia123"
	snippet := cleanNotificationSnippet(rawSecretMsg)

	// Secrets should be masked
	if strings.Contains(snippet, "sk-1234567890") {
		t.Errorf("expected secret key to be masked, got: %s", snippet)
	}

	htmlMsg := "<div class=\"chat\"><b>Pesan Penting:</b> <script>alert(1)</script>Halo semua!</div>"
	htmlSnippet := cleanNotificationSnippet(htmlMsg)
	if strings.Contains(htmlSnippet, "<div") || strings.Contains(htmlSnippet, "<script>") || strings.Contains(htmlSnippet, "<b>") {
		t.Errorf("expected HTML tags to be stripped, got: %s", htmlSnippet)
	}
	if !strings.Contains(htmlSnippet, "Pesan Penting: alert(1)Halo semua!") {
		t.Errorf("expected text content preserved after stripping HTML, got: %s", htmlSnippet)
	}

	longMsg := strings.Repeat("KataPanjang ", 30) // ~360 chars
	longSnippet := cleanNotificationSnippet(longMsg)
	if len([]rune(longSnippet)) > 100 { // bounded by limit ~100
		t.Errorf("expected truncated snippet <= 100 chars, got length %d: %s", len([]rune(longSnippet)), longSnippet)
	}
	if !strings.HasSuffix(longSnippet, "...") {
		t.Errorf("expected ellipsis suffix, got: %s", longSnippet)
	}
}

// TestEventDrivenPush_InboxLifecycleEndToEnd verifies that when an inbox goal runs
// through the full execution loop, FinishInboxRequest triggers instant push.
func TestEventDrivenPush_InboxLifecycleEndToEnd(t *testing.T) {
	s, _, ownerToken, _, _ := setupStaffPushTest(t)

	spy := &pushPayloadSpyClient{status: 201}
	prevHTTPClient := notificationHTTPClient
	notificationHTTPClient = &http.Client{Transport: spy}
	defer func() { notificationHTTPClient = prevHTTPClient }()

	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+ownerToken)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}

	reqID := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{
		"request_id": reqID,
		"goal":       "Buat ringkasan penjualan mingguan",
	})
	w := call("POST", "/api/v1/studio/goals", string(body))
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		ConversationID string `json:"conversation_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)

	waitInboxState(t, s, res.ConversationID, reqID, "completed")

	// Both Owner and Member should receive push instantly.
	// Wait until push dispatcher finishes sending HTTP requests to all subscribed staff.
	deadline := time.Now().Add(3 * time.Second)
	for spy.CallCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if spy.CallCount() < 2 {
		t.Fatalf("expected at least 2 push calls on inbox goal completion, got %d", spy.CallCount())
	}
}

// TestPushPayload_HrefPreservationAndRouting verifies that buildPushPayload
// preserves explicit task/project/inbox deep links and routes empty Href fallbacks accurately.
func TestPushPayload_HrefPreservationAndRouting(t *testing.T) {
	// Case 1: Explicit task deep link must NOT be rewritten to /inbox/<task_id>
	taskNotification := ownerNotification{
		ID:    "task:task-abc-123:done:attempt1",
		Kind:  "completed",
		Title: "Penyelesaian Tugas",
		Href:  "/tasks/task-abc-123",
	}
	payload1 := buildPushPayload(taskNotification)
	if payload1["href"] != "/tasks/task-abc-123" {
		t.Errorf("expected href '/tasks/task-abc-123', got %q", payload1["href"])
	}

	// Case 2: Explicit project task deep link must be preserved
	projTaskNotification := ownerNotification{
		ID:    "task:task-xyz-789:review:attempt2",
		Kind:  "approval",
		Title: "Review Required",
		Href:  "/projects/proj-1/tasks/task-xyz-789",
	}
	payload2 := buildPushPayload(projTaskNotification)
	if payload2["href"] != "/projects/proj-1/tasks/task-xyz-789" {
		t.Errorf("expected href '/projects/proj-1/tasks/task-xyz-789', got %q", payload2["href"])
	}

	// Case 3: Conversation room deep link preserved
	roomNotification := ownerNotification{
		ID:    "request:room-555:req-1",
		Kind:  "blocker",
		Title: "Reply could not finish",
		Href:  "/inbox/room-555",
	}
	payload3 := buildPushPayload(roomNotification)
	if payload3["href"] != "/inbox/room-555" {
		t.Errorf("expected href '/inbox/room-555', got %q", payload3["href"])
	}

	// Case 4: Empty Href on task tag falls back to /tasks/<task_id>
	emptyTaskNotification := ownerNotification{
		ID:   "task:task-fallback-999:blocked:attempt1",
		Kind: "blocker",
	}
	payload4 := buildPushPayload(emptyTaskNotification)
	if payload4["href"] != "/tasks/task-fallback-999" {
		t.Errorf("expected fallback href '/tasks/task-fallback-999', got %q", payload4["href"])
	}

	// Case 5: Empty Href on agent tag falls back to /inbox/<room_id>
	emptyAgentNotification := ownerNotification{
		ID:   "approval:room-agent-777:msg-1",
		Kind: "approval",
	}
	payload5 := buildPushPayload(emptyAgentNotification)
	if payload5["href"] != "/inbox/room-agent-777" {
		t.Errorf("expected fallback href '/inbox/room-agent-777', got %q", payload5["href"])
	}
}

// TestEventDrivenPush_TaskCompleted verifies instant push when a task reaches 'done'.
func TestEventDrivenPush_TaskCompleted(t *testing.T) {
	s, ownerID, _, _, _ := setupStaffPushTest(t)
	ctx := context.Background()

	spy := &pushPayloadSpyClient{status: 201}
	prevHTTPClient := notificationHTTPClient
	notificationHTTPClient = &http.Client{Transport: spy}
	defer func() { notificationHTTPClient = prevHTTPClient }()

	roomID := "room-inbox-" + db.GenerateUUID()
	completedTask := &task.Task{
		ID:           "task-completed-123",
		Title:        "Auto-scroll ke tugas terakhir",
		Status:       task.TaskStatusDone,
		Summary:      "Fitur auto scroll berhasil diuji dan aktif.",
		SourceRoomID: roomID,
	}

	s.DispatchTaskCompletedPush(ctx, completedTask)

	if spy.CallCount() != 2 {
		t.Fatalf("expected 2 instant pushes for task completion (owner + member), got %d", spy.CallCount())
	}

	ownerPrefs, _ := readOwnerPreferences(ownerID)
	if len(ownerPrefs.Items) == 0 {
		t.Fatal("expected owner notification recorded")
	}
	item := ownerPrefs.Items[0]
	payload := buildPushPayload(item)

	if payload["title"] != "Auto-scroll ke tugas terakhir" {
		t.Errorf("expected title 'Auto-scroll ke tugas terakhir', got %q", payload["title"])
	}
	if !strings.Contains(payload["body"], "Fitur auto scroll berhasil diuji") {
		t.Errorf("expected body to contain task summary, got %q", payload["body"])
	}
	if payload["href"] != "/inbox/"+roomID {
		t.Errorf("expected href '/inbox/%s', got %q", roomID, payload["href"])
	}
}

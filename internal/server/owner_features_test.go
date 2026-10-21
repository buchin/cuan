package server

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestOwnerProfileAndUsername(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	before, _ := s.authSvc.ValidateToken(context.Background(), token)
	if w := call("PATCH", "/api/v1/profile", `{"display_name":"Buchin Example"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	me := call("GET", "/api/v1/auth/me", "")
	if !strings.Contains(me.Body.String(), "Buchin Example") {
		t.Fatal(me.Body.String())
	}
	if w := call("PATCH", "/api/v1/auth/username", `{"username":"new-owner","current_password":"wrong-password"}`); w.Code != 403 {
		t.Fatal(w.Code)
	}
	for _, name := range []string{"ab", "bad name", "bad@example.invalid", "../bad", "UPPER CASE"} {
		raw, _ := json.Marshal(map[string]string{"username": name, "current_password": "whosyourdaddy"})
		if w := call("PATCH", "/api/v1/auth/username", string(raw)); w.Code != 400 {
			t.Fatal(name, w.Code)
		}
	}
	if w := call("PATCH", "/api/v1/auth/username", `{"username":"New-Owner","current_password":"whosyourdaddy"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	after, err := s.authSvc.ValidateToken(context.Background(), token)
	if err != nil || after.ID != before.ID || after.Username != "new-owner" {
		t.Fatal(after, err)
	}
	if _, err := s.authSvc.Login(context.Background(), "admin", "whosyourdaddy"); err == nil {
		t.Fatal("old username still accepted")
	}
	if _, err := s.authSvc.Login(context.Background(), "new-owner", "whosyourdaddy"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.authSvc.Login(context.Background(), before.Email, "whosyourdaddy"); err != nil {
		t.Fatal(err)
	}
	if w := call("PATCH", "/api/v1/profile", `{"display_name":"bad\nname"}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if p, err := readOwnerPreferences(after.ID); err != nil || p.DisplayName != "Buchin Example" {
		t.Fatal(p, err)
	}
}

type pushFixtureClient struct {
	calls  int
	status int
}

func (c *pushFixtureClient) Do(r *http.Request) (*http.Response, error) {
	c.calls++
	if r.Header.Get("Content-Encoding") != "aes128gcm" || !strings.Contains(r.Header.Get("Authorization"), "vapid") {
		panic("missing Web Push encryption/signature")
	}
	return &http.Response{StatusCode: c.status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
}
func fixturePushSubscription(t *testing.T) webpush.Subscription {
	t.Helper()
	_, x, y, e := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	auth := make([]byte, 16)
	rand.Read(auth)
	return webpush.Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/test-fixture", Keys: webpush.Keys{P256dh: base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y)), Auth: base64.RawURLEncoding.EncodeToString(auth)}}
}
func TestOwnerNotificationsAndPush(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	ctx := context.Background()
	owner, _ := s.authSvc.ValidateToken(ctx, token)
	keyResponse := call("GET", "/api/v1/notifications/push", "")
	if keyResponse.Code != 200 || strings.Contains(keyResponse.Body.String(), "private") {
		t.Fatal(keyResponse.Code, keyResponse.Body.String())
	}
	sub := fixturePushSubscription(t)
	raw, _ := json.Marshal(sub)
	if w := call("POST", "/api/v1/notifications/push", string(raw)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("POST", "/api/v1/notifications/push", string(raw)); w.Code != 200 {
		t.Fatal(w.Code)
	}
	p, _ := readOwnerPreferences(owner.ID)
	if len(p.Subscriptions) != 1 {
		t.Fatal("duplicate push subscription")
	}
	invalid := sub
	invalid.Endpoint = "https://127.0.0.1/private"
	if validatePushSubscription(invalid) {
		t.Fatal("private endpoint allowed")
	}
	invalid = sub
	invalid.Keys.Auth = "bad"
	if validatePushSubscription(invalid) {
		t.Fatal("invalid encryption keys allowed")
	}
	created, e := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Notification fixture", Instructions: "Fixture", ProjectID: "default"})
	if e != nil {
		t.Fatal(e)
	}
	status := "blocked"
	if _, e = s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &status}); e != nil {
		t.Fatal(e)
	}
	w := call("GET", "/api/v1/notifications", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var feed struct{ Items []ownerNotification }
	json.Unmarshal(w.Body.Bytes(), &feed)
	if len(feed.Items) != 1 || feed.Items[0].Kind != "blocker" {
		t.Fatal(feed)
	}
	// Fake HTTP transport only: verify encrypted delivery, dedupe and expired-device cleanup.
	client := &pushFixtureClient{status: 201}
	previous := notificationHTTPClient
	notificationHTTPClientForTests := &http.Client{Transport: roundTripPushFixture{client}}
	notificationHTTPClient = notificationHTTPClientForTests
	defer func() { notificationHTTPClient = previous }()
	s.dispatchOwnerNotifications(ctx, owner.ID)
	s.dispatchOwnerNotifications(ctx, owner.ID)
	if client.calls != 1 {
		t.Fatal("push replayed", client.calls)
	}
	if w := call("PATCH", "/api/v1/notifications", `{"all":true}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	restored, _ := readOwnerPreferences(owner.ID)
	if !restored.Items[0].Read {
		t.Fatal("read state not durable")
	}
	prefs := defaultOwnerPreferences().Notifications
	prefs.Quiet = true
	prefs.Start = 22
	prefs.End = 8
	prefs.Timezone = "Asia/Jakarta"
	if !quietAt(prefs, time.Date(2026, 10, 20, 16, 0, 0, 0, time.UTC)) || quietAt(prefs, time.Date(2026, 10, 20, 5, 0, 0, 0, time.UTC)) {
		t.Fatal("quiet hours timezone mismatch")
	}
	prefs.End = 22
	if validNotificationPreferences(prefs) {
		t.Fatal("equal quiet hours accepted")
	}
	client.status = 410
	nextStatus := "review"
	if _, err := s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &nextStatus}); err != nil {
		t.Fatal(err)
	}
	s.dispatchOwnerNotifications(ctx, owner.ID)
	expired, _ := readOwnerPreferences(owner.ID)
	if len(expired.Subscriptions) != 0 {
		t.Fatal("expired push endpoint retained")
	}
	if w := call("POST", "/api/v1/notifications/push", string(raw)); w.Code != 200 {
		t.Fatal(w.Code)
	}

	if w := call("DELETE", "/api/v1/notifications/push", string(raw)); w.Code != 200 {
		t.Fatal(w.Code)
	}
	p, _ = readOwnerPreferences(owner.ID)
	if len(p.Subscriptions) != 0 {
		t.Fatal("subscription not removed")
	}
}

type roundTripPushFixture struct{ client *pushFixtureClient }

func (r roundTripPushFixture) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.client.Do(req)
}

func TestApprovalAndCustomerNotifications(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	ctx := context.Background()
	owner, _ := s.authSvc.ValidateToken(ctx, token)
	content := "Proposed scope. Shall I proceed?\n```cuan-approval\n{\"title\":\"Approve the proposed scope\",\"description\":\"Review the proposed scope and report findings.\",\"project_id\":\"default\"}\n```"
	if _, title := splitInboxApproval("user", content); title != "" {
		t.Fatal("user text became agent approval")
	}
	raw, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": coordinatorFixtureResponse(content)}})
	script := "#!/bin/sh\nprintf '%s' '" + string(raw) + "'\n"
	if err := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	id := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{"request_id": id, "goal": "Prepare a plan; ask before executing"})
	w := call("POST", "/api/v1/studio/goals", string(body))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		ConversationID string `json:"conversation_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	waitInboxState(t, s, result.ConversationID, id, "completed")
	items, err := s.notificationCandidates(ctx, owner.ID)
	if err != nil || len(items) != 1 || items[0].Kind != "approval" {
		t.Fatal(items, err)
	}
	messages := call("GET", "/api/v1/chat/conversations/"+result.ConversationID+"/messages", "")
	if !strings.Contains(messages.Body.String(), `"approval_title":"Approve the proposed scope"`) || strings.Contains(messages.Body.String(), "cuan-approval") {
		t.Fatal("approval marker leaked", messages.Body.String())
	}
	tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(tasks) != 0 {
		t.Fatal("approval marker executed work")
	}
	customer, err := s.crmUpsert(ctx, "guest_notification", "fixture", "Customer", "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: customer.SessionUUID, ProjectID: "default", Title: "Customer", Topic: "fixture", CreatedAt: now, UpdatedAt: now})
	for _, author := range []string{"guest_notification", "operator_user"} {
		if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: customer.SessionUUID, AuthorID: author, AuthorType: "user", Content: "Fixture message", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	items, err = s.notificationCandidates(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range items {
		if item.Kind == "customer" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("owner reply counted as customer message", items)
	}
}

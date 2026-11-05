package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/buchin/cuan/internal/db"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func receiptFixture(t *testing.T) (*Server, string, string, []string) {
	t.Helper()
	s, token := setupTestChatServer(t)
	ctx := context.Background()
	contact, err := s.crmUpsert(ctx, "receipts", db.GenerateUUID(), "Fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	room := contact.SessionUUID
	if err = s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Receipt fixture"}); err != nil {
		t.Fatal(err)
	}
	ids := []string{db.GenerateUUID(), db.GenerateUUID(), db.GenerateUUID()}
	for i, id := range ids {
		m := &db.ProjectMessage{ID: id, RoomID: room, AuthorType: "user", AuthorID: "operator_user", Content: "Public response"}
		if i == 1 {
			m.AuthorID = "visitor"
		}
		if i == 2 {
			m.Content = "@cuan private"
		}
		if err = s.projectStudioRepo.CreateMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	return s, token, room, ids
}
func TestChatReceiptsScopeReadAndPolling(t *testing.T) {
	s, token, room, ids := receiptFixture(t)
	sess, _ := s.crmSession(context.Background(), room)
	call := func(path string, payload any, owner bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(payload)
		r := httptest.NewRequest("POST", path, strings.NewReader(string(raw)))
		if owner {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	input := map[string]any{"session_uuid": room, "guest_uuid": "wrong", "read_ids": ids}
	if w := call("/api/chat/receipts", input, false); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	input["guest_uuid"] = sess.GuestUUID
	if w := call("/api/chat/receipts", input, false); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	store := s.projectStudioRepo.(db.ChatReceiptRepository)
	first, _ := store.MessageReceipts(context.Background(), room)
	if len(first) != 1 || first[ids[0]].ReadConfirmedAt == nil || first[ids[0]].DeliveredAt == nil {
		t.Fatal("opposite public message scope", first)
	}
	if w := call("/api/chat/receipts", input, false); w.Code != 200 {
		t.Fatal(w.Code)
	}
	again, _ := store.MessageReceipts(context.Background(), room)
	if !again[ids[0]].ReadConfirmedAt.Equal(*first[ids[0]].ReadConfirmedAt) {
		t.Fatal("duplicate changed timestamp")
	}
	if w := call("/api/v1/chat/conversations/"+room+"/receipts", map[string]any{"read_ids": ids}, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	result, _ := store.MessageReceipts(context.Background(), room)
	if len(result) != 2 || result[ids[1]].ReadConfirmedAt == nil {
		t.Fatal("owner opposite scope", result)
	}
	foreign := db.GenerateUUID()
	input["session_uuid"] = foreign
	if w := call("/api/chat/receipts", input, false); w.Code != 403 {
		t.Fatal("foreign session accepted", w.Code)
	}
	// Incremental polls still carry receipts for previously received visitor messages.
	r := httptest.NewRequest("GET", "/api/chat/messages?session_uuid="+room+"&after_id=99999", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	var response struct {
		Messages []ChatMessageDTO `json:"messages"`
		Receipts []map[string]any `json:"receipts"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response.Messages) != 0 || len(response.Receipts) != 1 {
		t.Fatal(w.Body.String())
	}
}

func TestChatEmailAndReceiptsPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, _, room, ids := receiptFixture(t)
		ctx := context.Background()
		memory := s.projectStudioRepo
		_, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url,default_branch,clone_status) VALUES('default','Fixture','','main','pending');`)
		if err != nil {
			t.Fatal(err)
		}
		repo := db.NewSQLProjectStudioRepository(database)
		rm, _ := memory.GetRoomByID(ctx, room)
		if err = repo.CreateRoom(ctx, rm); err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			m, _ := memory.GetMessageByID(ctx, id)
			if err = repo.CreateMessage(ctx, m); err != nil {
				t.Fatal(err)
			}
		}
		if err = repo.AcknowledgeMessages(ctx, room, []string{ids[0]}, true); err != nil {
			t.Fatal(err)
		}
		restarted := db.NewSQLProjectStudioRepository(database)
		receipts, err := restarted.MessageReceipts(ctx, room)
		if err != nil || receipts[ids[0]].ReadConfirmedAt == nil {
			t.Fatal("receipt persistence", err)
		}
		s.db = database
		s.SetProjectStudioRepository(repo)
		settings := &ResendSettings{WebhookSecret: "whsec_Zml4dHVyZS1zaWduaW5nLXNlY3JldA=="}
		s.cfg.WorkspacesDir = t.TempDir() + "/projects"
		if err = s.SaveResendSettings(ctx, settings); err != nil {
			t.Fatal(err)
		}
		verifier, _ := newResendWebhook(settings.WebhookSecret)
		eventTime := time.Now().UTC()
		emailID := "fixture-email"
		post := func(kind string, stamp time.Time, valid bool) int {
			body, _ := json.Marshal(map[string]any{"type": kind, "created_at": stamp, "data": map[string]string{"email_id": emailID}})
			r := httptest.NewRequest("POST", "/api/webhooks/resend", strings.NewReader(string(body)))
			r.Header.Set("svix-id", "fixture-event")

			// Signature timestamp is independent of provider event ordering.
			now := time.Now()
			signature, _ := verifier.Sign("fixture-event", now, body)
			r.Header.Set("svix-timestamp", fmt.Sprint(now.Unix()))
			if valid {
				r.Header.Set("svix-signature", signature)
			}
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, r)
			return w.Code
		}
		if code := post("email.delivered", eventTime, false); code != 400 {
			t.Fatal("unsigned event accepted", code)
		}
		if code := post("email.delivered", eventTime, true); code != 200 {
			t.Fatal("early webhook", code)
		}
		if err = s.recordChatEmail(ctx, room, "fixture-email", []string{ids[1]}, "operator"); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"email.sent", "email.delivered", "email.opened"} {
			if code := post(kind, eventTime, true); code != 200 {
				t.Fatal(kind, code)
			}
		}
		delivery, err := s.chatEmailDeliveries(ctx, room)
		if err != nil || delivery[ids[1]] == nil || delivery[ids[1]].Status != "delivered" || delivery[ids[1]].RecipientType != "operator" {
			t.Fatal(delivery, err)
		}
		receipts, _ = repo.MessageReceipts(ctx, room)
		if receipts[ids[1]].ReadConfirmedAt != nil {
			t.Fatal("email set chat read")
		}
		// Delivery may happen before the send API response, but its webhook can
		// arrive after local mapping. Local clock ordering must not hide delivery.
		emailID = "fixture-delayed-webhook"
		if err = s.recordChatEmail(ctx, room, emailID, []string{ids[2]}, "operator"); err != nil {
			t.Fatal(err)
		}
		if code := post("email.delivered", eventTime.Add(-time.Minute), true); code != 200 {
			t.Fatal(code)
		}
		delivery, err = s.chatEmailDeliveries(ctx, room)
		if err != nil || delivery[ids[2]] == nil || delivery[ids[2]].Status != "delivered" {
			t.Fatal("local send clock blocked delivery", delivery, err)
		}
	})
}

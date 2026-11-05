package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestInboxApprovalDecisionsStayPrivateAcrossVisitorAPIs(t *testing.T) {
	s, _, room, _ := receiptFixture(t)
	ctx := context.Background()
	private := map[string]bool{}
	for _, action := range []string{"", "approve", "defer"} {
		raw, err := json.Marshal(attachmentMessage{Text: "Approved: Langkah berikutnya", Approval: &approvedInboxPlan{Action: action, MessageID: db.GenerateUUID(), Plan: inboxApprovalPlan{Title: "Private plan"}}})
		if err != nil {
			t.Fatal(err)
		}
		id := db.GenerateUUID()
		private[id] = true
		if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: id, RoomID: room, AuthorType: "user", AuthorID: "operator_user", Content: attachmentEnvelope + string(raw)}); err != nil {
			t.Fatal(err)
		}
	}
	publicID := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: publicID, RoomID: room, AuthorType: "user", AuthorID: "visitor", Content: "Approved: Langkah berikutnya"}); err != nil {
		t.Fatal(err)
	}
	owner := s.getFormattedMessages(ctx, room, "receipts")
	found := 0
	for _, m := range owner {
		if private[m.UUID] {
			found++
			if !m.IsInternal {
				t.Fatal("owner decision classified public")
			}
		}
	}
	if found != len(private) {
		t.Fatal("owner lost decision history")
	}
	sess, err := s.crmSession(ctx, room)
	if err != nil {
		t.Fatal(err)
	}
	check := func(method, path string, payload any) {
		t.Helper()
		raw, _ := json.Marshal(payload)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(string(raw))))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var response struct {
			Messages []ChatMessageDTO `json:"messages"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		publicFound := false
		for _, m := range response.Messages {
			if private[m.UUID] {
				t.Fatal("visitor received internal decision")
			}
			publicFound = publicFound || m.UUID == publicID
		}
		if !publicFound {
			t.Fatal("ordinary visitor text incorrectly hidden")
		}
	}
	check("GET", "/api/chat/messages?session_uuid="+room, nil)
	check("GET", "/api/chat/messages?session_uuid="+room+"&after_id=0", nil)
	check("POST", "/api/chat/session", map[string]string{"session_uuid": room, "guest_uuid": sess.GuestUUID, "product": sess.Product})
	ids := []string{}
	for id := range private {
		ids = append(ids, id)
	}
	raw, _ := json.Marshal(map[string]any{"session_uuid": room, "guest_uuid": sess.GuestUUID, "read_ids": ids})
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, httptest.NewRequest("POST", "/api/chat/receipts", strings.NewReader(string(raw))))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	receipts, err := s.projectStudioRepo.(db.ChatReceiptRepository).MessageReceipts(ctx, room)
	if err != nil {
		t.Fatal(err)
	}
	for id := range private {
		if _, exists := receipts[id]; exists {
			t.Fatal("visitor acknowledged private decision")
		}
	}
}

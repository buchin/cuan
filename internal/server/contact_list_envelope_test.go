package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestContactListCompactEnvelopePreservesContentSearchAndCounts(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	room := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Compact list fixture", Topic: "agent-topic:agent_lifecycle", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("Useful update with Unicode: selesai ✓. ", 2000) + "UniqueTailNeedle"
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "agent", AuthorID: "agent_lifecycle", AuthorName: "@lifecycle", Content: content, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	legacy := call("GET", "/api/v1/chat/contacts", "")
	compact := call("GET", "/api/v1/chat/contacts?view=list", "")
	if legacy.Code != 200 || compact.Code != 200 {
		t.Fatalf("list failed: legacy%d compact%d", legacy.Code, compact.Code)
	}
	var oldBody, newBody map[string]json.RawMessage
	if err := json.Unmarshal(legacy.Body.Bytes(), &oldBody); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(compact.Body.Bytes(), &newBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := newBody["conversations"]; ok {
		t.Fatal("compact view retained duplicate array")
	}
	if !bytes.Equal(oldBody["contacts"], newBody["contacts"]) || !bytes.Equal(oldBody["total"], newBody["total"]) || !bytes.Equal(oldBody["contacts"], oldBody["conversations"]) {
		t.Fatal("envelope changed data/counts or legacy alias")
	}
	if compact.Body.Len()*10 > legacy.Body.Len()*6 {
		t.Fatalf("duplicate payload not removed: %d versus %d", compact.Body.Len(), legacy.Body.Len())
	}
	search := call("GET", "/api/v1/chat/contacts?view=list&q="+url.QueryEscape("UniqueTailNeedle"), "")
	if search.Code != 200 {
		t.Fatal(search.Code)
	}
	var response struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(search.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Contacts) != 1 || response.Contacts[0].LastMessage != content || response.Contacts[0].UnreadCount != 1 {
		t.Fatal("full preview/search/unread semantics lost")
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room)
	if err != nil || len(messages) != 1 || messages[0].Content != content {
		t.Fatal("stored message changed", err)
	}
}

func TestInboxCoordinationInputsDoNotBecomePreviewsOrUnreadMessages(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	room := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Visible conversation", Topic: "agent-topic:agent_lifecycle", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	real := db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "agent", AuthorID: "agent_lifecycle", Content: "Saya sedang memeriksa hasilnya.", CreatedAt: now, UpdatedAt: now}
	internal := db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "system", AuthorID: "cuan_work_review", Content: "INTERNAL_REVIEW_NEEDLE: review detailed task instructions", CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)}
	for _, m := range []*db.ProjectMessage{&real, &internal} {
		if err := s.projectStudioRepo.CreateMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	get := func() ChatContact {
		t.Helper()
		w := call("GET", "/api/v1/chat/contacts?view=list", "")
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		var response struct {
			Contacts []ChatContact `json:"contacts"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		for _, c := range response.Contacts {
			if c.ConversationID == room {
				return c
			}
		}
		t.Fatal("conversation missing")
		return ChatContact{}
	}
	first := get()
	if first.LastMessage != real.Content || first.UnreadCount != 1 || !first.LastMessageAt.Equal(real.CreatedAt) {
		t.Fatalf("internal job leaked into preview/count/time: %+v", first)
	}
	search := call("GET", "/api/v1/chat/contacts?view=list&q=INTERNAL_REVIEW_NEEDLE", "")
	var results struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(search.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results.Contacts) != 0 {
		t.Fatal("internal instructions leaked through search")
	}
	issue := db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "system", AuthorID: "cuan_assistant", Content: "Koneksi GitHub belum tersedia.", CreatedAt: now.Add(2 * time.Second), UpdatedAt: now.Add(2 * time.Second)}
	if err := s.projectStudioRepo.CreateMessage(ctx, &issue); err != nil {
		t.Fatal(err)
	}
	after := get()
	if after.LastMessage != issue.Content || after.UnreadCount != 2 {
		t.Fatal("actual issue notification was hidden", after)
	}
	stored, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room)
	if err != nil || len(stored) != 3 || stored[1].Content != internal.Content {
		t.Fatal("history was rewritten", err)
	}
	quoted := real
	quoted.AuthorType = "user"
	quoted.AuthorID = "cuan_work_review"
	if internalInboxReviewInput(quoted) {
		t.Fatal("user text mistaken for coordinator input")
	}
}

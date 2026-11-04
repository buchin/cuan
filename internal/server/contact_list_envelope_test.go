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

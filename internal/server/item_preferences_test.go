package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

func TestItemPreferencesRetainReadArchiveAndOwnerIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	call := func(method, path, body, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	url := "/api/v1/item-preferences?kind=conversation"
	if w := call("GET", url, "", ""); w.Code != 401 {
		t.Fatal("owner required", w.Code)
	}
	for _, body := range []string{`{}`, `{"id":"x","archived":null}`, `{"id":"x","archived":true,"other":true}`} {
		if w := call("PATCH", url, body, token); w.Code != 400 {
			t.Fatal("invalid request accepted", w.Code)
		}
	}
	// Real room/messages: viewing all contacts must use the saved read cutoff.
	id := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(context.Background(), &db.ProjectRoom{ID: id, ProjectID: "default", Title: "Customer fixture", Topic: "visitor", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	first := time.Now().Add(-time.Minute)
	second := first.Add(time.Second)
	for i, at := range []time.Time{first, second} {
		if err := s.projectStudioRepo.CreateMessage(context.Background(), &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: id, AuthorType: "user", Content: strings.Repeat("x", i+1), CreatedAt: at, UpdatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	read := func() itemPreferences {
		w := call("GET", url, "", token)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result struct {
			Items itemPreferences `json:"items"`
		}
		if json.Unmarshal(w.Body.Bytes(), &result) != nil {
			t.Fatal("invalid response")
		}
		return result.Items
	}
	patch := func(body string) {
		w := call("PATCH", url, body, token)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	patch(`{"id":"` + id + `","read":true,"seen_at":"` + first.UTC().Format(time.RFC3339Nano) + `"}`)
	if read()[id].ReadAt != first.UTC().Format(time.RFC3339Nano) {
		t.Fatal("read marker exceeded the messages actually seen", read())
	}
	w := call("GET", "/api/v1/chat/contacts", "", ""+token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var list struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if json.Unmarshal(w.Body.Bytes(), &list) != nil {
		t.Fatal("invalid contacts")
	}
	found := false
	for _, c := range list.Contacts {
		if c.ConversationID == id {
			found = true
			if c.UnreadCount != 1 {
				t.Fatal("new unseen message was incorrectly marked read", c.UnreadCount)
			}
		}
	}
	if !found {
		t.Fatal("fixture conversation missing")
	}
	patch(`{"id":"` + id + `","archived":true}`)
	before := read()[id]
	if before.ArchivedAt == "" || before.ReadAt == "" {
		t.Fatal("archive lost read marker", before)
	}
	patch(`{"id":"` + id + `","archived":false}`)
	if got := read()[id]; got.ArchivedAt != "" || got.ReadAt != before.ReadAt {
		t.Fatal("undo lost independent read state", got)
	}
	patch(`{"id":"` + id + `","read":false}`)
	if !read()[id].Unread {
		t.Fatal("manual unread not retained")
	}
	owner, e := s.authSvc.ValidateToken(context.Background(), token)
	if e != nil {
		t.Fatal(e)
	}
	path, e := itemPreferencesPath(owner.ID)
	if e != nil {
		t.Fatal(e)
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private persistent file required", e)
	}
	// Re-read from disk rather than depending on request-local maps.
	saved, e := readItemPreferences(owner.ID)
	if e != nil || !saved["conversation:"+id].Unread {
		t.Fatal("state not persisted", e)
	}
	original := s.authSvc
	otherSvc := auth.NewService(auth.NewMemoryStore())
	other, e := otherSvc.Setup(context.Background(), "other@fixture.invalid", "fixture-other-password")
	if e != nil {
		t.Fatal(e)
	}
	s.authSvc = otherSvc
	if otherItems, e := readItemPreferences(other.Owner.ID); e != nil || len(otherItems) != 0 {
		t.Fatal("owner state leaked", e)
	}
	if r := call("GET", url, "", other.Token); r.Code != 200 || !strings.Contains(r.Body.String(), `"items":{}`) {
		t.Fatal("owner isolation", r.Code)
	}
	s.authSvc = original
	if e := os.WriteFile(path, []byte("{broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if r := call("PATCH", url, `{"id":"x","archived":true}`, token); r.Code != 503 {
		t.Fatal("corrupt state was overwritten", r.Code)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "{broken" {
		t.Fatal("corrupt state lost")
	}
}

func TestItemPreferencesBatchMarkAllRead(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	url := "/api/v1/item-preferences?kind=conversation"

	// Create 2 fixture rooms with unread messages
	room1 := db.GenerateUUID()
	room2 := db.GenerateUUID()
	for _, id := range []string{room1, room2} {
		if err := s.projectStudioRepo.CreateRoom(context.Background(), &db.ProjectRoom{
			ID: id, ProjectID: "default", Title: "Room " + id[:4], Topic: "visitor", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.projectStudioRepo.CreateMessage(context.Background(), &db.ProjectMessage{
			ID: db.GenerateUUID(), RoomID: id, AuthorType: "user", Content: "Hello unread", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Verify both rooms have unread messages in contacts
	w := call("GET", "/api/v1/chat/contacts", "")
	if w.Code != 200 {
		t.Fatalf("expected 200 from contacts, got %d", w.Code)
	}
	var contactsResp struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &contactsResp); err != nil {
		t.Fatal(err)
	}
	var unreadBefore int
	for _, c := range contactsResp.Contacts {
		if (c.ConversationID == room1 || c.ConversationID == room2) && c.UnreadCount > 0 {
			unreadBefore++
		}
	}
	if unreadBefore < 2 {
		t.Fatalf("expected at least 2 unread fixture rooms before batch mark read, got %d", unreadBefore)
	}

	// Perform batch mark all read: {"all": true, "read": true}
	patchRes := call("PATCH", url, `{"all":true,"read":true}`)
	if patchRes.Code != 200 {
		t.Fatalf("expected 200 from batch PATCH, got %d: %s", patchRes.Code, patchRes.Body.String())
	}

	// Verify contacts now report 0 unread for both rooms
	wAfter := call("GET", "/api/v1/chat/contacts", "")
	if err := json.Unmarshal(wAfter.Body.Bytes(), &contactsResp); err != nil {
		t.Fatal(err)
	}
	for _, c := range contactsResp.Contacts {
		if c.ConversationID == room1 || c.ConversationID == room2 {
			if c.UnreadCount != 0 {
				t.Fatalf("expected 0 unread for room %s after mark all read, got %d", c.ConversationID, c.UnreadCount)
			}
		}
	}

	// Test batch by IDs
	patchIDsRes := call("PATCH", url, `{"ids":["`+room1+`","`+room2+`"],"read":true}`)
	if patchIDsRes.Code != 200 {
		t.Fatalf("expected 200 from batch IDs PATCH, got %d: %s", patchIDsRes.Code, patchIDsRes.Body.String())
	}
}

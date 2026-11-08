package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCRMContactIdentityAndTags(t *testing.T) {
	s, token := setupTestChatServer(t)
	call := func(method, path, body string, authed bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if authed {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	init := func(guest, product, email string) string {
		t.Helper()
		b, _ := json.Marshal(map[string]string{"guest_uuid": guest, "product": product, "email": email, "session_uuid": defaultScrumMasterRoomID})
		w := call("POST", "/api/chat/session", string(b), false)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var v struct {
			Session string `json:"session_uuid"`
		}
		json.Unmarshal(w.Body.Bytes(), &v)
		if v.Session == defaultScrumMasterRoomID {
			t.Fatal("visitor selected internal room")
		}
		return v.Session
	}
	a := init("a", "one", "A@example.com")
	if init("a", "one", "") != a {
		t.Fatal("reconnect duplicated contact")
	}
	if init("a", "two", "a@example.com") == a {
		t.Fatal("products share identity")
	}
	if init("b", "one", "a@example.com") == a {
		t.Fatal("claimed email merged visitors")
	}
	bad := call("POST", "/api/chat/session", `{"guest_uuid":"x","email":"bad"}`, false)
	if bad.Code != 400 {
		t.Fatal("bad email accepted")
	}
	if call("GET", "/api/v1/contacts", "", false).Code != 401 {
		t.Fatal("contacts public")
	}
	w := call("GET", "/api/v1/contacts", "", true)
	var out struct {
		Contacts []CRMContact `json:"contacts"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Contacts) != 3 {
		t.Fatalf("contacts=%s", w.Body.String())
	}
	var id string
	for _, c := range out.Contacts {
		if c.SessionUUID == a {
			id = c.ID
			if c.Email != "a@example.com" {
				t.Fatal("email lost")
			}
		}
	}
	w = call("PATCH", "/api/v1/contacts/"+id, `{"tags":[" VIP ","vip","Customer"]}`, true)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"tags":["VIP","Customer"]`)) {
		t.Fatal(w.Code, w.Body.String())
	}
	// Cache loss simulates a process restart; persistent identity is authoritative.
	globalChatManager.Lock()
	delete(globalChatManager.sessions, a)
	globalChatManager.Unlock()
	w = call("POST", "/api/chat/messages", `{"session_uuid":"`+a+`","text":"hello"}`, false)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	list := call("GET", "/api/v1/chat/contacts?type=customers", "", true)
	var inbox struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if list.Code != 200 || json.Unmarshal(list.Body.Bytes(), &inbox) != nil {
		t.Fatal("Inbox contact lookup failed", list.Code)
	}
	found := false
	for _, c := range inbox.Contacts {
		if c.ConversationID == a {
			found = true
			if c.ContactID != id || c.ID == c.ContactID {
				t.Fatal("Inbox alias replaced CRM identity", c)
			}
		}
	}
	if !found {
		t.Fatal("conversation missing from Inbox")
	}
	detail := call("GET", "/api/v1/chat/conversations/"+a+"/messages", "", true)
	var payload struct {
		Contact ChatContact `json:"contact"`
	}
	if detail.Code != 200 || json.Unmarshal(detail.Body.Bytes(), &payload) != nil || payload.Contact.ContactID != id || payload.Contact.Email != "a@example.com" {
		t.Fatal("detail identity not bound to CRM", detail.Code, detail.Body.String())
	}
	if call("GET", "/api/chat/messages?session_uuid="+defaultScrumMasterRoomID, "", false).Code != 404 {
		t.Fatal("visitor can read internal room")
	}
	if call("POST", "/api/chat/messages", `{"session_uuid":"`+defaultScrumMasterRoomID+`","text":"bad"}`, false).Code != 404 {
		t.Fatal("visitor can write internal room")
	}
}

func TestWidgetUsesActiveReleaseAssets(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "chat"), 0700); err != nil {
		t.Fatal(err)
	}
	want := []byte("// active release fixture")
	if err := os.WriteFile(filepath.Join(root, "chat", "widget.js"), want, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUAN_STATIC_DIR", root)
	s, _ := setupTestChatServer(t)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, httptest.NewRequest("GET", "/chat/widget.js", nil))
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), want) {
		t.Fatal("widget did not follow configured release", w.Code)
	}
}

package server

import (
	"bytes"
	"encoding/json"
	"github.com/buchin/cuan/internal/db"
	"net/http/httptest"
	"testing"
)

func TestCRMManualContactCreateRetryAndIsolation(t *testing.T) {
	s, token := setupTestChatServer(t)
	call := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	id := db.GenerateUUID()
	body := `{"request_id":"` + id + `","name":" Alice ","email":"ALICE@example.com"}`
	if w := call("POST", "/api/v1/contacts", body, false); w.Code != 401 {
		t.Fatal("unauthed create", w.Code)
	}
	for i := 0; i < 3; i++ {
		w := call("POST", "/api/v1/contacts", body, true)
		if w.Code != 200 && w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := call("GET", "/api/v1/contacts", "", true)
	var list struct{ Contacts []CRMContact }
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Contacts) != 1 || !list.Contacts[0].IsManual || list.Contacts[0].Name != "Alice" || list.Contacts[0].Email != "alice@example.com" {
		t.Fatal("wrong manual contact", w.Body.String())
	}
	w = call("GET", "/api/v1/chat/contacts", "", true)
	var chats struct{ Contacts []ChatContact }
	json.Unmarshal(w.Body.Bytes(), &chats)
	for _, c := range chats.Contacts {
		if c.ConversationID == list.Contacts[0].SessionUUID {
			t.Fatal("unconnected manual contact presented as message recipient")
		}
	}
	conflict := `{"request_id":"` + id + `","name":"Changed","email":"alice@example.com"}`
	if w := call("POST", "/api/v1/contacts", conflict, true); w.Code != 409 {
		t.Fatal("retry changed identity", w.Code)
	}
	second := `{"request_id":"` + db.GenerateUUID() + `","name":"Alice two","email":"alice@example.com"}`
	if w := call("POST", "/api/v1/contacts", second, true); w.Code != 201 {
		t.Fatal("same email was merged", w.Code)
	}
	for _, body := range []string{`{"request_id":"bad","name":"Name"}`, `{"request_id":"` + db.GenerateUUID() + `","name":""}`, `{"request_id":"` + db.GenerateUUID() + `","name":"Name","email":"bad"}`, `{"request_id":"` + db.GenerateUUID() + `","name":"Name","product":"spoof"}`} {
		if w := call("POST", "/api/v1/contacts", body, true); w.Code != 400 {
			t.Fatal("invalid input accepted", w.Code)
		}
	}
	if w := call("POST", "/api/chat/session", `{"guest_uuid":"manual_`+id+`","product":"Manual","name":"Spoof"}`, false); w.Code != 400 {
		t.Fatal("visitor claimed owner namespace", w.Code)
	}
}

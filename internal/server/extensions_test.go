package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	sdk "github.com/buchin/cuan/pkg/extensions"
)

func TestExtensionHostContactFilterValidationEventsAndAPI(t *testing.T) {
	s, token := setupTestChatServer(t)
	registry := sdk.New()
	events := make(chan sdk.Event, 4)
	err := registry.Register(sdk.Plugin{Manifest: sdk.Manifest{ID: "example.contacts", Name: "Contact example", Version: "1.0.0", APIVersion: 1, Capabilities: []string{"filter:contact.before_save", "validate:contact.before_save", "event:contact.created", "action:inspect", "search:provide"}}, Filters: map[string]sdk.Filter{"contact.before_save": func(ctx context.Context, scope sdk.Scope, data json.RawMessage) (json.RawMessage, error) {
		var value map[string]string
		json.Unmarshal(data, &value)
		value["name"] = strings.TrimSpace(value["name"])
		return json.Marshal(value)
	}}, Validators: map[string]sdk.Validator{"contact.before_save": func(ctx context.Context, scope sdk.Scope, data json.RawMessage) error {
		if bytes.Contains(data, []byte("Blocked")) {
			return sdk.ErrCallback
		}
		return nil
	}}, Listeners: map[string]sdk.Listener{"contact.created": func(ctx context.Context, event sdk.Event) error { events <- event; return sdk.ErrCallback }}, Actions: map[string]sdk.Action{"inspect": func(ctx context.Context, scope sdk.Scope, data json.RawMessage) (json.RawMessage, error) {
		if scope.OwnerID == "" {
			t.Error("missing verified owner")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}}, Search: func(ctx context.Context, request sdk.SearchRequest) ([]sdk.SearchItem, error) {
		return []sdk.SearchItem{{ID: "extra", Title: "Extension result", URL: "/contacts?contact=example"}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	registry.Enable("example.contacts")
	s.SetExtensionRegistry(registry)
	call := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	id := db.GenerateUUID()
	raw := `{"request_id":"` + id + `","name":"  Alice  ","email":"alice@example.invalid"}`
	for i := 0; i < 2; i++ {
		w := call("POST", "/api/v1/contacts", raw, true)
		if w.Code != 200 && w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	select {
	case event := <-events:
		if event.Scope.ResourceID != id || len(event.Data) != 0 {
			t.Fatal("event data contract", event)
		}
	case <-time.After(time.Second):
		t.Fatal("no created event")
	}
	if len(events) != 0 {
		t.Fatal("replayed created event")
	}
	w := call("POST", "/api/v1/contacts", `{"request_id":"`+db.GenerateUUID()+`","name":"Blocked"}`, true)
	if w.Code != 422 {
		t.Fatal("validator ignored", w.Code)
	}
	contacts, _ := s.crmList(context.Background())
	if len(contacts) != 1 {
		t.Fatal("failed extension wrote contact")
	}
	if w := call("GET", "/api/v1/extensions", "", false); w.Code != 401 {
		t.Fatal("public extension catalog")
	}
	if w := call("POST", "/api/v1/extensions/example.contacts/actions/inspect", `{"input":{}}`, false); w.Code != 401 {
		t.Fatal("public extension action")
	}
	if w := call("POST", "/api/v1/extensions/example.contacts/actions/inspect", `{"input":{}}`, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call("POST", "/api/v1/extensions/example.contacts/actions/inspect", `{"input":{},"project_id":"missing"}`, true); w.Code != 404 {
		t.Fatal("unregistered project scope", w.Code)
	}
	w = call("GET", "/api/v1/search?q=alice", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Extension result") {
		t.Fatal("search provider integration", w.Body.String())
	}
	registry.Disable("example.contacts")
	w = call("GET", "/api/v1/search?q=alice", "", true)
	if strings.Contains(w.Body.String(), "Extension result") {
		t.Fatal("disabled search ran")
	}
}

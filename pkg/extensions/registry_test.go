package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func manifest(id string, caps ...string) Manifest {
	return Manifest{ID: id, Name: "Fixture extension", Version: "1.0.0", APIVersion: 1, Capabilities: caps}
}
func TestManifestCapabilitiesLifecycleAndScope(t *testing.T) {
	r := New()
	p := Plugin{Manifest: manifest("example.contacts", "filter:contact.before_save"), Filters: map[string]Filter{"contact.before_save": func(ctx context.Context, scope Scope, data json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"name":"Filtered"}`), nil
	}}}
	if err := r.Register(Plugin{Manifest: manifest("bad")}); err == nil {
		t.Fatal("invalid ID accepted")
	}
	bad := p
	bad.Manifest = manifest("example.bad")
	if err := r.Register(bad); err == nil {
		t.Fatal("undeclared capability accepted")
	}
	p.Manifest.Projects = []string{"p1"}
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(p); err == nil {
		t.Fatal("duplicate ID")
	}
	p.Filters["contact.before_save"] = nil
	p.Manifest.Projects[0] = "changed"
	input := json.RawMessage(`{"name":"Original"}`)
	value, _ := r.Prepare(context.Background(), "contact.before_save", Scope{ProjectID: "p1"}, input)
	if string(value) != string(input) {
		t.Fatal("enabled by default")
	}
	r.Enable("example.contacts")
	value, err := r.Prepare(context.Background(), "contact.before_save", Scope{ProjectID: "p1"}, input)
	if err != nil || !strings.Contains(string(value), "Filtered") {
		t.Fatal("filter copy/scope", string(value), err)
	}
	value, _ = r.Prepare(context.Background(), "contact.before_save", Scope{ProjectID: "p2"}, input)
	if string(value) != string(input) {
		t.Fatal("cross-project filter")
	}
	statuses := r.List()
	statuses[0].Manifest.Projects[0] = "mutated"
	if r.List()[0].Manifest.Projects[0] != "p1" {
		t.Fatal("mutable manifest")
	}
	r.Disable("example.contacts")
	value, _ = r.Prepare(context.Background(), "contact.before_save", Scope{ProjectID: "p1"}, input)
	if string(value) != string(input) {
		t.Fatal("disabled filter ran")
	}
	r.Unregister("example.contacts")
	if len(r.List()) != 0 {
		t.Fatal("unregister")
	}
}
func TestFiltersValidatorsPanicAndTimeout(t *testing.T) {
	r := New()
	r.timeout = 15 * time.Millisecond
	p := Plugin{Manifest: manifest("example.validator", "validate:contact.before_save"), Validators: map[string]Validator{"contact.before_save": func(context.Context, Scope, json.RawMessage) error { panic("private-password") }}}
	r.Register(p)
	r.Enable(p.Manifest.ID)
	_, err := r.Prepare(context.Background(), "contact.before_save", Scope{}, json.RawMessage(`{}`))
	if err != ErrCallback || strings.Contains(err.Error(), "password") {
		t.Fatal("panic isolation", err)
	}
	r.Unregister(p.Manifest.ID)
	release := make(chan struct{})
	var completed atomic.Int32
	r.Register(Plugin{Manifest: manifest("example.hung", "validate:contact.before_save"), Validators: map[string]Validator{"contact.before_save": func(context.Context, Scope, json.RawMessage) error { <-release; completed.Add(1); return nil }}})
	r.Enable("example.hung")
	for i := 0; i < 8; i++ {
		if _, err = r.Prepare(context.Background(), "contact.before_save", Scope{}, json.RawMessage(`{}`)); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("missing deadline", err)
		}
	}
	if _, err = r.Prepare(context.Background(), "contact.before_save", Scope{}, json.RawMessage(`{}`)); err != ErrBusy {
		t.Fatal("unbounded callback capacity", err)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for completed.Load() != 8 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if completed.Load() != 8 {
		t.Fatal("fixture did not release")
	}
}
func TestEventsSearchActionsAndConnectors(t *testing.T) {
	r := New()
	var calls atomic.Int32
	p := Plugin{Manifest: manifest("example.integration", "event:task.completed", "search:provide", "action:echo", "connector:telegram"), Listeners: map[string]Listener{"task.completed": func(context.Context, Event) error { return errors.New("private failure") }}, Search: func(context.Context, SearchRequest) ([]SearchItem, error) {
		return []SearchItem{{ID: "good", Title: "Fixture", URL: "/projects/p1/tasks"}, {ID: "good", Title: "Duplicate", URL: "/projects/p1/tasks"}, {ID: "bad", Title: "Bad", URL: "https://external.invalid"}}, nil
	}, Actions: map[string]Action{"echo": func(ctx context.Context, scope Scope, data json.RawMessage) (json.RawMessage, error) {
		return data, nil
	}}, Connectors: map[string]Connector{"telegram": func(context.Context, Scope, Message) error { calls.Add(1); return nil }}}
	if err := r.Register(p); err != nil {
		t.Fatal(err)
	}
	r.Enable(p.Manifest.ID)
	failures := r.Publish(context.Background(), Event{ID: "stable", Type: "task.completed", Version: 1, At: time.Now()})
	if len(failures) != 1 || failures[0] != ErrCallback {
		t.Fatal("event failure reporting", failures)
	}
	found := r.Search(context.Background(), SearchRequest{Query: "fi", Limit: 6})
	if len(found) != 1 || len(found[0].Items) != 1 {
		t.Fatal("search URL/dedup", found)
	}
	echoed, err := r.Action(context.Background(), p.Manifest.ID, "echo", Scope{}, json.RawMessage(`{"ok":true}`))
	if err != nil || string(echoed) != `{"ok":true}` {
		t.Fatal(err)
	}
	if err = r.Send(context.Background(), p.Manifest.ID, "telegram", Scope{}, Message{Recipient: "r", Text: "Hi"}); err == nil {
		t.Fatal("missing idempotency key accepted")
	}
	if err = r.Send(context.Background(), p.Manifest.ID, "telegram", Scope{}, Message{IdempotencyKey: "approved-1", Recipient: "r", Text: "Hi"}); err != nil || calls.Load() != 1 {
		t.Fatal("connector dispatch", err, calls.Load())
	}
}

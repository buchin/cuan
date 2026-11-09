package diagnostics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOwnerCRMUsesConfiguredAPIAndDeclaresBounds(t *testing.T) {
	mode := "ready"
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer fixture-owner" {
			t.Error("missing configured read-only Owner request")
		}
		if mode == "unavailable" {
			w.WriteHeader(503)
			w.Write([]byte("provider secret must not escape"))
			return
		}
		if r.URL.Path == "/api/v1/contacts" {
			if mode == "invalid directory" {
				w.Write([]byte(`{}`))
				return
			}
			// Actual /contacts wire contract uses conversation_id, unlike the
			// MCP snapshot's normalized session_uuid. Do not generate this fixture
			// from the consumer struct: that hid a real identity mismatch.
			w.Write([]byte(`{"contacts":[{"id":"contact-one","conversation_id":"room-one","guest_uuid":"guest","name":"First","email":"first@example.invalid","product":"dojo"},{"id":"contact-two","conversation_id":"room-two","guest_uuid":"guest","name":"Second","email":"second@example.invalid","product":"other"}]}`))
			return
		}
		if r.URL.Path == "/api/v1/contacts/contact-one/memory" {
			if mode == "notes unavailable" {
				w.WriteHeader(503)
				return
			}
			if mode == "invalid notes" {
				w.Write([]byte(`{"items":[]}`))
				return
			}
			w.Write([]byte(`{"items":[{"id":"note-one","body":"Untrusted note"}],"has_more":true}`))
			return
		}
		t.Error("unexpected CRM request", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer api.Close()
	previous := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture-owner", HTTPClient: api.Client()}
	t.Cleanup(func() { globalCuanClient = previous })
	ctx := context.Background()
	result, err := ListContacts(ctx, "", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	list := result.(map[string]any)
	if list["total"] != 1 || list["has_more"] != true || list["source"] != "configured_owner_api" {
		t.Fatal("lost truthful bounds", list)
	}
	result, err = ListContacts(ctx, "FIRST@EXAMPLE", "dojo", 20)
	if err != nil || result.(map[string]any)["total"] != 1 {
		t.Fatal(result, err)
	}
	result, err = ListContacts(ctx, "", "other", 20)
	if err != nil || result.(map[string]any)["contacts"].([]ContactSummary)[0].ID != "contact-two" {
		t.Fatal(result, err)
	}
	for _, identity := range []string{"room-one", "contact-one"} {
		result, err = GetContact(ctx, identity)
		if err != nil {
			t.Fatal(err)
		}
		detail := result.(map[string]any)
		if detail["contact"].(*ContactSummary).ID != "contact-one" || detail["memories_loaded"] != true || detail["memory_has_more"] != true || len(detail["memories"].([]map[string]any)) != 1 {
			t.Fatal("lost actual contact or note pagination", detail)
		}
	}
	if _, err = GetContact(ctx, "guest"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal("ambiguous guest identity selected", err)
	}
	if _, err = GetContact(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "current connected CRM") {
		t.Fatal("missing contact result unqualified", err)
	}
	for _, failure := range []string{"unavailable", "invalid directory", "notes unavailable", "invalid notes"} {
		mode = failure
		if _, err = GetContact(ctx, "room-one"); err == nil || strings.Contains(err.Error(), "provider secret") {
			t.Fatal("read failure became success/empty notes or leaked response", failure, err)
		}
	}
	mode = "ready"
	before := requests
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "bound-task-fixture")
	if _, err = GetContact(ctx, "contact-one"); err == nil || requests != before {
		t.Fatal("bound task inherited ambient Owner CRM connection", err)
	}
	if _, err = ListContacts(ctx, "", "", 20); err == nil || requests != before {
		t.Fatal("bound task inherited Owner directory", err)
	}
}

package diagnostics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatResolutionUsesObservedCutoffAndValidatesReceipt(t *testing.T) {
	const room = "11111111-1111-4111-8111-111111111111"
	for _, scenario := range []string{"resolve", "reopen", "stale", "bad receipt", "bound"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
			if scenario == "bound" {
				t.Setenv("CUAN_TASK_CAPABILITY_JSON", "{}")
			}
			reads, writes := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("wrong identity")
				}
				if r.Method == "GET" {
					reads++
					fmt.Fprintf(w, `{"contacts":[{"conversation_id":"%s","resolution_observed_at":"2026-11-10T08:00:00Z"}]}`, room)
					return
				}
				writes++
				if r.Method != "PATCH" || r.URL.Path != "/api/v1/chat/conversations/"+room+"/resolution" {
					t.Error("wrong target")
				}
				var payload map[string]any
				if json.NewDecoder(r.Body).Decode(&payload) != nil {
					t.Error("invalid body")
				}
				resolved := scenario != "reopen"
				if payload["resolved"] != resolved || (resolved && payload["observed_at"] != "2026-11-10T08:00:00Z") {
					t.Error("observation lost", payload)
				}
				if scenario == "stale" {
					w.WriteHeader(409)
					return
				}
				id := room
				if scenario == "bad receipt" {
					id = "foreign"
				}
				fmt.Fprintf(w, `{"id":"%s","resolved":%t,"resolved_at":"2026-11-10T08:00:00Z"}`, id, resolved)
			}))
			defer server.Close()
			old := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
			t.Cleanup(func() { globalCuanClient = old })
			command := "resolve"
			if scenario == "reopen" {
				command = "reopen"
			}
			var out, errOut bytes.Buffer
			code := RunChatCLI([]string{command, room}, &out, &errOut)
			if (code == 0) != (scenario == "resolve" || scenario == "reopen") {
				t.Fatal(code, errOut.String())
			}
			if scenario == "bound" {
				if writes != 0 || reads != 0 {
					t.Fatal("worker used owner operation")
				}
			} else if writes != 1 {
				t.Fatal("write missing or repeated", writes)
			}
		})
	}
}

func TestResolveCompletedSkipsUnfinishedAndAlreadyResolvedConversations(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprint(w, `{"contacts":[{"conversation_id":"done","resolution_observed_at":"2026-11-10T08:00:00Z","task_count":1,"done_tasks":1},{"conversation_id":"active","resolution_observed_at":"2026-11-10T08:00:00Z","task_count":2,"done_tasks":1,"active_tasks":1},{"conversation_id":"old","resolved":true,"resolution_observed_at":"2026-11-10T08:00:00Z","task_count":1,"done_tasks":1},{"conversation_id":"empty","resolution_observed_at":"2026-11-10T08:00:00Z"}]}`)
			return
		}
		writes++
		if r.URL.Path != "/api/v1/chat/conversations/done/resolution" {
			t.Error("wrong bulk target", r.URL.Path)
		}
		fmt.Fprint(w, `{"id":"done","resolved":true}`)
	}))
	defer server.Close()
	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })
	var out, errOut bytes.Buffer
	if code := RunChatCLI([]string{"resolve-completed"}, &out, &errOut); code != 0 || writes != 1 {
		t.Fatal(code, writes, errOut.String())
	}
}

func TestResolveAllIncludesConversationWithoutTasks(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprint(w, `{"contacts":[{"conversation_id":"taskless","has_messages":true,"resolution_observed_at":"2026-11-10T08:00:00Z"},{"conversation_id":"online-visitor","has_messages":false,"resolution_observed_at":"2026-11-10T08:00:00Z"},{"conversation_id":"pending","has_messages":true,"task_count":1,"queued_tasks":1,"resolution_observed_at":"2026-11-10T08:00:00Z"}]}`)
			return
		}
		writes++
		if r.URL.Path != "/api/v1/chat/conversations/taskless/resolution" {
			t.Error("incorrect bulk target", r.URL.Path)
		}
		fmt.Fprint(w, `{"id":"taskless","resolved":true}`)
	}))
	defer server.Close()
	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })
	var out, errOut bytes.Buffer
	if code := RunChatCLI([]string{"resolve-all"}, &out, &errOut); code != 0 || writes != 1 {
		t.Fatal(code, writes, errOut.String())
	}
}

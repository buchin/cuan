package diagnostics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskExecutionInspectionUsesOwnerRouteAndFiltersResponse(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong task", "failed", "trailing"} {
		t.Run(scenario, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/v1/tasks/task-one/execution" || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("unbound inspection")
				}
				if scenario == "failed" {
					w.WriteHeader(503)
					w.Write([]byte("PRIVATE_ERROR"))
					return
				}
				id := "task-one"
				if scenario == "wrong task" {
					id = "another-task"
				}
				w.Write([]byte(`{"task_id":"` + id + `","intent_kind":"verification","private_field":"PRIVATE_RESPONSE"}`))
				if scenario == "trailing" {
					w.Write([]byte(`{}`))
				}
			}))
			defer api.Close()
			previous := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
			defer func() { globalCuanClient = previous }()
			var out, errOut bytes.Buffer
			code := RunTaskCLI([]string{"execution", "task-one"}, &out, &errOut)
			if (code == 0) != (scenario == "valid") || strings.Contains(out.String()+errOut.String(), "PRIVATE_") {
				t.Fatal("unsafe inspection response", code)
			}
		})
	}
}

func TestTaskExecutionSelectsExactAttemptAndRejectsOldServerFallback(t *testing.T) {
	for _, scenario := range []string{"selected", "ignored selection", "foreign", "latest"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/api/v1/tasks/task/execution" || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("unexpected diagnostic request")
				}
				want := "source"
				if scenario == "latest" {
					want = ""
				}
				if r.URL.Query().Get("attempt_id") != want {
					t.Error("attempt selector was lost")
				}
				if scenario == "foreign" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				attempt := "source"
				if scenario == "ignored selection" || scenario == "latest" {
					attempt = "reviewer"
				}
				json.NewEncoder(w).Encode(map[string]any{"task_id": "task", "attempt_id": attempt, "intent_kind": "execution"})
			}))
			defer api.Close()
			previous := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
			t.Cleanup(func() { globalCuanClient = previous })
			args := []string{"execution", "task"}
			if scenario != "latest" {
				args = append(args, "--attempt", "source")
			}
			var out, errOut bytes.Buffer
			code := RunTaskCLI(args, &out, &errOut)
			wantSuccess := scenario == "selected" || scenario == "latest"
			if (code == 0) != wantSuccess || calls != 1 || (!wantSuccess && out.Len() != 0) {
				t.Fatal("attempt inspection returned unrelated execution or retried", code, calls, out.String(), errOut.String())
			}
		})
	}
}

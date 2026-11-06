package diagnostics

import (
	"bytes"
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

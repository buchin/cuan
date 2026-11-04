package diagnostics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskDeleteUsesOwnerAPIAndDoesNotRetry(t *testing.T) {
	for _, status := range []int{200, 404, 409, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/tasks/task-fixture" || r.Header.Get("Authorization") != "Bearer fixture-token" {
					t.Error("unexpected deletion request")
				}
				w.WriteHeader(status)
				w.Write([]byte(`{"private":"must-not-echo"}`))
			}))
			defer server.Close()
			previous := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture-token", HTTPClient: server.Client()}
			t.Cleanup(func() { globalCuanClient = previous })
			var out, errs bytes.Buffer
			code := RunTaskCLI([]string{"delete", "task-fixture"}, &out, &errs)
			if calls != 1 || (status == 200) != (code == 0) {
				t.Fatal(calls, code, errs.String())
			}
			if strings.Contains(out.String()+errs.String(), "must-not-echo") {
				t.Fatal("response leaked")
			}
		})
	}
}

func TestTaskDeleteRejectsMissingID(t *testing.T) {
	var out, errs bytes.Buffer
	if RunTaskCLI([]string{"delete"}, &out, &errs) != 2 {
		t.Fatal("missing ID accepted")
	}
}

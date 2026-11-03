package diagnostics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskCancelRequestsAuthenticatedCancellationWithoutClaimingStopped(t *testing.T) {
	for _, status := range []int{200, 409, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/v1/tasks/task-fixture/cancel" || r.Header.Get("Authorization") != "Bearer fixture-token" {
					t.Error("unexpected cancellation request")
				}
				w.WriteHeader(status)
				w.Write([]byte(`{"private":"must-not-echo"}`))
			}))
			defer server.Close()
			previous := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture-token", HTTPClient: server.Client()}
			t.Cleanup(func() { globalCuanClient = previous })
			var out, errors bytes.Buffer
			code := RunTaskCLI([]string{"cancel", "task-fixture"}, &out, &errors)
			if calls != 1 {
				t.Fatal("cancel must not retry itself")
			}
			if (status == 200) != (code == 0) {
				t.Fatal(code, errors.String())
			}
			combined := out.String() + errors.String()
			if strings.Contains(combined, "must-not-echo") {
				t.Fatal("response body leaked")
			}
			if status == 200 && !strings.Contains(out.String(), "Check the task to confirm") {
				t.Fatal("confirmation step missing")
			}
		})
	}
}

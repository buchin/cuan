package diagnostics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskLogsExplicitAttemptAndBoundedFailure(t *testing.T) {
	for _, status := range []int{200, 404, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/api/v1/tasks/task-a/attempts/att_a/logs" || r.URL.Query().Get("after_id") != "12" || r.URL.Query().Get("limit") != "5" || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("unexpected log request")
				}
				w.WriteHeader(status)
				if status == 200 {
					w.Write([]byte(`{"logs":[{"id":13,"stream":"stdout","line":"verified log"}],"total":1}`))
				} else {
					w.Write([]byte(`{"secret":"do-not-echo"}`))
				}
			}))
			defer server.Close()
			previous := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
			defer func() { globalCuanClient = previous }()
			var out, errors bytes.Buffer
			code := RunTaskCLI([]string{"logs", "task-a", "att_a", "--after-id", "12", "--limit", "5", "--json"}, &out, &errors)
			if calls != 1 || (code == 0) != (status == 200) {
				t.Fatal(code, calls, errors.String())
			}
			if strings.Contains(out.String()+errors.String(), "do-not-echo") {
				t.Fatal("error body leaked")
			}
			if status == 200 && !strings.Contains(out.String(), "verified log") {
				t.Fatal("logs absent")
			}
		})
	}
}
func TestTaskLogsRejectsAmbiguousOrInvalidRange(t *testing.T) {
	for _, args := range [][]string{{"logs", "task"}, {"logs", "../task", "att"}, {"logs", "task", "att", "--limit", "1001"}, {"logs", "task", "att", "--after-id", "-1"}, {"logs", "task", "att", "--tail", "--after-id", "1"}} {
		var out, errors bytes.Buffer
		if RunTaskCLI(args, &out, &errors) != 2 {
			t.Fatal(args)
		}
	}
}

func TestTaskLogsTailUsesAuthenticatedBoundedAttemptRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" || r.URL.Query().Get("tail") != "true" || r.URL.Query().Get("limit") != "50" || r.URL.Query().Get("after_id") != "0" {
			t.Error("invalid tail request")
		}
		w.Write([]byte(`{"logs":[{"id":900,"stream":"system","line":"native stage complete"}],"total":1}`))
	}))
	defer server.Close()
	previous := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	defer func() { globalCuanClient = previous }()
	var out, errOut bytes.Buffer
	if RunTaskCLI([]string{"logs", "task", "attempt", "--tail", "--limit", "50", "--json"}, &out, &errOut) != 0 || !strings.Contains(out.String(), "native stage complete") {
		t.Fatal(out.String(), errOut.String())
	}
}

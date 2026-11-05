package diagnostics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskSteerUsesOwnerAPIAndRejectsBoundWorkers(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "POST" || r.URL.Path != "/api/v1/tasks/task-fixture/steer" || r.Header.Get("Authorization") != "Bearer owner-fixture" {
			t.Error("wrong steering request")
		}
		var payload map[string]string
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["prompt"] != "Preserve original scope" || payload["author"] != "" {
			t.Error("wrong prompt or forged author")
		}
		w.Write([]byte(`{"status":"delivered","fallback":false}`))
	}))
	defer server.Close()
	previous := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "owner-fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = previous })
	var out, errs bytes.Buffer
	if code := RunTaskCLI([]string{"steer", "task-fixture", "Preserve original scope"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "bound-fixture")
	if code := RunTaskCLI([]string{"steer", "task-fixture", "Preserve original scope"}, &out, &errs); code != 1 || requests != 1 {
		t.Fatal("bound worker steered owner task")
	}
}

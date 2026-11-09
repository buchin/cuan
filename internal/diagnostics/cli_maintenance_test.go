package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMaintenanceAndMCPSchedulerUseActualStateAndConfirmedRequests(t *testing.T) {
	mode := "ready"
	posts := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing configured Owner connection")
		}
		if r.URL.Path == "/api/v1/system/scheduler/jobs" && r.Method == "GET" {
			if mode == "failed-read" {
				w.WriteHeader(503)
				w.Write([]byte("not scheduler state"))
				return
			}
			if mode == "missing" {
				w.Write([]byte(`{"jobs":[]}`))
				return
			}
			enabled := mode != "disabled"
			json.NewEncoder(w).Encode(map[string]any{"jobs": []map[string]any{{"id": "storage_maintenance", "name": "Storage maintenance", "enabled": enabled, "last_status": "failed"}}})
			return
		}
		if r.URL.Path == "/api/v1/system/scheduler/jobs/storage_maintenance/trigger" && r.Method == "POST" {
			posts++
			w.WriteHeader(202)
			if mode == "wrong-receipt" {
				w.Write([]byte(`{"status":"triggered","job_id":"another-job"}`))
			} else {
				w.Write([]byte(`{"status":"triggered","job_id":"storage_maintenance"}`))
			}
			return
		}
		t.Error("unexpected maintenance route", r.Method, r.URL.Path)
		w.WriteHeader(404)
	}))
	defer api.Close()
	previous := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
	t.Cleanup(func() { globalCuanClient = previous })
	result, err := ListScheduledJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	jobs := result.(map[string]any)["jobs"].([]ScheduledJobState)
	if len(jobs) != 1 || jobs[0].LastStatus != "failed" {
		t.Fatal("MCP substituted fabricated healthy jobs", result)
	}
	var out, errors bytes.Buffer
	if code := RunMaintenanceCLI([]string{"status"}, &out, &errors); code != 0 || posts != 0 {
		t.Fatal("status mutated or failed", code, errors.String())
	}
	if code := RunMaintenanceCLI([]string{"run"}, &out, &errors); code != 0 || posts != 1 {
		t.Fatal("fixed job was not requested once", code, posts, errors.String())
	}
	for _, next := range []string{"disabled", "missing", "failed-read"} {
		mode = next
		if code := RunMaintenanceCLI([]string{"run"}, &out, &errors); code == 0 || posts != 1 {
			t.Fatal("unavailable state started maintenance", next, code, posts)
		}
	}
	if _, err := ListScheduledJobs(context.Background()); err == nil {
		t.Fatal("failed scheduler read became fabricated success")
	}
	mode = "wrong-receipt"
	if code := RunMaintenanceCLI([]string{"run"}, &out, &errors); code == 0 || posts != 2 {
		t.Fatal("unconfirmed request became success or repeated", code, posts)
	}
}

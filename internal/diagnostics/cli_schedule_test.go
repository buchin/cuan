package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScheduleCLILifecycle(t *testing.T) {
	tasks := []map[string]any{
		{
			"id":            "sch_fixture_1",
			"title":         "Daily Codebase Maintenance",
			"prompt":        "Audit waste in repo",
			"schedule_type": "daily",
			"time_of_day":   "03:00",
			"enabled":       true,
			"last_status":   "success",
			"project_id":    "default",
			"next_run_at":   "2026-11-23T03:00:00Z",
		},
	}

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing authorization header")
		}

		switch {
		case r.URL.Path == "/api/v1/system/scheduler/tasks" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"tasks": tasks})

		case r.URL.Path == "/api/v1/system/scheduler/tasks" && r.Method == http.MethodPost:
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(400)
				return
			}
			req["id"] = "sch_new_123"
			req["enabled"] = true
			req["last_status"] = "idle"
			tasks = append(tasks, req)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(req)

		case strings.HasPrefix(r.URL.Path, "/api/v1/system/scheduler/tasks/") && strings.HasSuffix(r.URL.Path, "/toggle") && r.Method == http.MethodPatch:
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/system/scheduler/tasks/"), "/toggle")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "enabled": false})

		case strings.HasPrefix(r.URL.Path, "/api/v1/system/scheduler/tasks/") && strings.HasSuffix(r.URL.Path, "/trigger") && r.Method == http.MethodPost:
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/system/scheduler/tasks/"), "/trigger")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "triggered", "id": id})

		case strings.HasPrefix(r.URL.Path, "/api/v1/system/scheduler/tasks/") && r.Method == http.MethodDelete:
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/system/scheduler/tasks/")
			filtered := []map[string]any{}
			for _, item := range tasks {
				if item["id"] != id {
					filtered = append(filtered, item)
				}
			}
			tasks = filtered
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "deleted", "id": id})

		default:
			t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer api.Close()

	prevClient := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: api.URL, AuthToken: "fixture", HTTPClient: api.Client()}
	t.Cleanup(func() { globalCuanClient = prevClient })

	ctx := context.Background()

	// 1. Test Help
	var out, errOut bytes.Buffer
	code := RunScheduleCLI([]string{"help"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "Cuan Schedule CLI") {
		t.Fatalf("help failed: code=%d, out=%s", code, out.String())
	}

	// 2. Test List
	out.Reset()
	errOut.Reset()
	code = RunScheduleCLI([]string{"list"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "Daily Codebase Maintenance") {
		t.Fatalf("list failed: code=%d, out=%s, err=%s", code, out.String(), errOut.String())
	}

	// 3. Test List JSON
	out.Reset()
	errOut.Reset()
	code = RunScheduleCLI([]string{"list", "--json"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "sch_fixture_1") {
		t.Fatalf("list json failed: code=%d, out=%s", code, out.String())
	}

	// 4. Test Create
	out.Reset()
	errOut.Reset()
	code = RunScheduleCLI([]string{"create", "--title", "Security Scan", "--prompt", "Scan CVEs", "--type", "daily", "--time", "04:00"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "Security Scan") {
		t.Fatalf("create failed: code=%d, out=%s, err=%s", code, out.String(), errOut.String())
	}

	// 5. Test Trigger
	out.Reset()
	errOut.Reset()
	code = RunScheduleCLI([]string{"trigger", "sch_new_123"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "triggered") {
		t.Fatalf("trigger failed: code=%d, out=%s, err=%s", code, out.String(), errOut.String())
	}

	// 6. Test Toggle
	out.Reset()
	errOut.Reset()
	code = RunScheduleCLI([]string{"toggle", "sch_new_123", "--disable"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "disabled") {
		t.Fatalf("toggle failed: code=%d, out=%s, err=%s", code, out.String(), errOut.String())
	}

	// 7. Test Delete
	out.Reset()
	errOut.Reset()
	code = RunScheduleCLI([]string{"delete", "sch_new_123"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "deleted") {
		t.Fatalf("delete failed: code=%d, out=%s, err=%s", code, out.String(), errOut.String())
	}

	// Verify MCP integration
	mcpList, err := globalCuanClient.ListScheduledTasks(ctx)
	if err != nil || len(mcpList) != 1 {
		t.Fatalf("client.ListScheduledTasks failed: err=%v, count=%d", err, len(mcpList))
	}
}

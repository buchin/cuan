package taskcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStopHookBoundedReportContinuation(t *testing.T) {
	for _, scenario := range []string{"missing", "reported", "new-progress", "cancelled", "error", "background", "deadline", "unavailable", "historical"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CUAN_TASK_LIFECYCLE_DIR", root)
			deadline := time.Now().Add(time.Minute)
			if scenario == "deadline" {
				deadline = time.Now().Add(-time.Minute)
			}
			t.Setenv("CUAN_TASK_REPORT_DEADLINE", deadline.Format(time.RFC3339Nano))
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("unscoped access")
				}
				if scenario == "unavailable" {
					w.WriteHeader(503)
					return
				}
				state := "progress"
				if scenario == "reported" || scenario == "new-progress" || scenario == "historical" {
					state = "review"
				}
				attempt := "current"
				if scenario == "historical" {
					attempt = "old"
				}
				activities := []map[string]string{{"attempt_id": attempt, "activity_type": "agent_report", "new_status": state}}
				if scenario == "new-progress" {
					activities = append(activities, map[string]string{"attempt_id": "current", "activity_type": "agent_report", "new_status": "progress"})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"summary": "Saved", "complete_note": "Evidence", "latest_attempt": map[string]any{"id": "current", "status": "running", "cancel_requested": scenario == "cancelled"}, "activities": activities})
			}))
			defer api.Close()
			cap, _ := json.Marshal(Capability{API: api.URL, Token: "fixture", TaskID: "task", AttemptID: "current"})
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(cap))
			event := `{"fullyIdle":true,"terminationReason":"NO_TOOL_CALL"}`
			if scenario == "error" {
				event = `{"fullyIdle":true,"terminationReason":"ERROR"}`
			}
			if scenario == "background" {
				event = `{"fullyIdle":false,"terminationReason":"NO_TOOL_CALL"}`
			}
			want := scenario == "missing" || scenario == "new-progress" || scenario == "historical"
			for n := 0; n < 2; n++ {
				var out bytes.Buffer
				runStopHook(strings.NewReader(event), &out)
				var result struct{ Decision string }
				if json.Unmarshal(out.Bytes(), &result) != nil {
					t.Fatal(out.String())
				}
				if (result.Decision == "continue") != (want && n == 0) {
					t.Fatalf("call %d: %s", n, out.String())
				}
			}
		})
	}
}

package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestNativePreparationFailureAcceptsOnlyTypedConflict(t *testing.T) {
	for _, scenario := range []string{"conflict", "unavailable", "unknown", "malformed", "trailing"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/v1/runner/attempts/attempt/release/prepare" {
					t.Error("unexpected preparation request")
				}
				status, body := http.StatusConflict, `{"code":"source_merge_conflict","error":"PRIVATE_PROVIDER_DETAILS"}`
				switch scenario {
				case "unavailable":
					status = http.StatusServiceUnavailable
				case "unknown":
					body = `{"code":"arbitrary_permission","error":"PRIVATE_PROVIDER_DETAILS"}`
				case "malformed":
					body = "PRIVATE_PROVIDER_DETAILS"
				case "trailing":
					body += " {}"
				}
				w.WriteHeader(status)
				w.Write([]byte(body))
			}))
			defer api.Close()
			r := NewRunner(&config.Config{RunnerID: "runner", RunnerToken: "fixture", RunnerServerURL: api.URL}, nil, api.Client())
			prepared, failure := r.requestNativeReleasePreparation(context.Background(), ClaimTaskResponse{Task: &task.Task{ID: "task"}, Attempt: &task.TaskAttempt{ID: "attempt"}, NativeRelease: &task.NativeReleaseIntent{}})
			if prepared != nil || calls != 1 || (failure == task.NativeReleaseMergeConflictMessage) != (scenario == "conflict") || strings.Contains(failure, "PRIVATE_") {
				t.Fatal("preparation failure retried, exposed raw error or invented a cause", prepared, failure, calls)
			}
		})
	}
}

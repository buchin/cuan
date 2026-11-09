package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestNativePreparationTransportRetainsOnlyKnownFailureCodes(t *testing.T) {
	for _, code := range []string{"native_preparation_source_push", "native_preparation_runtime", "source_merge_conflict", "authorization", "native_preparation_private-secret"} {
		t.Run(code, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{"code": code, "error": "private-provider-secret"})
			}))
			defer api.Close()
			r := NewRunner(&config.Config{RunnerID: "native-fixture", RunnerToken: "fixture", RunnerServerURL: api.URL}, nil, api.Client())
			prepared, failure := r.requestNativeReleasePreparation(context.Background(), ClaimTaskResponse{Task: &task.Task{ID: "task"}, Attempt: &task.TaskAttempt{ID: "attempt"}, NativeRelease: &task.NativeReleaseIntent{}})
			if prepared != nil || strings.Contains(failure, "private") {
				t.Fatal("untrusted error leaked or failed response supplied execution", failure)
			}
			switch code {
			case "source_merge_conflict":
				if failure != task.NativeReleaseMergeConflictMessage {
					t.Fatal("existing merge classification changed")
				}
			case "native_preparation_runtime", "native_preparation_source_push":
				if !strings.Contains(failure, strings.TrimPrefix(code, "native_preparation_")) {
					t.Fatal("specific stage lost in runner completion", failure)
				}
			default:
				if failure != "" {
					t.Fatal("unknown server code became a trusted diagnostic", failure)
				}
			}
		})
	}
}

func TestNativePreparationServiceUnavailableStage(t *testing.T) {
	for _, code := range []string{"publication_config", "runtime_paths", "runtime_permissions", "npm_config", "ssh_runtime", "context_changed", "remote_target", "canonical_preparation"} {
		t.Run(code, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]string{"code": "native_preparation_" + code, "error": "credential-must-not-leak"})
			}))
			defer api.Close()
			r := NewRunner(&config.Config{RunnerID: "fixture", RunnerToken: "fixture", RunnerServerURL: api.URL}, nil, api.Client())
			prepared, failure := r.requestNativeReleasePreparation(context.Background(), ClaimTaskResponse{Task: &task.Task{ID: "task"}, Attempt: &task.TaskAttempt{ID: "attempt"}, NativeRelease: &task.NativeReleaseIntent{}})
			if prepared != nil || failure != "Native release preparation failed at "+code+"; no release process started" {
				t.Fatalf("lost bounded native stage: %q", failure)
			}
		})
	}
}

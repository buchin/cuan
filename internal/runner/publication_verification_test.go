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

func TestNativePublicationResponseCannotSubstituteOtherAttemptOrRevision(t *testing.T) {
	for _, scenario := range []string{"verified", "wrong-attempt", "wrong-task", "wrong-commit", "unhealthy", "prose", "unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			commit := strings.Repeat("a", 40)
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/runner/attempts/attempt/publication/verify" || r.Method != "POST" || r.Header.Get("X-Runner-Token") != "runner-token" {
					t.Error("unscoped native request")
				}
				var input map[string]string
				if json.NewDecoder(r.Body).Decode(&input) != nil || input["task_id"] != "task" || input["runner_id"] != "runner" || input["commit_sha"] != commit || len(input) != 3 {
					t.Error("caller-supplied target or evidence")
				}
				if scenario == "unavailable" {
					w.WriteHeader(503)
					return
				}
				if scenario == "prose" {
					w.Write([]byte(`{"summary":"Deployed and verified"}`))
					return
				}
				receipt := task.DeploymentReceipt{Version: 1, TaskID: "task", AttemptID: "attempt", CommitSHA: commit, ReleaseSHA: commit, Environment: "production", Health: "healthy"}
				switch scenario {
				case "wrong-attempt":
					receipt.AttemptID = "old"
				case "wrong-task":
					receipt.TaskID = "other"
				case "wrong-commit":
					receipt.ReleaseSHA = strings.Repeat("b", 40)
				case "unhealthy":
					receipt.Health = "failed"
				}
				json.NewEncoder(w).Encode(map[string]any{"verified": true, "receipt": receipt})
			}))
			defer api.Close()
			r := NewRunner(&config.Config{RunnerID: "runner", RunnerToken: "runner-token", RunnerServerURL: api.URL}, nil, api.Client())
			claim := ClaimTaskResponse{Task: &task.Task{ID: "task"}, Attempt: &task.TaskAttempt{ID: "attempt"}}
			if r.verifyRegisteredPublication(context.Background(), claim, commit) != (scenario == "verified") {
				t.Fatal("unbound or unsupported publication response accepted")
			}
		})
	}
}

package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestDeploymentReceiptReadRecoversGrantWithoutAcceptingProse(t *testing.T) {
	for _, valid := range []bool{false, true} {
		t.Run(map[bool]string{false: "prose rejected", true: "receipt accepted"}[valid], func(t *testing.T) {
			now := time.Now().UTC()
			started := now.Add(-time.Minute)
			commit := strings.Repeat("a", 40)
			attempt := &task.TaskAttempt{ID: "attempt", TaskID: "task", StartedAt: &started}
			payload, _ := json.Marshal(task.DeploymentReceipt{Version: 1, TaskID: "task", AttemptID: "attempt", CommitSHA: commit, ReleaseSHA: commit, Environment: "production", Health: "healthy"})
			activities := []*task.TaskActivity{}
			if valid {
				activities = append(activities, &task.TaskActivity{ID: task.DeploymentReceiptID("task", "attempt", commit), TaskID: "task", AttemptID: &attempt.ID, Author: "cuan-deploy", ActivityType: "deployment_verified", Message: string(payload), CreatedAt: now})
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/lost" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/publication/verify") {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				calls++
				if r.URL.Path != "/api/v1/runner/attempts/attempt/report" || r.URL.Query().Get("task_id") != "task" || r.URL.Query().Get("runner_id") != "runner" || r.Header.Get("X-Runner-Token") != "secret" {
					t.Error("unbound deployment evidence read")
				}
				json.NewEncoder(w).Encode(map[string]any{"summary": "Deployed healthy live production", "complete_note": "release verified", "activities": activities})
			}))
			defer server.Close()
			runner := NewRunner(&config.Config{RunnerID: "runner", RunnerToken: "secret", RunnerServerURL: server.URL}, nil)
			claim := ClaimTaskResponse{AutoDeploy: true, Task: &task.Task{ID: "task"}, Attempt: attempt, TaskAgent: &TaskAgentCapability{API: server.URL + "/lost", Token: "old"}}
			got, note := runner.confirmAutoDeployCompliance(context.Background(), claim, true, commit)
			if got != valid || calls != 1 {
				t.Fatalf("accepted=%v calls=%d note=%s", got, calls, note)
			}
		})
	}
}

package runner

import (
	"context"
	"fmt"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfirmAgentReportRecoversReadAfterEphemeralGrantLoss(t *testing.T) {
	for _, reportStatus := range []string{"done", "progress"} {
		t.Run(reportStatus, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/lost-grant" {
					w.WriteHeader(401)
					return
				}
				calls++
				if r.URL.Path != "/api/v1/runner/attempts/att_a/report" || r.URL.Query().Get("task_id") != "task-a" || r.URL.Query().Get("runner_id") != "runner-a" || r.Header.Get("X-Runner-Token") != "runner-secret" {
					t.Error("unbound recovery request")
				}
				fmt.Fprintf(w, `{"summary":"Guide ready","complete_note":"Counted content","dod_checklist":[],"activities":[{"attempt_id":"att_a","activity_type":"agent_report","new_status":%q}]}`, reportStatus)
			}))
			defer server.Close()
			runner := NewRunner(&config.Config{RunnerID: "runner-a", RunnerToken: "runner-secret", RunnerServerURL: server.URL}, nil)
			claim := ClaimTaskResponse{Task: &task.Task{ID: "task-a"}, Attempt: &task.TaskAttempt{ID: "att_a"}, TaskAgent: &TaskAgentCapability{API: server.URL + "/lost-grant", Token: "old-grant"}}
			got := runner.confirmAgentReport(context.Background(), claim)
			if got != (reportStatus == "done") || calls != 1 {
				t.Fatal("unexpected recovery", got, calls)
			}
			if got && claim.Task.CompleteNote != "Counted content" {
				t.Fatal("evidence not preserved")
			}
		})
	}
}

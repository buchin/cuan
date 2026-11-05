package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestPTYFinalReportDoesNotHideUnexpectedFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("native PTY completion fixture")
	}
	for _, scenario := range []string{"unexpected-exit", "unexpected-kill", "reported-stop", "reported-blocker", "new-progress"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			script := "#!/bin/sh\nsleep 0.15\nexit 7\n"
			if scenario == "unexpected-kill" {
				script = "#!/bin/sh\nsleep 0.15\nkill -KILL $$\n"
			}
			if scenario != "unexpected-exit" && scenario != "unexpected-kill" {
				script = "#!/bin/sh\nwhile :; do sleep 1; done\n"
			}
			if err := os.WriteFile(filepath.Join(root, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", root+":"+os.Getenv("PATH"))
			var calls atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/report" {
					w.Write([]byte(`{"status":"ok"}`))
					return
				}
				state := "review"
				if scenario == "reported-blocker" {
					state = "blocked"
				}
				if scenario == "new-progress" && calls.Add(1) > 1 {
					state = "progress"
				}
				fmt.Fprintf(w, `{"summary":"Fixture result","complete_note":"Fixture evidence","activities":[{"attempt_id":"attempt-%s","activity_type":"agent_report","new_status":%q}]}`, scenario, state)
			}))
			defer api.Close()
			r := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL}, nil, api.Client())
			current := &task.Task{ID: "pty-report-" + scenario}
			attempt := &task.TaskAttempt{ID: "attempt-" + scenario, TaskID: current.ID, AttemptNumber: 1}
			claim := ClaimTaskResponse{Task: current, Attempt: attempt, TaskAgent: &TaskAgentCapability{API: api.URL + "/report", Token: "fixture-only"}}
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			var logs int64
			result, err := r.executeWithPTY(ctx, claim, current, attempt, "Fixture only", root, map[string]string{"HOME": root}, 3*time.Second, &logs)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "unexpected-exit", "unexpected-kill":
				if result.Status == "SUCCESS" || result.ExitCode == 0 {
					t.Fatal("final report hid process failure", result)
				}
			case "reported-stop":
				if result.Status != "SUCCESS" || result.ExitCode != 0 {
					t.Fatal("intentional reported stop was treated as failure", result)
				}
			case "reported-blocker":
				if result.Status != "BLOCKED" {
					t.Fatal("reported blocker was accepted or left until deadline", result)
				}
			case "new-progress":
				if result.Status == "SUCCESS" {
					t.Fatal("an old final report stopped later work")
				}
			}
		})
	}
}

package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestPTYDiagnosticPipelinePreservesSplitFailureAndStopsSplitQuota(t *testing.T) {
	for _, scenario := range []string{"failure", "quota"} {
		t.Run(scenario, func(t *testing.T) {
			directory := t.TempDir()
			binary := filepath.Join(directory, "agy")
			script := "#!/bin/sh\nprintf 'fatal: diag'\nsleep 0.05\nprintf 'nostic failure\\nfinal cause without newline'\nexit 2\n"
			if scenario == "quota" {
				script = "#!/bin/sh\nprintf 'Quota ex'\nsleep 0.05\nprintf 'ceeded'\nsleep 10\n"
			}
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", directory+":"+os.Getenv("PATH"))
			var mu sync.Mutex
			var logs []task.LogRecord
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/logs") {
					var request StreamLogsRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
					}
					mu.Lock()
					logs = append(logs, request.Logs...)
					mu.Unlock()
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer server.Close()
			runner := NewRunner(&config.Config{RunnerID: "diagnostic-fixture", RunnerServerURL: server.URL}, nil, server.Client())
			id := "pty-diagnostic-" + scenario
			current := &task.Task{ID: id, Title: "Diagnostic fixture", ExecutionCategory: task.ExecutionCategoryCode}
			attempt := &task.TaskAttempt{ID: "att-" + id, TaskID: id, AttemptNumber: 1}
			claim := ClaimTaskResponse{Claimed: true, Task: current, Attempt: attempt, WorkingDir: directory}
			var count int64
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := runner.executeWithPTY(ctx, claim, current, attempt, "fixture", directory, nil, 3*time.Second, &count)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "quota" {
				if result.Status != "QUOTA_LIMITED" || result.ExitCode != 75 {
					t.Fatalf("split quota waited for timeout: %+v", result)
				}
				return
			}
			if result.Status != "FAILED" || result.ExitCode != 2 {
				t.Fatalf("failure result changed: %+v", result)
			}
			mu.Lock()
			defer mu.Unlock()
			var output []string
			for _, record := range logs {
				if record.Stream == "stdout" {
					output = append(output, record.Line)
				}
			}
			if len(output) != 2 || output[0] != "fatal: diagnostic failure" || output[1] != "final cause without newline" {
				t.Fatalf("split or final diagnostic lost: %q", output)
			}
		})
	}
}

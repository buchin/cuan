package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

type canceledDiagnosticAdapter struct {
	cancel      context.CancelFunc
	t           *testing.T
	evidenceDir string
}

func (a *canceledDiagnosticAdapter) Name() string             { return "diagnostic-fixture" }
func (a *canceledDiagnosticAdapter) Version() (string, error) { return "1", nil }
func (a *canceledDiagnosticAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	if req.Env["GIT_PAGER"] != "cat" {
		a.t.Error("bound native worker can wait in an interactive Git pager")
	}
	if a.evidenceDir != "" && req.Env["CUAN_TASK_ARTIFACTS_DIR"] != a.evidenceDir {
		a.t.Error("runner lost the attempt-bound evidence directory")
	}
	if a.evidenceDir != "" && req.Env["CUAN_TASK_REPORT_FILE"] != a.evidenceDir+"/../report.json" {
		a.t.Error("native runner lost prepared report file")
	}
	if !strings.Contains(req.Prompt, "=== EXECUTION TIME BUDGET ===") || !strings.Contains(req.Prompt, `"$CUAN_TASK_EXECUTABLE" task report --state progress --progress`) {
		a.t.Error("dispatched task without a checklist lost checkpoint/deadline instructions")
	}
	a.cancel()
	return nil, context.Canceled
}

func TestRunnerFinalDiagnosticSurvivesExecutionCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var mu sync.Mutex
	var logs []task.LogRecord
	var completion CompleteAttemptRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/logs"):
			var payload StreamLogsRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			logs = append(logs, payload.Logs...)
		case strings.HasSuffix(r.URL.Path, "/complete"):
			if err := json.NewDecoder(r.Body).Decode(&completion); err != nil {
				t.Error(err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer api.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	evidenceDir := t.TempDir()
	runner := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL}, &canceledDiagnosticAdapter{cancel: cancel, t: t, evidenceDir: evidenceDir}, api.Client())
	now := time.Now().UTC()
	claim := ClaimTaskResponse{Claimed: true, OwnerID: "owner", WorkingDir: t.TempDir(), Task: &task.Task{ID: "cancel-diagnostic", Title: "Inspect", ExecutionCategory: task.ExecutionCategoryReadOnly}, Attempt: &task.TaskAttempt{ID: "attempt", TaskID: "cancel-diagnostic", AttemptNumber: 1, CreatedAt: now}}
	claim.TaskAgent = &TaskAgentCapability{API: api.URL + "/report", Token: "fixture", TaskID: claim.Task.ID, AttemptID: claim.Attempt.ID, ArtifactsDir: evidenceDir, ReportFile: evidenceDir + "/../report.json"}
	runner.runAttempt(ctx, claim)
	mu.Lock()
	defer mu.Unlock()
	if completion.Status != task.AttemptStatusInterrupted {
		t.Fatalf("cancellation lost its outcome: %+v", completion)
	}
	count := 0
	for _, line := range logs {
		if line.Stream == "system" && strings.Contains(line.Line, "selesai: interrupted") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected final diagnostic despite cancellation, got %d", count)
	}
}

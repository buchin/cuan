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

func TestUnpreparedNativeReleaseNeverEntersModelExecution(t *testing.T) {
	var completion CompleteAttemptRequest
	requests := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasSuffix(r.URL.Path, "/complete") {
			t.Errorf("native intent entered generic execution: %s", r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&completion); err != nil {
			t.Error(err)
		}
		json.NewEncoder(w).Encode(task.TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "native", Status: task.AttemptStatusFailed})
	}))
	defer api.Close()
	// A nil adapter proves dispatch happens before even reading a provider name.
	worker := NewRunner(&config.Config{RunnerID: "native", RunnerServerURL: api.URL, RunnerWorkerRegistryDir: t.TempDir(), WorkspacesDir: t.TempDir()}, nil, api.Client())
	worker.runAttempt(context.Background(), ClaimTaskResponse{Task: &task.Task{ID: "task", ExecutionCategory: task.ExecutionCategoryCode}, Attempt: &task.TaskAttempt{ID: "attempt", TaskID: "task"}, NativeRelease: &task.NativeReleaseIntent{}, WorkingDir: "/unprepared"})
	if requests != 1 || completion.Status != task.AttemptStatusFailed || completion.ExitCode == 0 || completion.AgentReportRequired || completion.LiveDeploymentConfirmed || completion.CommitSHA != "" || !strings.Contains(completion.ErrorMessage, "no model or release execution") {
		t.Fatal("native preparation failure was not terminal and factual", completion)
	}
}

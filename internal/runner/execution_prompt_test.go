package runner

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExecutionPromptPreservesAcceptanceWithoutCredentialSnapshot(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{Goal: "Original product goal", Instructions: "Run targeted checks and report gaps", Environment: "PASSWORD=private-snapshot-value"}})
	original := project.SnapshotProjectGuidance("Approved original task. Do not deploy.", &project.ProjectResponse{ID: "project", Metadata: raw})
	got := project.TaskPromptInstructions(original)
	for _, required := range []string{"Approved original task.", "Do not deploy.", "Original product goal", "Run targeted checks and report gaps"} {
		if !strings.Contains(got, required) {
			t.Fatal("lost acceptance guidance", required)
		}
	}
	for _, private := range []string{"private-snapshot-value", "PASSWORD=", "cuan-project-context", "environment"} {
		if strings.Contains(got, private) {
			t.Fatal("supplemental prompt exposed credential snapshot")
		}
	}
	if !strings.Contains(original, "private-snapshot-value") {
		t.Fatal("fixture did not contain credential")
	}
}

func TestOrdinaryExecutionKeepsProjectValuesInEnvironmentOnly(t *testing.T) {
	const privateValue = "private-fixture-environment-value"
	raw, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{Goal: "Keep original goal", Instructions: "Preserve approved scope", Environment: "PASSWORD=" + privateValue}})
	original := project.SnapshotProjectGuidance("Inspect the approved output. Do not deploy.", &project.ProjectResponse{ID: "project", Metadata: raw})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }))
	defer api.Close()
	adapter := &mockStreamingSteeringAdapter{}
	r := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL}, adapter, api.Client())
	now := time.Now()
	claim := ClaimTaskResponse{Claimed: true, WorkingDir: t.TempDir(), Environment: "PASSWORD=" + privateValue, Task: &task.Task{ID: "prompt-environment", Title: "Approved output inspection", Instructions: original, ExecutionCategory: task.ExecutionCategoryOps}, Attempt: &task.TaskAttempt{ID: "prompt-attempt", TaskID: "prompt-environment", CreatedAt: now}}
	r.runAttempt(context.Background(), claim)
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	req := adapter.executedReq
	if req.Env["PASSWORD"] != privateValue {
		t.Fatal("saved process environment was lost")
	}
	for _, forbidden := range []string{privateValue, "PASSWORD=", "cuan-project-context"} {
		if strings.Contains(req.Prompt, forbidden) {
			t.Fatal("ordinary execution prompt exposed credential snapshot")
		}
	}
	for _, required := range []string{"Inspect the approved output.", "Do not deploy.", "Keep original goal", "Preserve approved scope"} {
		if !strings.Contains(req.Prompt, required) {
			t.Fatal("prompt lost original acceptance guidance", required)
		}
	}
}

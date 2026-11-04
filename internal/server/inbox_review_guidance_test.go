package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxReviewPreservesLongScopeAndSavedGuidanceWithoutEnvironment(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	scope := strings.Repeat("Approved scope context. ", 430) + "FINAL_ACCEPTANCE_REQUIREMENT"
	metadata := json.RawMessage(`{"cuan_settings":{"goal":"SAVED_GOAL","instructions":"Inspect real mobile and desktop screenshots","environment":"PRIVATE_TOKEN=secret-fixture"}}`)
	instructions := project.SnapshotProjectGuidance(scope, &project.ProjectResponse{ID: "project", Metadata: metadata})
	d := &task.TaskDetailResponse{Task: task.Task{ID: "task", Instructions: instructions}, LatestAttempt: &task.TaskAttempt{ID: "attempt"}}
	raw := s.inboxReviewData(context.Background(), d)
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["original_request"] != scope || payload["saved_project_goal"] != "SAVED_GOAL" || payload["saved_project_instructions"] != "Inspect real mobile and desktop screenshots" {
		t.Fatal("review lost saved acceptance context")
	}
	if strings.Contains(raw, "PRIVATE_TOKEN") || strings.Contains(raw, "secret-fixture") || strings.Contains(raw, "cuan-project-context") {
		t.Fatal("review included execution snapshot")
	}
	if d.Instructions != instructions {
		t.Fatal("review mutated saved instructions")
	}
}

func TestInboxReviewSurfacesInvalidSavedGuidanceWithoutRawSnapshot(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	d := &task.TaskDetailResponse{Task: task.Task{Instructions: "Approved scope\n\nProject guidance snapshot:\n```cuan-project-context\nPRIVATE_BROKEN_SNAPSHOT\n```"}, LatestAttempt: &task.TaskAttempt{ID: "attempt"}}
	raw := s.inboxReviewData(context.Background(), d)
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["project_guidance_error"] == "" || payload["original_request"] != "Approved scope" || strings.Contains(raw, "PRIVATE_BROKEN_SNAPSHOT") {
		t.Fatal("invalid guidance was silently accepted or exposed")
	}
}

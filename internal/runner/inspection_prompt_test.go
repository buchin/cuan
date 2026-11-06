package runner

import (
	"encoding/json"
	"github.com/buchin/cuan/internal/project"
	"strings"
	"testing"
)

func TestInspectionPromptPreservesAcceptanceWithoutCredentialSnapshot(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{Goal: "Original product goal", Instructions: "Run targeted checks and report gaps", Environment: "PASSWORD=private-snapshot-value"}})
	original := project.SnapshotProjectGuidance("Approved original task. Do not deploy.", &project.ProjectResponse{ID: "project", Metadata: raw})
	got := inspectionPromptInstructions(original)
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

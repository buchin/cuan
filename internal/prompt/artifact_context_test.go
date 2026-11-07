package prompt

import (
	"encoding/json"
	"github.com/buchin/cuan/internal/project"
	"strings"
	"testing"
)

func TestArtifactPromptRetainsOriginalContextWithoutUnneededExecutionSecrets(t *testing.T) {
	scope := "Create a standalone file containing EXACT TEXT. Do not modify the repository. Do not deploy."
	frozen := project.SnapshotProjectGuidance(scope, &project.ProjectResponse{ID: "original-project", Metadata: json.RawMessage(`{"cuan_settings":{"goal":"Original saved goal","instructions":"Preserve literal requested content","environment":"UNNEEDED_SECRET=private-value"}}`)})
	prompt := BuildRunnerPromptWithDoD("Deliver result.txt", frozen, "", []string{"Keep the original filename"})
	for _, required := range []string{scope, "Original saved goal", "Preserve literal requested content", "Keep the original filename", "uncertain/rejected report"} {
		if !strings.Contains(prompt, required) {
			t.Fatal("required context lost", required)
		}
	}
	for _, unrelated := range []string{"UNNEEDED_SECRET", "private-value", "AGENTIC SCRUM", "MANDATORY REPOSITORY GUIDANCE", "task get before final reporting"} {
		if strings.Contains(prompt, unrelated) {
			t.Fatal("unneeded startup context", unrelated)
		}
	}
}

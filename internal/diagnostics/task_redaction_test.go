package diagnostics

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskDiagnosticOmitsFrozenEnvironment(t *testing.T) {
	for _, marker := range []string{"Project guidance snapshot:", "Project guidance snapshot (context only; does not expand task authorization):"} {
		raw := "Inspect dependencies\n\n" + marker + "\n```cuan-project-context\n{\"environment\":{\"TOKEN\":\"private-secret\"}}\n```"
		response := map[string]any{"task": map[string]any{"instructions": raw, "status": "done"}}
		redactTaskDiagnostic(response)
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "private-secret") {
			t.Fatal("execution environment leaked")
		}
		task := response["task"].(map[string]any)
		if task["instructions"] != "Inspect dependencies" || task["status"] != "done" {
			t.Fatal("scope or metadata changed")
		}
		if !strings.Contains(raw, "private-secret") {
			t.Fatal("source mutated")
		}
	}
}

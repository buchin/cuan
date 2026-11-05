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

func TestTaskDiagnosticMasksCredentialsInScopeAndChecklist(t *testing.T) {
	scope := "Inspect access logs\n- Kredensial root/SSH: fixture-sensitive-value\nDefinition of Done (DoD):\n- Report findings"
	value := map[string]any{"task": map[string]any{"instructions": scope, "dod_checklist": []any{map[string]any{"text": "Kredensial root/SSH: fixture-sensitive-value"}}, "summary": "password: fixture-sensitive-value"}}
	redactTaskDiagnostic(value)
	raw, _ := json.Marshal(value)
	if strings.Contains(string(raw), "fixture-sensitive-value") || !strings.Contains(string(raw), "Report findings") {
		t.Fatal("credential leaked or scope lost")
	}
	summary := TaskSummary{Instructions: scope, DoDChecklist: []string{"Credentials: fixture-sensitive-value"}}
	redactTaskSummary(&summary)
	raw, _ = json.Marshal(summary)
	if strings.Contains(string(raw), "fixture-sensitive-value") {
		t.Fatal("typed database response leaked credential")
	}
	if !strings.Contains(scope, "fixture-sensitive-value") {
		t.Fatal("saved execution scope changed")
	}
}

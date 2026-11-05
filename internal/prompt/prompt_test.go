package prompt

import (
	"strings"
	"testing"
	"time"
)

func TestGetAgentsGuidance(t *testing.T) {
	guidance := GetAgentsGuidance()
	if guidance == "" {
		t.Fatalf("expected guidance not to be empty")
	}
	if !strings.Contains(guidance, "Cuan repository guidance") {
		t.Errorf("expected guidance to contain 'Cuan repository guidance', got: %s", guidance)
	}
	if !strings.Contains(guidance, "Zero Cognitive Fatigue") {
		t.Errorf("expected guidance to contain 'Zero Cognitive Fatigue'")
	}
	if !strings.Contains(guidance, "Mandatory Design & UX Definition of Done") {
		t.Errorf("expected guidance to contain 'Mandatory Design & UX Definition of Done'")
	}
}

func TestGetScrumRules(t *testing.T) {
	rules := GetScrumRules()
	if rules == "" {
		t.Fatalf("expected scrum rules not to be empty")
	}
	if !strings.Contains(rules, "Agentic Scrum Rules (§1–§13)") {
		t.Errorf("expected rules to contain header 'Agentic Scrum Rules (§1–§13)'")
	}
	for i := 1; i <= 13; i++ {
		sec := "## " + string(rune('0'+i/10))
		if i < 10 {
			sec = "## " + string(rune('0'+i)) + "."
		} else {
			sec = "## " + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "."
		}
		if !strings.Contains(rules, sec) {
			t.Errorf("expected rules to contain section '%s'", sec)
		}
	}
}

func TestBuildRunnerPrompt(t *testing.T) {
	title := "Implement feature X"
	instructions := "Make sure X works cleanly with tests."
	agentPrompt := "Agent specific context."

	result := BuildRunnerPrompt(title, instructions, agentPrompt)
	if !strings.Contains(result, instructions) {
		t.Errorf("expected result to contain instructions")
	}
	if !strings.Contains(result, agentPrompt) {
		t.Errorf("expected result to contain agentPrompt")
	}
	if !strings.Contains(result, "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===") {
		t.Errorf("expected result to contain AGENTS.md banner")
	}
	if !strings.Contains(result, "=== AGENTIC SCRUM RULES FOR DEVELOPMENT TEAM (§1–§13) ===") {
		t.Errorf("expected result to contain Scrum Rules banner")
	}

	// Test idempotency: calling again should not duplicate banner
	duplicated := BuildRunnerPrompt(title, result, "")
	count := strings.Count(duplicated, "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===")
	if count != 1 {
		t.Errorf("expected exactly 1 AGENTS.md banner, got %d", count)
	}
}

func TestBuildRunnerPromptWithDoD(t *testing.T) {
	title := "Integrate micro-packages"
	instructions := "Wire pkg/taskfsm into runner"
	agentPrompt := "Focus on backward compatibility."
	dod := []string{
		"Verifikasi unit test 100% pass",
		"Build Go binary tanpa error",
		"Verifikasi health check /healthz",
	}

	result := BuildRunnerPromptWithDoD(title, instructions, agentPrompt, dod)

	if !strings.Contains(result, "=== MANDATORY DEFINITION OF DONE & ACCEPTANCE CRITERIA ===") {
		t.Errorf("expected result to contain DoD header")
	}
	for _, item := range dod {
		if !strings.Contains(result, "- [ ] "+item) {
			t.Errorf("expected result to contain DoD item %q", item)
		}
	}
	if !strings.Contains(result, `"$CUAN_TASK_EXECUTABLE" task report --state review --summary`) {
		t.Errorf("expected result to contain task reporting instructions")
	}
}

func TestBuildChatSystemPrompt(t *testing.T) {
	baseInst := "You coordinate work."
	schema := `{"type": "object"}`
	policy := "Scrum Master rules."
	name := "Scrum Master"
	contextData := "User message here."

	result := BuildChatSystemPrompt(baseInst, schema, policy, name, contextData)
	if !strings.Contains(result, baseInst) {
		t.Errorf("expected result to contain baseInst")
	}
	if !strings.Contains(result, schema) {
		t.Errorf("expected result to contain schema")
	}
	if !strings.Contains(result, policy) {
		t.Errorf("expected result to contain policy")
	}
	if !strings.Contains(result, "Your display name: Scrum Master") {
		t.Errorf("expected result to contain display name")
	}
	if !strings.Contains(result, "Context data:\nUser message here.") {
		t.Errorf("expected result to contain context data")
	}
	if !strings.Contains(result, "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===") {
		t.Errorf("expected result to contain AGENTS.md banner")
	}
	if !strings.Contains(result, "=== AGENTIC SCRUM RULES (§1–§13) ===") {
		t.Errorf("expected result to contain Scrum Rules banner")
	}
}

func TestTaskWithoutChecklistStillRequiresCheckpointAndFinalEvidence(t *testing.T) {
	prompt := BuildRunnerPromptWithDoD("Investigate a UI crash", "Original approved scope", "", nil)
	for _, required := range []string{`"$CUAN_TASK_EXECUTABLE" task report --state progress --progress`, "An empty checklist does not waive verification or reporting", "Continue the approved task afterward", "Original approved scope"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("missing execution contract %q", required)
		}
	}
	repeated := BuildRunnerPrompt("Title", prompt, "")
	if strings.Count(repeated, "=== TASK COMPLETION & VERIFICATION REPORTING REQUIREMENT ===") != 1 {
		t.Fatal("recovery duplicated reporting contract")
	}
}

func TestExecutionBudgetUsesExactDeadlineAndNeverGrantsCompletion(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	for _, remaining := range []time.Duration{20 * time.Minute, -time.Minute} {
		deadline := now.Add(remaining)
		text := ExecutionBudget(deadline, now)
		if !strings.Contains(text, deadline.Format(time.RFC3339)) || !strings.Contains(text, "do not extend it or treat it as task acceptance") || !strings.Contains(text, "actual checks") {
			t.Fatal("budget lost deadline or evidence boundary", text)
		}
		if remaining < 0 && !strings.Contains(text, "Remaining at dispatch: 0s") {
			t.Fatal("expired attempt gained execution time")
		}
	}
}

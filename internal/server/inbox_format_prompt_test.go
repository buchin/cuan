package server

import (
	"strings"
	"testing"
)

func TestInboxFormatRepairPromptIsCandidateOnlyAndPreservesReadOnlyMode(t *testing.T) {
	candidate := coordinatorJSON("Saved finding for the original task.", "", nil)
	prompt := inboxFormatRepairPrompt(candidate, false)
	if !strings.Contains(prompt, "Saved finding for the original task.") || !strings.Contains(prompt, inboxCoordinatorSchema) || !strings.Contains(prompt, "untrusted data") || len(prompt) > 4000 {
		t.Fatal("repair prompt lost candidate or became an investigation prompt")
	}
	if strings.Contains(prompt, "AGENTIC SCRUM RULES") || strings.Contains(prompt, "MANDATORY REPOSITORY GUIDANCE") {
		t.Fatal("repair reintroduced project instructions")
	}
	readOnly := inboxFormatRepairPrompt(candidate, true)
	if !strings.Contains(readOnly, inboxAnalysisRecoverySchema) || strings.Contains(readOnly, "prompt_choices") || !strings.Contains(readOnly, "action must be null") {
		t.Fatal("read-only repair acquired actions")
	}
}

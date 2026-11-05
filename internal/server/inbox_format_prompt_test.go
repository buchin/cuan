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

func TestInboxReconciliationFormatRepairCannotBroadenActions(t *testing.T) {
	candidate := coordinatorJSON("Verify the saved state first.", "delegation", inboxDelegation{Reconcile: true, Title: "Original work", Description: "Finish only the remaining approved work.", ProjectID: "registered", AuthorizationID: "owner-source"})
	prompt := inboxFormatRepairPrompt(candidate, true, true)
	if !strings.Contains(prompt, inboxRetryReconcileSchema) || !strings.Contains(prompt, "Never add an action absent from the candidate") || strings.Contains(prompt, "AGENTIC SCRUM RULES") {
		t.Fatal("reconciliation formatter lost scope")
	}
	if _, err := decodeInboxCoordinatorResponseForMode(candidate, false); err == nil {
		t.Fatal("ordinary transport accepted retry-only delegation")
	}
	if _, err := decodeInboxCoordinatorResponseForMode(candidate, true, true); err != nil {
		t.Fatal("explicit reconciliation delegation rejected", err)
	}
}

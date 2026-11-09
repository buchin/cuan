package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReviewInstructionKeepsEveryAcceptanceRuleWhenObservationsAbsent(t *testing.T) {
	short := inboxReviewInstruction(`{"original_request":"Create the approved file","publication_context":null,"live_result_context":null}`)
	if len(short) >= len(inboxReviewEvaluationInstruction) {
		t.Fatal("absent native observation definitions were not omitted")
	}
	// These sections own outcome, provenance, corrections, authority and delivery;
	// they apply regardless of the optional native observation types.
	for _, prefix := range []string{"You evaluate supplied work reports", "Own the decision", "When proposing revision", "source_review is", "execution_scope and", "For code implementation tasks", "Mere generic claims", "In explanation,"} {
		for _, paragraph := range strings.Split(inboxReviewEvaluationInstruction, "\n\n") {
			if strings.HasPrefix(paragraph, prefix) && !strings.Contains(short, paragraph) {
				t.Fatalf("changed required policy paragraph %s", prefix)
			}
		}
	}
	if strings.Contains(short, "live_result_context, when present,") {
		t.Fatal("absent observation definition retained")
	}
	t.Logf("review policy bytes: full=%d selected=%d removed=%d", len(inboxReviewEvaluationInstruction), len(short), len(inboxReviewEvaluationInstruction)-len(short))
}

func TestReviewInstructionRetainsPresentNativeObservationsVerbatim(t *testing.T) {
	keys := []string{"native_release_failure", "native_visual_verification", "verification_context", "native_preparation_history", "account_outcome_context", "publication_context", "running_project_release", "github_connection_observation", "live_result_context", "failed_repository_observation"}
	data := map[string]any{}
	for _, key := range keys {
		data[key] = map[string]any{"state": "unavailable"}
	}
	raw, _ := json.Marshal(data)
	if got := inboxReviewInstruction(string(raw)); got != inboxReviewEvaluationInstruction {
		t.Fatal("present native observations lost or changed their interpretation rules")
	}
	for _, raw := range []string{"not JSON", "null", "[]", `{"original_request":`, "Taste guidance\n{}"} {
		if got := inboxReviewInstruction(raw); got != inboxReviewEvaluationInstruction {
			t.Fatal("unparseable context did not retain full policy", raw)
		}
	}
}

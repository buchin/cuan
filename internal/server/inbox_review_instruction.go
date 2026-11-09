package server

import (
	"encoding/json"
	"strings"
)

// Keep the original policies verbatim. Only definitions of absent optional
// native observations can be omitted; outcome, scope and acceptance rules remain.
// Unknown input falls back to the full contract, never a permissive review.
func inboxReviewInstruction(snapshot string) string {
	var data map[string]json.RawMessage
	if json.Unmarshal([]byte(snapshot), &data) != nil || data == nil {
		return inboxReviewEvaluationInstruction
	}
	optional := map[string]string{
		"native_release_failure contains":           "native_release_failure",
		"native_visual_verification, when present,": "native_visual_verification",
		"verification_context, when present,":       "verification_context",
		"native_preparation_history contains":       "native_preparation_history",
		"account_outcome_context.original_scope=":   "account_outcome_context",
		"A publication_context with required=false": "publication_context",
		"Source and release identity:":              "publication_context",
		"running_project_release identifies":        "running_project_release",
		"github_connection_observation is":          "github_connection_observation",
		"live_result_context, when present,":        "live_result_context",
		"failed_repository_observation is":          "failed_repository_observation",
	}
	paragraphs := strings.Split(inboxReviewEvaluationInstruction, "\n\n")
	kept := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		omit := false
		for prefix, key := range optional {
			if strings.HasPrefix(paragraph, prefix) {
				raw := strings.TrimSpace(string(data[key]))
				omit = raw == "" || raw == "null" || raw == "[]" || raw == "{}"
				break
			}
		}
		if !omit {
			kept = append(kept, paragraph)
		}
	}
	return strings.Join(kept, "\n\n")
}

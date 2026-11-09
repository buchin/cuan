package server

import (
	"encoding/json"
)

// Classify transport failures without persisting provider content, file paths,
// action values or customer data. This is diagnostic only, never a decoder or
// permission to accept a rejected action.
func inboxResponseDiagnostic(raw string) string {
	if uniqueCoordinatorJSON([]byte(raw)) != nil {
		return "Response format: invalid or ambiguous JSON envelope."
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil || fields == nil {
		return "Response format: invalid JSON envelope."
	}
	for _, key := range []string{"reply", "topic_title", "action"} {
		if _, ok := fields[key]; !ok {
			return "Response format: required envelope field missing."
		}
	}
	if len(fields) != 3 {
		return "Response format: unexpected envelope fields."
	}
	var reply, title string
	if json.Unmarshal(fields["reply"], &reply) != nil || json.Unmarshal(fields["topic_title"], &title) != nil {
		return "Response format: invalid reply or title field type."
	}
	if isExplicitChoicePrompt(reply) {
		return "Response format: choice options missing or invalid."
	}
	var action struct {
		Kind    string          `json:"kind"`
		Payload json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(fields["action"], &action) == nil && action.Kind == "delegation" {
		var payload inboxDelegation
		if strictCoordinatorJSON(action.Payload, &payload) != nil {
			return "Response format: delegation payload fields or types rejected."
		}
		if _, delegation, err := parseInboxDelegation("```cuan-delegate\n" + string(action.Payload) + "\n```"); err != nil || delegation == nil {
			return "Response format: delegation payload constraints rejected."
		}
	}
	return "Response format: action payload or response contract rejected."
}

// Only fixed labels are retained, never provider values or private content.
func inboxReconciliationResponseDiagnostic(raw string) string {
	var response inboxCoordinatorResponse
	if strictCoordinatorJSON([]byte(raw), &response) != nil {
		return inboxResponseDiagnostic(raw)
	}
	if response.TopicTitle != "" {
		return "Retry contract: topic must remain unchanged."
	}
	if response.Action == nil {
		return inboxResponseDiagnostic(raw)
	}
	if response.Action.Kind != "delegation" {
		return "Retry contract: unsupported action kind."
	}
	var payload inboxDelegation
	if strictCoordinatorJSON(response.Action.Payload, &payload) != nil {
		return "Retry contract: unsupported payload fields or types."
	}
	if !payload.Reconcile {
		return "Retry contract: explicit reconciliation marker missing."
	}
	if payload.ProjectID == "" || payload.AuthorizationID == "" {
		return "Retry contract: original project or authorization binding missing."
	}
	if payload.TaskID == "" && (payload.Title == "" || payload.Description == "") {
		return "Retry contract: self-contained task title or description missing."
	}
	if payload.ProjectName != "" || payload.Action == "save" {
		return "Retry contract: unsupported project creation or save action."
	}
	return "Retry contract: task payload or response constraints rejected."
}

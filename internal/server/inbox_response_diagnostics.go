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
	return "Response format: action payload or response contract rejected."
}

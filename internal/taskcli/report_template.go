package taskcli

import (
	"encoding/json"
	"errors"
	"strings"
)

// Build only the writable report fields from the currently bound task. History,
// instructions, account metadata and previous evidence never become defaults.
func reportTemplate(raw []byte, capability Capability) ([]byte, error) {
	var current struct {
		ID      string `json:"id"`
		Attempt *struct {
			ID string `json:"id"`
		} `json:"latest_attempt"`
		Criteria []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"dod_checklist"`
	}
	bad := errors.New("Current task context does not match this bound attempt or its criteria; no report template was created")
	if json.Unmarshal(raw, &current) != nil || current.ID != capability.TaskID || capability.AttemptID == "" || current.Attempt == nil || current.Attempt.ID != capability.AttemptID {
		return nil, bad
	}
	type criterion struct {
		ID        string `json:"id,omitempty"`
		Text      string `json:"text"`
		Completed bool   `json:"completed"`
		Evidence  string `json:"evidence"`
	}
	items := make([]criterion, 0, len(current.Criteria))
	for _, item := range current.Criteria {
		if strings.TrimSpace(item.Text) == "" {
			return nil, bad
		}
		items = append(items, criterion{ID: item.ID, Text: item.Text})
	}
	return json.MarshalIndent(struct {
		State        string      `json:"state"`
		Summary      string      `json:"summary"`
		CompleteNote string      `json:"complete_note"`
		Criteria     []criterion `json:"dod_checklist"`
	}{State: "review", Criteria: items}, "", "  ")
}

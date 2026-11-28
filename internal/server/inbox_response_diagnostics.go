package server

import (
	"encoding/json"
	"strings"
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
	if string(fields["action"]) == "null" && strings.TrimSpace(reply) == "" {
		return "Response format: empty reply without action."
	}
	if len(reply) > 18000 || len([]rune(title)) > 72 {
		return "Response format: reply or title exceeds transport limit."
	}
	if strings.Contains(reply, "```cuan-") || strings.Contains(strings.ToLower(reply), "file://") {
		return "Response format: embedded action marker or local file URL in reply."
	}
	if json.Unmarshal(fields["action"], &action) == nil && string(fields["action"]) != "null" {
		switch action.Kind {
		case "approval", "delegation", "memory", "memory_search", "task_draft", "task_cancel", "task_reopen", "task_link", "prompt_choices", "secret_input", "customer_reply", "schedule", "project", "skill", "team", "brain", "brand_memory":
		default:
			return "Response format: unsupported action kind."
		}
		if len(action.Payload) == 0 || string(action.Payload) == "null" {
			return "Response format: action payload missing."
		}
		if action.Kind == "schedule" {
			var payload inboxScheduleAction
			if strictCoordinatorJSON(action.Payload, &payload) != nil {
				return "Response format: schedule payload fields or types rejected."
			}
			if _, _, err := parseInboxSchedule("```cuan-schedule\n" + string(action.Payload) + "\n```"); err != nil {
				return "Response format: " + err.Error()
			}
		}
		if action.Kind == "project" {
			var payload inboxProjectAction
			if strictCoordinatorJSON(action.Payload, &payload) != nil {
				return "Response format: project payload fields or types rejected."
			}
			if _, _, err := parseInboxProject("```cuan-project\n" + string(action.Payload) + "\n```"); err != nil {
				return "Response format: " + err.Error()
			}
		}
		if action.Kind == "skill" {
			var payload inboxSkillAction
			if strictCoordinatorJSON(action.Payload, &payload) != nil {
				return "Response format: skill payload fields or types rejected."
			}
			if _, _, err := parseInboxSkill("```cuan-skill\n" + string(action.Payload) + "\n```"); err != nil {
				return "Response format: " + err.Error()
			}
		}
		if action.Kind == "team" {
			var payload inboxTeamAction
			if strictCoordinatorJSON(action.Payload, &payload) != nil {
				return "Response format: team payload fields or types rejected."
			}
			if _, _, err := parseInboxTeam("```cuan-team\n" + string(action.Payload) + "\n```"); err != nil {
				return "Response format: " + err.Error()
			}
		}
		if action.Kind == "brain" {
			var payload inboxBrainAction
			if strictCoordinatorJSON(action.Payload, &payload) != nil {
				return "Response format: brain payload fields or types rejected."
			}
			if _, _, err := parseInboxBrain("```cuan-brain\n" + string(action.Payload) + "\n```"); err != nil {
				return "Response format: " + err.Error()
			}
		}
		if action.Kind == "brand_memory" {
			var payload inboxBrandMemoryAction
			if strictCoordinatorJSON(action.Payload, &payload) != nil {
				return "Response format: brand memory payload fields or types rejected."
			}
			if _, _, err := parseInboxBrandMemory("```cuan-brand-memory\n" + string(action.Payload) + "\n```"); err != nil {
				return "Response format: " + err.Error()
			}
		}
		if action.Kind == "task_reopen" {
			var payload inboxTaskReopenAction
			if strictCoordinatorJSON(action.Payload, &payload) != nil {
				return "Response format: task reopen payload fields or types rejected."
			}
			if strings.TrimSpace(payload.TaskID) == "" {
				return "Response format: task reopen identity missing."
			}
		}
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

// Review labels identify contract conflicts without recording findings or values.
func inboxReviewResponseDiagnostic(raw string) string {
	cleaned, ok := stripReviewJSONFences(raw)
	if !ok {
		return "Review format: invalid JSON object or field types."
	}
	var result inboxReviewEvaluation
	decoder := json.NewDecoder(strings.NewReader(cleaned))
	if decoder.Decode(&result) != nil {
		return "Review format: invalid JSON object or field types."
	}
	if result.Decision != "pass" && result.Decision != "revise" && result.Decision != "blocked" {
		return "Review format: decision must be pass, revise, or blocked."
	}
	if strings.TrimSpace(result.Explanation) == "" && !(result.Decision == "pass" && strings.TrimSpace(result.Finding) != "") {
		return "Review format: explanation is required."
	}
	if len(result.Finding) > 6000 {
		return "Review format: finding exceeds 6000 characters."
	}
	if len(result.Explanation) > 6000 {
		return "Review format: explanation exceeds 6000 characters."
	}
	if len(result.VisualInspection) > 4000 {
		return "Review format: visual_inspection exceeds 4000 characters."
	}
	if len(result.ManagerReason) > 1500 {
		return "Review format: manager_reason exceeds 1500 characters."
	}
	if result.Decision == "pass" && (result.ManagerAction != "" || result.ManagerReason != "") {
		return "Review format: pass decision must omit manager_action and manager_reason."
	}
	if result.SourceReview == "revise" && result.Decision == "pass" {
		return "Review format: source_review revise conflicts with pass decision."
	}
	if result.SourceReview == "pass" && result.Decision != "pass" && result.RemainingWork != "publication" {
		return "Review format: source pass with unfinished outcome requires publication classification."
	}
	if result.ManagerAction != "" && strings.TrimSpace(result.ManagerReason) == "" {
		return "Review format: manager_reason is required when manager_action is set."
	}
	if (result.ManagerAction == "continue" || result.ManagerAction == "replan") && (result.Decision != "revise" || result.RemainingWork == "external" || result.RequestedOutcome == "needs_input") {
		return "Review format: continuation conflicts with decision or owner prerequisite."
	}
	if (result.ManagerAction == "wait" || result.ManagerAction == "needs_owner") && result.Decision != "blocked" {
		return "Review format: wait/needs_owner requires blocked decision."
	}
	if result.ManagerAction == "wait" && result.RemainingWork != "external" && !(result.SourceReview == "pass" && result.RemainingWork == "publication") {
		return "Review format: wait manager_action requires external remaining_work (or publication when source_review is pass)."
	}
	if result.ManagerAction == "needs_owner" && result.RemainingWork != "external" && result.RequestedOutcome != "needs_input" && !(result.SourceReview == "pass" && result.RemainingWork == "publication") {
		return "Review format: needs_owner requires external remaining_work, needs_input outcome, or publication when source_review is pass."
	}
	if result.Decision == "revise" && strings.TrimSpace(result.Revision) == "" {
		return "Review format: revision missing for revise decision."
	}
	if result.Decision != "revise" && result.Revision != "" {
		return "Review format: revision conflicts with decision."
	}
	return "Review format: fields or decision constraints rejected."
}

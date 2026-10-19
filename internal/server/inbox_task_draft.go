package server

import (
	"encoding/json"
	"strings"
)

// Stored in the agent's reply so drafts survive SSE reconnects and reloads.
// A draft has no side effects: only the authenticated capture endpoint creates work.
type InboxTaskDraft struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ProjectID   string `json:"project_id,omitempty"`
}

const inboxTaskDraftInstruction = `Only if the user explicitly requests a draft without execution, use conversation history to prepare ONE self-contained draft: an imperative title and a description with the agreed goal, scope, constraints, and acceptance criteria. Do not create files or tasks or start implementation. Briefly explain that this is a draft that has not been saved or started. End the reply with one fenced block labeled cuan-task containing valid JSON with title (up to 200 characters), description (up to 6000 characters), and project_id (a registered project ID from the snapshot, or an empty string if ambiguous). Example:
` + "```cuan-task\n{\"title\":\"Improve chat navigation\",\"description\":\"Goal and scope from the discussion.\\nAcceptance criteria: ...\",\"project_id\":\"\"}\n```" + `
Only prepare a draft when requested, not for a greeting or ordinary analysis. If the goal or scope is unclear, ask first. Do not claim the task is saved; the UI shows an editable card and the user must select Create task. If the user revises a draft, provide a complete revised draft. Do not include credentials or private reasoning.`

func splitInboxTaskDraft(author, content string) (string, *InboxTaskDraft) {
	if author != "agent" {
		return content, nil
	}
	const marker = "```cuan-task\n"
	trimmed := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.LastIndex(trimmed, marker)
	if start < 0 || !strings.HasSuffix(trimmed, "```") || strings.Count(trimmed, marker) != 1 {
		return content, nil
	}
	raw := strings.TrimSpace(trimmed[start+len(marker) : len(trimmed)-3])
	if len(raw) > 32000 {
		return content, nil
	}
	var draft InboxTaskDraft
	if json.Unmarshal([]byte(raw), &draft) != nil {
		return content, nil
	}
	draft.Title = strings.TrimSpace(draft.Title)
	draft.Description = strings.TrimSpace(draft.Description)
	draft.ProjectID = strings.TrimSpace(draft.ProjectID)
	if draft.Title == "" || len([]rune(draft.Title)) > 200 || draft.Description == "" || len([]rune(draft.Description)) > 6000 || len(draft.ProjectID) > 200 {
		return content, nil
	}
	return strings.TrimSpace(trimmed[:start]), &draft
}

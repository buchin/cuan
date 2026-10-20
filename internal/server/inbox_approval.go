package server

import (
	"bytes"
	"encoding/json"
	"strings"
)

const inboxApprovalInstruction = "When a concrete plan is ready and you need the user's approval before starting work, present its outcome, scope, limits and acceptance criteria in plain language, then end the reply with exactly one cuan-approval fenced JSON block, for example:\n```cuan-approval\n{\"title\":\"Approve the proposed navigation update\"}\n```\nThe title is at most 200 characters. This marker only makes an approval notification; it does not authorize or run anything. Do not use it for greetings, ordinary questions, clarification, already approved work or a completed result. Do not combine it with cuan-delegate or cuan-task blocks."

func splitInboxApproval(author, content string) (string, string) {
	if author != "agent" {
		return content, ""
	}
	const marker = "```cuan-approval\n"
	text := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	index := strings.LastIndex(text, marker)
	if index < 0 || !strings.HasSuffix(text, "```") || strings.Count(text, marker) != 1 || strings.Contains(text, "```cuan-delegate") || strings.Contains(text, "```cuan-task") {
		return content, ""
	}
	raw := []byte(strings.TrimSpace(text[index+len(marker) : len(text)-3]))
	if len(raw) > 2048 {
		return content, ""
	}
	var value struct {
		Title string `json:"title"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || json.Valid(raw) == false {
		return content, ""
	}
	value.Title = strings.TrimSpace(value.Title)
	if value.Title == "" || len([]rune(value.Title)) > 200 {
		return content, ""
	}
	return strings.TrimSpace(text[:index]), value.Title
}

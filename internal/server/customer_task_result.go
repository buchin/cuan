package server

import (
	"strings"

	"github.com/buchin/cuan/internal/task"
)

// Acceptance does not imply that a service outage was fixed. Use the evaluated
// result and an actual saved draft, never a domain-specific success template.
func acceptedCustomerTaskResult(d *task.TaskDetailResponse, explanation string) string {
	body := strings.TrimSpace(explanation)
	if body == "" {
		body = strings.TrimSpace(d.Summary)
	}
	body, embedded := splitCustomerReplyDraft(body)
	if body == "" {
		body = "Hasil pekerjaan sudah ditinjau."
	}
	draft := extractCustomerReplyDraft(d.CompleteNote)
	if draft == "" {
		draft = extractCustomerReplyDraft(d.Summary)
	}
	if draft == "" {
		draft = embedded
	}
	if draft != "" {
		body += "\n\n**Customer reply draft:**\n> " + strings.ReplaceAll(draft, "\n", "\n> ")
	}
	return body + "\n\n[View work](/tasks/" + d.ID + ")"
}

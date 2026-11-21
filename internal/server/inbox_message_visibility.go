package server

import (
	"strings"

	"github.com/buchin/cuan/internal/db"
)

// Coordinator inputs are jobs, not messages addressed to the person reading
// Inbox. Actual system status updates and agent replies remain visible.
func internalInboxReviewInput(m db.ProjectMessage) bool {
	if m.AuthorType == "system" && m.AuthorID == "cuan_work_review" {
		return true
	}
	content := strings.TrimSpace(m.Content)
	if strings.HasPrefix(content, "I’m checking the latest work update") ||
		strings.HasPrefix(content, "I'm checking the latest work update") ||
		strings.HasPrefix(content, "Review the result for [") ||
		strings.HasPrefix(content, "This update is no longer current") {
		return true
	}
	return false
}

func visibleInboxMessages(messages []db.ProjectMessage) []db.ProjectMessage {
	result := make([]db.ProjectMessage, 0, len(messages))
	for _, m := range messages {
		if !internalInboxReviewInput(m) {
			result = append(result, m)
		}
	}
	return result
}

// clientPreviewMessage extracts the most relevant message for contact list preview.
// Bare task links or internal progress telemetry are skipped if a conversational
// user/agent message is available.
func clientPreviewMessage(messages []db.ProjectMessage) *db.ProjectMessage {
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if internalInboxReviewInput(m) {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if strings.HasPrefix(content, "[View work](/tasks/") && strings.HasSuffix(content, ")") {
			continue
		}
		return &messages[i]
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if !internalInboxReviewInput(messages[i]) {
			return &messages[i]
		}
	}
	return nil
}

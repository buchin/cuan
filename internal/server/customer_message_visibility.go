package server

import "github.com/buchin/cuan/internal/db"

// Visitor history, receipts and outbound email must agree on visibility.
func customerMessageInternal(m *db.ProjectMessage) bool {
	if m == nil {
		return true
	}
	// Messages sent by visitors / customers are always public customer messages, never internal notes.
	// Even if a visitor includes an @ mention (e.g. '@gvinum' or '@cuan'), it must not be treated as an internal whisper.
	if m.AuthorType == "user" && m.AuthorID != "operator_user" && m.AuthorName != "Owner" && m.AuthorName != "You" {
		return false
	}
	if m.AuthorType == "user" && (m.AuthorID == "operator_user" || m.AuthorName == "Owner" || m.AuthorName == "You") {
		if decodeInboxPlanDecision(m.Content) != nil || decodeRegenerateReplyID(m.Content) != "" {
			return true
		}
		return extractMentionHandle(m.Content) != ""
	}
	return m.AuthorType == "system" || (m.AuthorType == "agent" && m.AuthorID != "operator_user")
}

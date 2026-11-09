package server

import "github.com/buchin/cuan/internal/db"

// Visitor history, receipts and outbound email must agree on visibility.
func customerMessageInternal(m *db.ProjectMessage) bool {
	if m == nil {
		return true
	}
	if m.AuthorType == "user" && m.AuthorID == "operator_user" && (decodeInboxPlanDecision(m.Content) != nil || decodeRegenerateReplyID(m.Content) != "") {
		return true
	}
	return extractMentionHandle(m.Content) != "" || m.AuthorType == "system" || (m.AuthorType == "agent" && m.AuthorID != "operator_user")
}

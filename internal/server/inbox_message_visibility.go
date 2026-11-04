package server

import "github.com/buchin/cuan/internal/db"

// Coordinator inputs are jobs, not messages addressed to the person reading
// Inbox. Actual system status updates and agent replies remain visible.
func internalInboxReviewInput(m db.ProjectMessage) bool {
	return m.AuthorType == "system" && m.AuthorID == "cuan_work_review"
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

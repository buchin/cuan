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

// isSystemLifecycleNotice identifies internal system status and lifecycle event messages
// (e.g. server updates, restarts, capacity adjustments, and inspection notices).
// These messages are informative records preserved in the timeline, but they must
// be automatically treated as read, must never increase unread counters, and must not
// reopen or keep conversations in the unresolved ("Belum tuntas") filter.
func isSystemLifecycleNotice(m db.ProjectMessage) bool {
	if m.AuthorType == "system" {
		if m.AuthorID == "cuan_system_lifecycle" || m.AuthorID == "system_lifecycle" || m.AuthorID == "system_status" {
			return true
		}
		content := strings.TrimSpace(m.Content)
		if strings.HasPrefix(content, "Sistem baru saja diperbarui.") ||
			strings.HasPrefix(content, "Penyimpanan server sempat penuh.") ||
			strings.HasPrefix(content, "Kapasitas AI sedang disesuaikan.") ||
			strings.HasPrefix(content, "Pekerjaan sebelumnya sempat terhenti.") ||
			strings.HasPrefix(content, "Cuan memeriksa hasil pekerjaan") ||
			strings.HasPrefix(content, "Cuan memeriksa laporan yang sudah tersimpan") {
			return true
		}
	}
	return false
}

// hasOnlySystemLifecycleNotices checks if a conversation's messages consist exclusively
// of system lifecycle / update notices (or internal review inputs) without any conversational user/agent dialogue.
func hasOnlySystemLifecycleNotices(messages []db.ProjectMessage) bool {
	if len(messages) == 0 {
		return false
	}
	for _, m := range messages {
		if !isSystemLifecycleNotice(m) && !internalInboxReviewInput(m) {
			return false
		}
	}
	return true
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
		if internalInboxReviewInput(m) || isSystemLifecycleNotice(m) {
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

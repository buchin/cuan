package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type inboxTaskReviewKey struct{}

// Result review uses the canonical task snapshot rather than replaying an entire
// conversation, which may contain obsolete agent actions or false completion.
func (s *Server) inboxTaskReviewPrompt(ctx context.Context, q db.InboxRequest) (string, bool) {
	if s.projectStudioRepo == nil {
		return "", false
	}
	m, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
	if err != nil || m == nil || m.RoomID != q.RoomID || m.AuthorType != "system" || m.AuthorID != "cuan_work_review" {
		return "", false
	}
	d := s.inboxReviewedTask(ctx, q)
	if d == nil {
		return "", true
	}
	return s.inboxReviewData(ctx, d), true
}

// Explain a stale automatic update using current saved state, without running AI
// or granting permission to resume cancelled work.
func (s *Server) inboxStaleReviewOutcome(ctx context.Context, q db.InboxRequest) string {
	const fallback = "This update is no longer current. Your saved work is unchanged; no work was restarted or marked complete by this update."
	if s.projectStudioRepo == nil || s.taskSvc == nil {
		return fallback
	}
	m, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
	if err != nil || m == nil || m.RoomID != q.RoomID || m.AuthorType != "system" || m.AuthorID != "cuan_work_review" {
		return fallback
	}
	for _, match := range inboxReviewTaskLink.FindAllStringSubmatch(m.Content, -1) {
		d, err := s.taskSvc.GetTaskDetail(ctx, match[1])
		if err != nil || d == nil || d.LatestAttempt == nil {
			continue
		}
		a := d.LatestAttempt
		if a.CancelRequested || a.Status == task.AttemptStatusCanceled || a.CanceledAt != nil {
			return "The work has stopped. Your conversation and saved work are preserved. I will wait for your instruction before continuing it."
		}
	}
	return fallback
}

package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
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
	return inboxReviewData(d), true
}

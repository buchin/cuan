package server

import (
	"context"
	"sort"

	"github.com/buchin/cuan/internal/task"
)

// taskDeliveredContext exposes only files actually delivered by this task's
// attempts into its original conversation. Excerpts are untrusted source data,
// never acceptance evidence or permission to repeat the worker's actions.
func (s *Server) taskDeliveredContext(ctx context.Context, d *task.TaskDetailResponse) []map[string]string {
	if d == nil || s.projectStudioRepo == nil || d.SourceRoomID == "" {
		return nil
	}
	owner, err := s.taskArtifactOwner(ctx, d)
	if err != nil || owner == "" {
		return nil
	}
	attempts := append([]*task.TaskAttempt(nil), d.Attempts...)
	sort.SliceStable(attempts, func(i, j int) bool {
		if attempts[i] == nil {
			return false
		}
		if attempts[j] == nil {
			return true
		}
		return attempts[i].CreatedAt.After(attempts[j].CreatedAt)
	})
	budget := 12000
	var result []map[string]string
	for _, attempt := range attempts {
		if attempt == nil || attempt.TaskID != d.ID {
			continue
		}
		for _, prefix := range []string{"task-delivery:", "task-visual-delivery:"} {
			message, err := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID(prefix+attempt.ID))
			if err != nil || message == nil || message.RoomID != d.SourceRoomID || message.AuthorType != "agent" || message.ReplyToID == nil || *message.ReplyToID != d.SourceMessageID {
				continue
			}
			_, ids := decodeAttachmentMessage(message.Content)
			for _, id := range ids {
				if len(result) >= 8 {
					return result
				}
				attachment, path, err := loadAttachment(owner, id)
				if err != nil {
					continue
				}
				row := map[string]string{"source_attempt_id": attempt.ID, "attachment_id": attachment.ID, "name": attachment.Name, "media_type": attachment.Type}
				if reviewableTextMediaType(attachment.Type) && budget > 0 {
					if excerpt, scope, err := reviewTextExcerpt(path, min(6000, budget)); err == nil && excerpt != "" {
						row["untrusted_excerpt"] = excerpt
						row["excerpt_scope"] = scope
						budget -= len(excerpt)
					}
				}
				result = append(result, row)
			}
		}
	}
	return result
}

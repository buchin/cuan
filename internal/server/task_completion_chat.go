package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type taskReviewCompletionKey struct{}
type taskReviewRoomIDKey struct{}

// PostTaskCompletionMessage delivers a closing delivery summary and verification note
// to the associated chat room when a task transitions to 'done' (closing the communication loop).
func (s *Server) PostTaskCompletionMessage(ctx context.Context, t *task.Task) error {
	if ctx != nil && ctx.Value(taskReviewCompletionKey{}) != nil {
		reviewRoom, _ := ctx.Value(taskReviewRoomIDKey{}).(string)
		if reviewRoom == "" || t == nil || t.SourceRoomID == reviewRoom {
			// Post-verify review in inbox is actively handling this task completion
			// and will emit the unified confirmation message. Avoid duplicate emitter.
			return nil
		}
	}
	if s.projectStudioRepo == nil || t == nil || t.SourceRoomID == "" || t.Status != task.TaskStatusDone {
		return nil
	}

	msgID := delegationMessageID("task-completed-closing:" + t.ID)

	// Idempotency: avoid creating duplicate message if this task's closing note already exists
	if existing, err := s.projectStudioRepo.GetMessageByID(ctx, msgID); err == nil && existing != nil {
		return nil
	}

	summary := strings.TrimSpace(t.Summary)
	completeNote := strings.TrimSpace(t.CompleteNote)

	// Single-bubble idempotency: avoid inserting duplicate message if already present in room
	if msgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, t.SourceRoomID); err == nil && len(msgs) > 0 {
		for i := len(msgs) - 1; i >= 0 && i >= len(msgs)-10; i-- {
			content := msgs[i].Content
			if msgs[i].ID == msgID || (strings.Contains(content, "/tasks/"+t.ID) &&
				(strings.Contains(content, "telah selesai") ||
					strings.Contains(content, "sudah diterima") ||
					(summary != "" && strings.Contains(content, summary)))) {
				return nil
			}
		}
	}

	var sb strings.Builder
	title := strings.TrimSpace(t.Title)
	if title != "" {
		sb.WriteString(fmt.Sprintf("Pekerjaan **%s** telah selesai dan diverifikasi.\n\n", title))
	} else {
		sb.WriteString("Pekerjaan telah selesai dan seluruh hasil telah diverifikasi.\n\n")
	}

	if summary != "" {
		sb.WriteString("### Ringkasan Hasil\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}

	if completeNote != "" && completeNote != summary {
		sb.WriteString("### Bukti Verifikasi\n")
		sb.WriteString(completeNote)
		sb.WriteString("\n\n")
	}

	taskLink := fmt.Sprintf("[Lihat rincian pekerjaan](/tasks/%s)", t.ID)
	sb.WriteString(taskLink)
	content := strings.TrimSpace(sb.String())

	authorID := "agent_cuan"
	authorName := "@cuan"
	if t.AssignedAgent != "" {
		authorID = t.AssignedAgent
		if s.agentRepo != nil {
			if ag, err := s.runtimeAgent(ctx, t.AssignedAgent); err == nil && ag != nil && ag.Name != "" {
				authorName = ag.Name
			}
		}
	}

	now := time.Now().UTC()
	var replyTo *string
	if t.SourceMessageID != "" {
		replyTo = &t.SourceMessageID
	}

	msg := &db.ProjectMessage{
		ID:         msgID,
		RoomID:     t.SourceRoomID,
		AuthorType: "agent",
		AuthorID:   authorID,
		AuthorName: authorName,
		Content:    content,
		ReplyToID:  replyTo,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := s.projectStudioRepo.CreateMessage(ctx, msg)
	if err == nil || errors.Is(err, db.ErrDuplicateMessage) {
		s.publishInbox(t.SourceRoomID)
		if s.roomEventBroker != nil {
			s.roomEventBroker.Broadcast(t.SourceRoomID, "message.created", msg.ToDTO())
		}
		return nil
	}
	return err
}

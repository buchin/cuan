package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrConversationChanged = errors.New("conversation has newer messages; review them before resolving")

// SetRoomResolved preserves all messages, tasks and timestamps used for list ordering.
// A cutoff rejects resolution of messages received after the operator's observation.
func (r *SQLProjectStudioRepository) SetRoomResolved(ctx context.Context, id string, at *time.Time) error {
	if !IsValidUUID(id) {
		return ErrInvalidUUID
	}
	result, err := r.db.ExecContext(ctx, `UPDATE project_rooms SET resolved_at=$2 WHERE id=$1 AND ($2::timestamptz IS NULL OR NOT EXISTS (SELECT 1 FROM project_messages WHERE room_id=$1 AND created_at>$2 AND author_id NOT IN ('cuan_system_lifecycle', 'cuan_work_review')))`, id, at)
	if err != nil {
		return fmt.Errorf("save conversation resolution: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := r.GetRoomByID(ctx, id); err != nil {
			return err
		}
		return ErrConversationChanged
	}
	return nil
}

func (m *MemoryProjectStudioRepository) SetRoomResolved(ctx context.Context, id string, at *time.Time) error {
	if !IsValidUUID(id) {
		return ErrInvalidUUID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	room := m.rooms[id]
	if room == nil {
		return ErrRoomNotFound
	}
	if at != nil {
		for _, msg := range m.messages {
			if msg.RoomID == id && msg.CreatedAt.After(*at) {
				if msg.AuthorID == "cuan_system_lifecycle" || msg.AuthorID == "cuan_work_review" || strings.HasPrefix(strings.TrimSpace(msg.Content), "Sistem baru saja diperbarui") {
					continue
				}
				return ErrConversationChanged
			}
		}
		copied := at.UTC()
		room.ResolvedAt = &copied
	} else {
		room.ResolvedAt = nil
	}
	return nil
}

// New activity reopens a conversation without erasing its last resolution marker.
// Callers must use the latest persisted message time, not a read receipt.
func (r *ProjectRoom) IsResolved(latestMessage time.Time) bool {
	return r != nil && r.ResolvedAt != nil && !latestMessage.After(*r.ResolvedAt)
}

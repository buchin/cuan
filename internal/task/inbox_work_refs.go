package task

import (
	"context"
	"time"
)

// InboxWorkRef identifies generated work updates without loading every task's
// instructions, reports and log history on each conversation refresh.
type InboxWorkRef struct {
	TaskID, AttemptID, SourceMessageID, Status string
	AttemptNumber                              int
}
type InboxWorkRefStore interface {
	InboxWorkRefs(context.Context, string) ([]InboxWorkRef, error)
}

func (s *DBStore) InboxWorkRefs(ctx context.Context, room string) ([]InboxWorkRef, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id,a.id,COALESCE(t.source_message_id::text,''),t.status,a.attempt_number FROM tasks t JOIN task_attempts a ON a.task_id=t.id WHERE t.source_room_id=$1::uuid`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []InboxWorkRef
	for rows.Next() {
		var ref InboxWorkRef
		if err := rows.Scan(&ref.TaskID, &ref.AttemptID, &ref.SourceMessageID, &ref.Status, &ref.AttemptNumber); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}
func (m *MemoryStore) InboxWorkRefs(_ context.Context, room string) ([]InboxWorkRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var refs []InboxWorkRef
	for _, a := range m.attempts {
		if t := m.tasks[a.TaskID]; t != nil && t.SourceRoomID == room {
			refs = append(refs, InboxWorkRef{TaskID: t.ID, AttemptID: a.ID, SourceMessageID: t.SourceMessageID, Status: t.Status, AttemptNumber: a.AttemptNumber})
		}
	}
	return refs, nil
}

// InboxLaunchRef retains native launch identity without loading revisions or credentials.
type InboxLaunchRef struct {
	ID, TaskID, RequestID, State, SafeError string
	CreatedAt, UpdatedAt                    time.Time
}
type InboxLaunchRefStore interface {
	InboxLaunchRefs(context.Context, string) ([]InboxLaunchRef, error)
}

func (s *DBStore) InboxLaunchRefs(ctx context.Context, room string) ([]InboxLaunchRef, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,l.task_id,l.request_id,l.state,COALESCE(l.safe_error,''),l.created_at,l.updated_at FROM task_launches l JOIN tasks t ON t.id=l.task_id WHERE l.room_id=$1::uuid AND t.source_room_id=$1::uuid ORDER BY l.created_at,l.id`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []InboxLaunchRef
	for rows.Next() {
		var ref InboxLaunchRef
		if err := rows.Scan(&ref.ID, &ref.TaskID, &ref.RequestID, &ref.State, &ref.SafeError, &ref.CreatedAt, &ref.UpdatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}
func (m *MemoryStore) InboxLaunchRefs(_ context.Context, room string) ([]InboxLaunchRef, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var refs []InboxLaunchRef
	for _, l := range m.launches {
		if t := m.tasks[l.TaskID]; t != nil && t.SourceRoomID == room && l.RoomID == room {
			refs = append(refs, InboxLaunchRef{ID: l.ID, TaskID: l.TaskID, RequestID: l.RequestID, State: l.State, SafeError: l.SafeError, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt})
		}
	}
	return refs, nil
}

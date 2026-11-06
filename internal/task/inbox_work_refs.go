package task

import "context"

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

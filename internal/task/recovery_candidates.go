package task

import "context"

// RecoveryTaskStore selects reconciliation candidates using status indexes.
// Board state alone is insufficient: an active attempt can outlive a stale board.
type RecoveryTaskStore interface {
	RecoveryTaskIDs(context.Context) ([]string, error)
}

// RecoveryTasks avoids loading completed history and its per-task read models
// on every dispatcher/reaper tick. Custom legacy stores retain the old fallback.
func (s *Service) RecoveryTasks(ctx context.Context) ([]*Task, error) {
	if reader, ok := s.store.(RecoveryTaskStore); ok {
		ids, err := reader.RecoveryTaskIDs(ctx)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return []*Task{}, nil
		}
		return s.store.ListTasks(ctx, TaskFilter{IDs: ids})
	}
	return s.store.ListTasks(ctx, TaskFilter{})
}

func (s *DBStore) RecoveryTaskIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM tasks WHERE status='doing'
 UNION SELECT task_id FROM task_attempts WHERE status='running' OR (status='queued' AND runner_id='local_terminal')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (m *MemoryStore) RecoveryTaskIDs(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := map[string]bool{}
	for _, t := range m.tasks {
		if t.Status == TaskStatusDoing {
			ids[t.ID] = true
		}
	}
	for _, a := range m.attempts {
		if a.Status == AttemptStatusRunning || (a.Status == AttemptStatusQueued && a.RunnerID == "local_terminal") {
			ids[a.TaskID] = true
		}
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	return out, nil
}

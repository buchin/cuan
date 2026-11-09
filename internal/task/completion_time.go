package task

import (
	"context"
	"time"
)

// Completion is a lifecycle event, not the last edit or the worker's exit.
// Missing history stays unknown; reopening never exposes an old completion.
func completionTime(t *Task, activities []*TaskActivity) *time.Time {
	if t == nil || t.Status != TaskStatusDone {
		return nil
	}
	var latest *TaskActivity
	for _, a := range activities {
		if a == nil || a.TaskID != t.ID || a.ActivityType != ActivityTypeStatusChange {
			continue
		}
		if latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID) {
			latest = a
		}
	}
	if latest == nil || latest.NewStatus != TaskStatusDone || latest.CreatedAt.IsZero() || latest.CreatedAt.Before(t.CreatedAt) {
		return nil
	}
	at := latest.CreatedAt
	return &at
}

type completionTimeReader interface {
	CompletionTimes(context.Context, []string) (map[string]time.Time, error)
}

func validCompletionTime(t *Task, at time.Time) *time.Time {
	if t.Status != TaskStatusDone || at.IsZero() || at.Before(t.CreatedAt) {
		return nil
	}
	return &at
}

func (s *DBStore) CompletionTimes(ctx context.Context, ids []string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_id,new_status,created_at FROM (
		SELECT DISTINCT ON (task_id) task_id,new_status,created_at
		FROM task_activities WHERE task_id=ANY($1::text[]) AND activity_type='status_change'
		ORDER BY task_id,created_at DESC,id DESC
	) latest WHERE new_status='done'`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, status string
		var at time.Time
		if err := rows.Scan(&id, &status, &at); err != nil {
			return nil, err
		}
		out[id] = at
	}
	return out, rows.Err()
}

func (m *MemoryStore) CompletionTimes(ctx context.Context, ids []string) (map[string]time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[string]time.Time{}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	grouped := map[string][]*TaskActivity{}
	for _, a := range m.activities {
		if a != nil && wanted[a.TaskID] && a.ActivityType == ActivityTypeStatusChange {
			grouped[a.TaskID] = append(grouped[a.TaskID], a)
		}
	}
	for _, id := range ids {
		if at := completionTime(m.tasks[id], grouped[id]); at != nil {
			out[id] = *at
		}
	}
	return out, nil
}

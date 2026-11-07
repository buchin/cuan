package task

import (
	"context"
	"sort"
)

// CurrentAttemptActivityStore supports bounded list projections without loading
// historical attempts or issuing one activity query for each visible task.
type CurrentAttemptActivityStore interface {
	CurrentAttemptActivities(context.Context, []string, []string) (map[string][]*TaskActivity, error)
}

func (s *DBStore) CurrentAttemptActivities(ctx context.Context, ids, kinds []string) (map[string][]*TaskActivity, error) {
	out := map[string][]*TaskActivity{}
	if len(ids) == 0 || len(kinds) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,e.task_id,e.attempt_id,e.activity_type,e.author,COALESCE(e.old_status,''),COALESCE(e.new_status,''),e.message,e.created_at
 FROM task_activities e JOIN task_attempts a ON a.id=e.attempt_id
 WHERE e.task_id=ANY($1::text[]) AND e.activity_type=ANY($2::text[])
 AND a.task_id=e.task_id AND a.id=(SELECT latest.id FROM task_attempts latest WHERE latest.task_id=e.task_id ORDER BY latest.created_at DESC,latest.id DESC LIMIT 1)
 ORDER BY e.created_at,e.id`, ids, kinds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a TaskActivity
		if err := rows.Scan(&a.ID, &a.TaskID, &a.AttemptID, &a.ActivityType, &a.Author, &a.OldStatus, &a.NewStatus, &a.Message, &a.CreatedAt); err != nil {
			return nil, err
		}
		out[a.TaskID] = append(out[a.TaskID], &a)
	}
	return out, rows.Err()
}

func (m *MemoryStore) CurrentAttemptActivities(ctx context.Context, ids, kinds []string) (map[string][]*TaskActivity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	wanted, types := map[string]bool{}, map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	for _, kind := range kinds {
		types[kind] = true
	}
	latest := map[string]*TaskAttempt{}
	for _, a := range m.attempts {
		if !wanted[a.TaskID] {
			continue
		}
		old := latest[a.TaskID]
		if old == nil || a.CreatedAt.After(old.CreatedAt) || (a.CreatedAt.Equal(old.CreatedAt) && a.ID > old.ID) {
			latest[a.TaskID] = a
		}
	}
	out := map[string][]*TaskActivity{}
	for _, a := range m.activities {
		current := latest[a.TaskID]
		if current == nil || a.AttemptID == nil || *a.AttemptID != current.ID || !types[a.ActivityType] {
			continue
		}
		cp := *a
		out[a.TaskID] = append(out[a.TaskID], &cp)
	}
	for _, rows := range out {
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].CreatedAt.Before(rows[j].CreatedAt) || (rows[i].CreatedAt.Equal(rows[j].CreatedAt) && rows[i].ID < rows[j].ID)
		})
	}
	return out, nil
}

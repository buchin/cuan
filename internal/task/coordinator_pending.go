package task

import (
	"context"
	"sort"
	"time"
)

const PhaseHandoffIntent = "phase_handoff_pending"

// PendingCoordinatorStore reads bounded durable intents, without scanning tasks
// and loading each task's complete history. Terminal records use intentID+":done".
type PendingCoordinatorStore interface {
	PendingPhaseHandoffs(context.Context, time.Time, int) ([]*TaskActivity, error)
}

func (s *DBStore) PendingPhaseHandoffs(ctx context.Context, now time.Time, limit int) ([]*TaskActivity, error) {
	if limit < 1 || limit > 20 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.task_id,a.attempt_id,a.activity_type,a.author,a.message,a.created_at FROM task_activities a WHERE a.activity_type='phase_handoff_pending' AND a.author='cuan-plan' AND a.created_at <= $1 AND NOT EXISTS (SELECT 1 FROM task_activities done WHERE done.id=a.id||':done' AND done.author='cuan-plan' AND done.activity_type='phase_handoff_finished') AND NOT EXISTS (SELECT 1 FROM task_activities retry WHERE retry.task_id=a.task_id AND retry.message=a.id AND retry.author='cuan-plan' AND retry.activity_type='phase_handoff_retry' AND retry.created_at > $1 - INTERVAL '5 minutes') ORDER BY a.created_at,a.id LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TaskActivity
	for rows.Next() {
		a := new(TaskActivity)
		if err := rows.Scan(&a.ID, &a.TaskID, &a.AttemptID, &a.ActivityType, &a.Author, &a.Message, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (m *MemoryStore) PendingPhaseHandoffs(ctx context.Context, now time.Time, limit int) ([]*TaskActivity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit < 1 || limit > 20 {
		limit = 20
	}
	var out []*TaskActivity
	for _, a := range m.activities {
		if a.ActivityType != PhaseHandoffIntent || a.Author != "cuan-plan" || a.CreatedAt.After(now) {
			continue
		}
		if done := m.activities[a.ID+":done"]; done != nil && done.Author == "cuan-plan" && done.ActivityType == "phase_handoff_finished" {
			continue
		}
		waiting := false
		for _, retry := range m.activities {
			if retry.TaskID == a.TaskID && retry.ActivityType == "phase_handoff_retry" && retry.Author == "cuan-plan" && retry.Message == a.ID && retry.CreatedAt.After(now.Add(-5*time.Minute)) {
				waiting = true
				break
			}
		}
		if !waiting {
			copy := *a
			out = append(out, &copy)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

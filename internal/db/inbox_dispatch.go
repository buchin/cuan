package db

import (
	"context"
	"sort"
	"time"
)

// Find durable pending work directly, including customer conversations which
// need not have a project-room record. Historical idle rooms are not jobs.
type InboxDispatchRepository interface {
	ListInboxDispatchRooms(context.Context, time.Time) ([]string, error)
}

func (r *SQLProjectStudioRepository) ListInboxDispatchRooms(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT room_id::text FROM inbox_requests
 WHERE state IN ('queued','running','cancelling') OR (state='failed' AND EXISTS (
 SELECT 1 FROM jsonb_array_elements(steps) step WHERE step->>'kind'='finished' AND (step->>'at')::timestamptz >= $1))
 GROUP BY room_id ORDER BY min(created_at),room_id`, now.Add(-2*time.Minute))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (r *MemoryProjectStudioRepository) ListInboxDispatchRooms(_ context.Context, now time.Time) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	oldest := map[string]time.Time{}
	for _, q := range r.requests {
		eligible := q.State == "queued" || q.State == "running" || q.State == "cancelling"
		if q.State == "failed" {
			for _, step := range q.Steps {
				if step.Kind == "finished" && !step.At.Before(now.Add(-2*time.Minute)) {
					eligible = true
				}
			}
		}
		if eligible {
			if at, ok := oldest[q.RoomID]; !ok || q.CreatedAt.Before(at) {
				oldest[q.RoomID] = q.CreatedAt
			}
		}
	}
	ids := make([]string, 0, len(oldest))
	for id := range oldest {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if oldest[ids[i]].Equal(oldest[ids[j]]) {
			return ids[i] < ids[j]
		}
		return oldest[ids[i]].Before(oldest[ids[j]])
	})
	return ids, nil
}

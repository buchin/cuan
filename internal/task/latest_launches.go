package task

import "context"

type launchInfoReader interface {
	LatestLaunchInfos(context.Context, []string) (map[string]*LaunchInfo, error)
}

// LatestLaunchInfos reads only presentation fields, never owner credentials or scope.
// A launch bound to an active execution takes precedence over newer admission
// requests; a rejected duplicate must not describe the task as stopped.
func (s *DBStore) LatestLaunchInfos(ctx context.Context, ids []string) (map[string]*LaunchInfo, error) {
	result := map[string]*LaunchInfo{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT ON (j.task_id) j.task_id,j.state,j.safe_error,j.available_at FROM task_launches j LEFT JOIN task_attempts a ON a.id=j.attempt_id AND a.task_id=j.task_id AND a.status IN ('queued','running') WHERE j.task_id=ANY($1::text[]) ORDER BY j.task_id,(a.id IS NOT NULL) DESC,j.created_at DESC,j.id DESC`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var info LaunchInfo
		if err := rows.Scan(&id, &info.State, &info.SafeError, &info.AvailableAt); err != nil {
			return nil, err
		}
		result[id] = &info
	}
	return result, rows.Err()
}
func (m *MemoryStore) LatestLaunchInfos(_ context.Context, ids []string) (map[string]*LaunchInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := map[string]*LaunchInfo{}
	wanted := map[string]bool{}
	latest := map[string]*TaskLaunch{}
	for _, id := range ids {
		wanted[id] = true
	}
	active := func(launch *TaskLaunch) bool {
		if launch == nil {
			return false
		}
		a := m.attempts[attemptRequestID(launch.TaskID, launch.RequestID)]
		return a != nil && a.TaskID == launch.TaskID && (a.Status == AttemptStatusQueued || a.Status == AttemptStatusRunning)
	}
	for _, j := range m.launches {
		if !wanted[j.TaskID] {
			continue
		}
		prev := latest[j.TaskID]
		if prev == nil || (active(j) && !active(prev)) || (active(j) == active(prev) && (j.CreatedAt.After(prev.CreatedAt) || (j.CreatedAt.Equal(prev.CreatedAt) && j.ID > prev.ID))) {
			latest[j.TaskID] = j
		}
	}
	for id, j := range latest {
		result[id] = &LaunchInfo{State: j.State, SafeError: j.SafeError, AvailableAt: j.AvailableAt}
	}
	return result, nil
}

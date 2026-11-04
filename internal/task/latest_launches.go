package task

import "context"

type launchInfoReader interface {
	LatestLaunchInfos(context.Context, []string) (map[string]*LaunchInfo, error)
}

// LatestLaunchInfos reads only presentation fields, never owner credentials or scope.
func (s *DBStore) LatestLaunchInfos(ctx context.Context, ids []string) (map[string]*LaunchInfo, error) {
	result := map[string]*LaunchInfo{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT ON (task_id) task_id,state,safe_error,available_at FROM task_launches WHERE task_id=ANY($1::text[]) ORDER BY task_id,created_at DESC,id DESC`, ids)
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
	for _, j := range m.launches {
		if !wanted[j.TaskID] {
			continue
		}
		prev := latest[j.TaskID]
		if prev == nil || j.CreatedAt.After(prev.CreatedAt) || (j.CreatedAt.Equal(prev.CreatedAt) && j.ID > prev.ID) {
			latest[j.TaskID] = j
		}
	}
	for id, j := range latest {
		result[id] = &LaunchInfo{State: j.State, SafeError: j.SafeError, AvailableAt: j.AvailableAt}
	}
	return result, nil
}

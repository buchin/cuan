package task

import "context"

type attemptLaunchBatchReader interface {
	LaunchesForAttempts(context.Context, []string) (map[string]*TaskLaunch, error)
}

// A presentation-only classification of the exact bound execution. Pending
// continuations, report prose and historical verifier sessions supply no role.
func executionReviewState(t *Task, a *TaskAttempt, launch *TaskLaunch) string {
	if t == nil || a == nil || launch == nil || a.TaskID != t.ID || launch.TaskID != t.ID || attemptRequestID(t.ID, launch.RequestID) != a.ID || t.Status == TaskStatusDone || t.Status == "cancelled" || t.Status == "canceled" || a.CancelRequested || a.CanceledAt != nil || (a.Status != AttemptStatusRunning && a.Status != AttemptStatusQueued) {
		return ""
	}
	if a.Status == AttemptStatusRunning && (a.RunnerID == "" || a.StartedAt == nil || a.FinishedAt != nil) {
		return ""
	}
	kind := diagnoseLaunch(t, launch).IntentKind
	if kind == "verification" || kind == "live_result_inspection" || kind == "current_outcome_inspection" {
		return a.Status
	}
	return ""
}

func (s *Service) executionReviewState(ctx context.Context, t *Task, a *TaskAttempt) string {
	if a == nil {
		return ""
	}
	if reader, ok := s.store.(AttemptLaunchStore); ok {
		launch, err := reader.LaunchForAttempt(ctx, a.ID)
		if err == nil {
			return executionReviewState(t, a, launch)
		}
	}
	return ""
}

func (s *DBStore) LaunchesForAttempts(ctx context.Context, ids []string) (map[string]*TaskLaunch, error) {
	result := map[string]*TaskLaunch{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+launchColumns+` FROM task_launches WHERE attempt_id = ANY($1::text[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		launch, err := scanLaunch(rows)
		if err != nil {
			return nil, err
		}
		result[attemptRequestID(launch.TaskID, launch.RequestID)] = launch
	}
	return result, rows.Err()
}

func (m *MemoryStore) LaunchesForAttempts(_ context.Context, ids []string) (map[string]*TaskLaunch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	result := map[string]*TaskLaunch{}
	for _, launch := range m.launches {
		id := attemptRequestID(launch.TaskID, launch.RequestID)
		if wanted[id] {
			result[id] = copyLaunch(launch)
		}
	}
	return result, nil
}

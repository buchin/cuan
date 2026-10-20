package task

import "context"

// ResultClassification belongs to one final report, not to the task forever.
// A subsequent attempt must not inherit the previous answer's display status.
type ResultClassification struct {
	AttemptID string
	Kind      string
}

func classifyResult(activities []*TaskActivity) ResultClassification {
	var report *TaskActivity
	for _, a := range activities {
		if a.ActivityType == "agent_report" && a.AttemptID != nil && (a.NewStatus == "review" || a.NewStatus == "done" || a.NewStatus == "blocked") && (report == nil || a.CreatedAt.After(report.CreatedAt)) {
			report = a
		}
	}
	if report == nil {
		return ResultClassification{}
	}
	for _, a := range activities {
		if a.ActivityType == "agent_result_kind" && a.AttemptID != nil && *a.AttemptID == *report.AttemptID && a.CreatedAt.Equal(report.CreatedAt) && (a.NewStatus == "answer" || a.NewStatus == "implementation") {
			return ResultClassification{*a.AttemptID, a.NewStatus}
		}
	}
	return ResultClassification{}
}
func (m *MemoryStore) LatestResultClassifications(_ context.Context, ids []string) (map[string]ResultClassification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	grouped := map[string][]*TaskActivity{}
	for _, a := range m.activities {
		if wanted[a.TaskID] {
			grouped[a.TaskID] = append(grouped[a.TaskID], a)
		}
	}
	result := map[string]ResultClassification{}
	for id, activities := range grouped {
		result[id] = classifyResult(activities)
	}
	return result, nil
}
func (s *DBStore) LatestResultClassifications(ctx context.Context, ids []string) (map[string]ResultClassification, error) {
	result := map[string]ResultClassification{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `WITH reports AS (
 SELECT DISTINCT ON(task_id) task_id,attempt_id,created_at FROM task_activities
 WHERE task_id = ANY($1::text[]) AND activity_type='agent_report' AND new_status IN ('review','done','blocked')
 ORDER BY task_id,created_at DESC,id DESC
 ) SELECT r.task_id,r.attempt_id,k.new_status FROM reports r JOIN task_activities k
 ON k.task_id=r.task_id AND k.attempt_id=r.attempt_id AND k.created_at=r.created_at
 WHERE k.activity_type='agent_result_kind' AND k.new_status IN ('answer','implementation')`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var c ResultClassification
		if err = rows.Scan(&id, &c.AttemptID, &c.Kind); err != nil {
			return nil, err
		}
		result[id] = c
	}
	return result, rows.Err()
}
func resultKindForAttempt(c ResultClassification, a *TaskAttempt) string {
	if a == nil || a.ID != c.AttemptID {
		return ""
	}
	return c.Kind
}

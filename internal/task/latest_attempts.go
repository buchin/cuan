package task

import (
	"context"
	"fmt"
)

func (s *DBStore) LatestAttempts(ctx context.Context, ids []string) (map[string]*TaskAttempt, error) {
	result := map[string]*TaskAttempt{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT ON (task_id)
 id,task_id,runner_id,agent_name,status,exit_code,peak_rss_bytes,
 lease_expires_at,COALESCE(cancel_requested,false),canceled_at,interrupted_at,
 COALESCE(attempt_number,1),started_at,finished_at,COALESCE(error_message,''),created_at,request_hash,execution_deadline
 FROM task_attempts WHERE task_id=ANY($1::text[]) ORDER BY task_id,created_at DESC,id DESC`, ids)
	if err != nil {
		return nil, fmt.Errorf("batch latest attempts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a TaskAttempt
		if err := rows.Scan(&a.ID, &a.TaskID, &a.RunnerID, &a.AgentName, &a.Status, &a.ExitCode, &a.PeakRSSBytes, &a.LeaseExpiresAt, &a.CancelRequested, &a.CanceledAt, &a.InterruptedAt, &a.AttemptNumber, &a.StartedAt, &a.FinishedAt, &a.ErrorMessage, &a.CreatedAt, &a.RequestHash, &a.ExecutionDeadline); err != nil {
			return nil, err
		}
		result[a.TaskID] = &a
	}
	return result, rows.Err()
}
func (m *MemoryStore) LatestAttempts(ctx context.Context, ids []string) (map[string]*TaskAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wanted := map[string]bool{}
	result := map[string]*TaskAttempt{}
	for _, id := range ids {
		wanted[id] = true
	}
	for _, a := range m.attempts {
		if !wanted[a.TaskID] {
			continue
		}
		prev := result[a.TaskID]
		if prev == nil || a.CreatedAt.After(prev.CreatedAt) || (a.CreatedAt.Equal(prev.CreatedAt) && a.ID > prev.ID) {
			cp := *a
			result[a.TaskID] = &cp
		}
	}
	return result, nil
}

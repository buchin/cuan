package task

import (
	"context"
	"strings"
)

const ActivityVisualAcceptanceAudit = "visual_acceptance_audit"

type VerificationAcceptanceAuditStore interface {
	UninspectedVerificationTaskIDs(context.Context) ([]string, error)
}

func (s *DBStore) UninspectedVerificationTaskIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id FROM tasks t
 JOIN task_attempts a ON a.task_id=t.id AND a.id=(SELECT id FROM task_attempts WHERE task_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1)
 JOIN task_launches l ON l.attempt_id=a.id
 WHERE t.status='done' AND a.status='succeeded' AND l.revision LIKE $1
 AND NOT EXISTS(SELECT 1 FROM task_activities r WHERE r.task_id=t.id AND r.attempt_id=a.id AND r.activity_type=$2 AND r.author='coordinator')
 ORDER BY t.id LIMIT 50`, "%"+VerificationIntentMarker+"%", ActivityVisualAcceptanceAudit)
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
func (m *MemoryStore) UninspectedVerificationTaskIDs(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for _, t := range m.tasks {
		if t.Status != TaskStatusDone {
			continue
		}
		var latest *TaskAttempt
		for _, a := range m.attempts {
			if a.TaskID == t.ID && (latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID)) {
				latest = a
			}
		}
		if latest == nil || latest.Status != AttemptStatusSucceeded {
			continue
		}
		audited := false
		for _, a := range m.activities {
			if a.TaskID == t.ID && a.AttemptID != nil && *a.AttemptID == latest.ID && a.ActivityType == ActivityVisualAcceptanceAudit && a.Author == "coordinator" {
				audited = true
				break
			}
		}
		if audited {
			continue
		}
		for _, j := range m.launches {
			if j.TaskID == t.ID && attemptRequestID(t.ID, j.RequestID) == latest.ID && strings.Contains(j.Revision, VerificationIntentMarker) {
				ids = append(ids, t.ID)
				break
			}
		}
		if len(ids) >= 50 {
			break
		}
	}
	return ids, nil
}

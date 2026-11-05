package task

import (
	"context"
	"strings"
	"time"
)

const ReconciliationExecutionMarker = "Mandatory reconciliation before continuing the original approved scope:"
const OperationalAcceptanceAudit = "operational_acceptance_audit"

type OperationalAcceptanceAuditStore interface {
	OperationalAcceptanceTaskIDs(context.Context) ([]string, error)
	RequeueOperationalAcceptance(context.Context, string, string, time.Time) (bool, error)
}

func (s *DBStore) OperationalAcceptanceTaskIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id FROM tasks t
 JOIN LATERAL (SELECT id,status,cancel_requested,canceled_at FROM task_attempts WHERE task_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1) a ON true
 WHERE t.status='done' AND t.execution_category='ops' AND t.instructions LIKE $1
 AND a.status='succeeded' AND NOT a.cancel_requested AND a.canceled_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM task_activities x WHERE x.task_id=t.id AND x.attempt_id=a.id AND x.activity_type=$2)`, "%"+ReconciliationExecutionMarker+"%", OperationalAcceptanceAudit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (m *MemoryStore) OperationalAcceptanceTaskIDs(ctx context.Context) ([]string, error) {
	items, err := m.ListTasks(ctx, TaskFilter{Status: TaskStatusDone})
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, t := range items {
		if t.ExecutionCategory != ExecutionCategoryOps || !strings.Contains(t.Instructions, ReconciliationExecutionMarker) {
			continue
		}
		a, err := m.GetLatestAttempt(ctx, t.ID)
		if err != nil || a == nil || a.Status != AttemptStatusSucceeded || a.CancelRequested || a.CanceledAt != nil {
			continue
		}
		activities, _ := m.ListActivities(ctx, t.ID)
		audited := false
		for _, x := range activities {
			if x.ActivityType == OperationalAcceptanceAudit && x.AttemptID != nil && *x.AttemptID == a.ID {
				audited = true
			}
		}
		if !audited {
			ids = append(ids, t.ID)
		}
	}
	return ids, nil
}

// Compare the exact accepted snapshot and latest attempt so a concurrent user
// cancellation, edit or new execution always wins over the policy audit.
func (s *DBStore) RequeueOperationalAcceptance(ctx context.Context, id, attemptID string, updatedAt time.Time) (bool, error) {
	return s.RequeueAcceptedSnapshot(ctx, id, attemptID, updatedAt)
}

func (s *DBStore) RequeueAcceptedSnapshot(ctx context.Context, id, attemptID string, updatedAt time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE tasks t SET status='review',updated_at=NOW()
 WHERE t.id=$1 AND t.status='done' AND t.updated_at=$3
 AND (SELECT a.id FROM task_attempts a WHERE a.task_id=t.id ORDER BY a.created_at DESC,a.id DESC LIMIT 1)=$2
 AND EXISTS(SELECT 1 FROM task_attempts a WHERE a.id=$2 AND a.task_id=t.id AND a.status='succeeded' AND NOT a.cancel_requested AND a.canceled_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM task_launches l WHERE l.task_id=t.id AND l.state IN ('queued','claimed'))`, id, attemptID, updatedAt)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (m *MemoryStore) RequeueOperationalAcceptance(ctx context.Context, id, attemptID string, updatedAt time.Time) (bool, error) {
	return m.RequeueAcceptedSnapshot(ctx, id, attemptID, updatedAt)
}

func (m *MemoryStore) RequeueAcceptedSnapshot(ctx context.Context, id, attemptID string, updatedAt time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tasks[id]
	if t == nil || t.Status != TaskStatusDone || !t.UpdatedAt.Equal(updatedAt) {
		return false, nil
	}
	var latest *TaskAttempt
	for _, a := range m.attempts {
		if a.TaskID == id && (latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID)) {
			latest = a
		}
	}
	if latest == nil || latest.ID != attemptID || latest.Status != AttemptStatusSucceeded || latest.CancelRequested || latest.CanceledAt != nil {
		return false, nil
	}
	for _, l := range m.launches {
		if l.TaskID == id && (l.State == "queued" || l.State == "claimed") {
			return false, nil
		}
	}
	t.Status = TaskStatusReview
	t.UpdatedAt = time.Now().UTC()
	return true, nil
}

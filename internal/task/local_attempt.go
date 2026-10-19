package task

import (
	"context"
	"database/sql"
	"time"
)

// ReconcileLocalTask updates the board only while this remains the newest
// terminal attempt. A delayed exit callback cannot overwrite a newer run.
func (s *Service) ReconcileLocalTask(ctx context.Context, taskID, attemptID string) error {
	if store, ok := s.store.(interface {
		ReconcileLocalTask(context.Context, string, string) error
	}); ok {
		return store.ReconcileLocalTask(ctx, taskID, attemptID)
	}
	return nil
}
func (s *DBStore) ReconcileLocalTask(ctx context.Context, taskID, attemptID string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "task:"+taskID); e != nil {
		return e
	}
	var id, status string
	e = tx.QueryRowContext(ctx, `SELECT id,status FROM task_attempts WHERE task_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1`, taskID).Scan(&id, &status)
	if e == sql.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	if id != attemptID || status == "running" || status == "queued" {
		return nil
	}
	board := TaskStatusBlocked
	if status == AttemptStatusSucceeded {
		board = TaskStatusReview
	}
	if _, e = tx.ExecContext(ctx, `UPDATE tasks SET status=$2,updated_at=NOW() WHERE id=$1 AND status='doing'`, taskID, board); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *MemoryStore) ReconcileLocalTask(ctx context.Context, taskID, attemptID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest *TaskAttempt
	for _, a := range s.attempts {
		if a.TaskID == taskID && (latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID)) {
			latest = a
		}
	}
	if latest == nil || latest.ID != attemptID || latest.Status == "queued" || latest.Status == "running" {
		return nil
	}
	t, ok := s.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if t.Status != TaskStatusDoing {
		return nil
	}
	t.UpdatedAt = time.Now().UTC()
	t.Status = TaskStatusBlocked
	if latest.Status == AttemptStatusSucceeded {
		t.Status = TaskStatusReview
	}
	return nil
}
func (s *Service) AttemptForTask(ctx context.Context, taskID, attemptID string) (*TaskAttempt, error) {
	a, e := s.store.GetAttempt(ctx, attemptID)
	if e != nil {
		return nil, e
	}
	if a.TaskID != taskID {
		return nil, ErrAttemptNotFound
	}
	return a, nil
}

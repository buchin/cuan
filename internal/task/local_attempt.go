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
		var before *Task
		if s.extensions != nil {
			before, _ = s.store.GetTask(ctx, taskID)
		}
		err := store.ReconcileLocalTask(ctx, taskID, attemptID)
		if err == nil && before != nil {
			if after, e := s.store.GetTask(ctx, taskID); e == nil {
				s.extensionTransition(ctx, before.Status, after)
			}
		}
		return err
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
		var ready bool
		e = tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM task_activities WHERE attempt_id=$2 AND activity_type='agent_contract') OR (btrim(summary)<>'' AND btrim(complete_note)<>'' AND COALESCE((SELECT new_status IN ('review','done') FROM task_activities WHERE attempt_id=$2 AND activity_type='agent_report' ORDER BY created_at DESC,id DESC LIMIT 1),false)) FROM tasks WHERE id=$1`, taskID, attemptID).Scan(&ready)
		if e != nil {
			return e
		}
		if ready {
			board = TaskStatusReview
			var final, sourceRoom string
			e = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT new_status FROM task_activities WHERE attempt_id=$1 AND activity_type='agent_report' ORDER BY created_at DESC,id DESC LIMIT 1),''),COALESCE(source_room_id::text,'') FROM tasks WHERE id=$2`, attemptID, taskID).Scan(&final, &sourceRoom)
			if e != nil {
				return e
			}
			if final == "done" && sourceRoom == "" {
				board = TaskStatusDone
			}
		}
	}
	// A cancelled task may still have a later attempt run; an interrupted attempt
	// must still be surfaced instead of silently leaving the task in an unblocked state.
	result, e := tx.ExecContext(ctx, `UPDATE tasks SET status=$2,updated_at=NOW() WHERE id=$1 AND status IN ('doing','backlog','cancelled')`, taskID, board)
	if e != nil {
		return e
	}
	changed, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if changed == 0 {
		return tx.Commit()
	}
	// A terminal callback must not stop a newer authorized launch awaiting its
	// own attempt. Match the memory store's board guard and chronological bound.
	if _, e = tx.ExecContext(ctx, `UPDATE task_launches SET state='stopped',lease_token='',lease_until=NULL,updated_at=NOW() WHERE task_id=$1 AND state IN ('queued','claimed','started') AND created_at <= (SELECT created_at FROM task_attempts WHERE id=$2)`, taskID, attemptID); e != nil {
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
	// A cancelled task may still have a later attempt run; an interrupted attempt
	// must still be surfaced instead of silently leaving the task in an unblocked state.
	if t.Status != TaskStatusDoing && t.Status != TaskStatusBacklog && t.Status != TaskStatusCancelled {
		return nil
	}
	t.UpdatedAt = time.Now().UTC()
	t.Status = TaskStatusBlocked
	for _, l := range s.launches {
		if l.TaskID == taskID && !l.CreatedAt.After(latest.CreatedAt) && (l.State == "queued" || l.State == "claimed" || l.State == "started") {
			l.State = "stopped"
			l.LeaseToken = ""
			l.LeaseUntil = nil
			l.UpdatedAt = t.UpdatedAt
		}
	}
	if latest.Status == AttemptStatusSucceeded {
		required := false
		var last *TaskActivity
		for _, a := range s.activities {
			if a.AttemptID == nil || *a.AttemptID != attemptID {
				continue
			}
			if a.ActivityType == "agent_contract" {
				required = true
			}
			if a.ActivityType == "agent_report" && (last == nil || a.CreatedAt.After(last.CreatedAt)) {
				last = a
			}
		}
		if !required || (last != nil && (last.NewStatus == "review" || last.NewStatus == "done") && t.Summary != "" && t.CompleteNote != "") {
			t.Status = TaskStatusReview
			if last != nil && last.NewStatus == "done" && t.SourceRoomID == "" {
				t.Status = TaskStatusDone
			}
		}
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

func (s *Service) GetAttempt(ctx context.Context, attemptID string) (*TaskAttempt, error) {
	return s.store.GetAttempt(ctx, attemptID)
}

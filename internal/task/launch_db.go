package task

import (
	"context"
	"database/sql"
	"time"
)

const launchColumns = `id,task_id,request_id,owner_id,room_id::text,source_request_id,agent_id,revision,instructions_hash,state,available_at,expires_at,lease_token,lease_until,checks,safe_error,created_at,updated_at,project_id`

type launchScanner interface{ Scan(...any) error }

func scanLaunch(row launchScanner) (*TaskLaunch, error) {
	var j TaskLaunch
	e := row.Scan(&j.ID, &j.TaskID, &j.RequestID, &j.OwnerID, &j.RoomID, &j.SourceRequestID, &j.AgentID, &j.Revision, &j.InstructionsHash, &j.State, &j.AvailableAt, &j.ExpiresAt, &j.LeaseToken, &j.LeaseUntil, &j.Checks, &j.SafeError, &j.CreatedAt, &j.UpdatedAt, &j.ProjectID)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &j, nil
}
func (s *DBStore) SaveLaunch(ctx context.Context, j TaskLaunch) (*TaskLaunch, error) {
	j, e := normalizeLaunch(j)
	if e != nil {
		return nil, e
	}
	_, e = s.db.ExecContext(ctx, `INSERT INTO task_launches(id,task_id,request_id,attempt_id,owner_id,room_id,source_request_id,agent_id,revision,instructions_hash,available_at,expires_at,created_at,updated_at,project_id) VALUES($1,$2,$3,$13,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12,$14) ON CONFLICT(id) DO NOTHING`, j.ID, j.TaskID, j.RequestID, j.OwnerID, j.RoomID, j.SourceRequestID, j.AgentID, j.Revision, j.InstructionsHash, j.AvailableAt, j.ExpiresAt, j.CreatedAt, attemptRequestID(j.TaskID, j.RequestID), j.ProjectID)
	if e != nil {
		return nil, e
	}
	old, e := s.GetLaunch(ctx, j.ID)
	if e != nil {
		return nil, e
	}
	if !sameLaunch(*old, j) {
		return nil, ErrRequestConflict
	}
	return old, nil
}
func (s *DBStore) GetLaunch(ctx context.Context, id string) (*TaskLaunch, error) {
	j, e := scanLaunch(s.db.QueryRowContext(ctx, `SELECT `+launchColumns+` FROM task_launches WHERE id=$1`, id))
	if e == nil && j == nil {
		return nil, ErrTaskNotFound
	}
	return j, e
}
func (s *DBStore) ClaimLaunch(ctx context.Context, now time.Time, token string, wanted ...string) (*TaskLaunch, error) {
	if token == "" {
		return nil, ErrInvalidTaskInput
	}
	id := ""
	if len(wanted) > 0 {
		id = wanted[0]
	}
	// For still-queued tasks, keep their launch window active so waiting work is not dropped prematurely.
	if _, e := s.db.ExecContext(ctx, `UPDATE task_launches SET expires_at = $1 + INTERVAL '24 hours', updated_at = $1 WHERE state = 'queued' AND expires_at <= $1 AND task_id IN (SELECT id FROM tasks WHERE status = 'queued')`, now); e != nil {
		return nil, e
	}
	// Expiry is terminal even after a lost worker; a later save cannot resurrect it.
	if _, e := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='The launch window expired before execution could start.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('queued','claimed') AND expires_at<=$1`, now); e != nil {
		return nil, e
	}
	// Reconcile orphan launches whose task is already completed or inactive (unless explicitly wanted for drain).
	if _, e := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Task is in terminal status.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('queued','claimed','started') AND ($2 = '' OR id != $2) AND task_id IN (SELECT id FROM tasks WHERE status IN ('done','cancelled','failed'))`, now, id); e != nil {
		return nil, e
	}
	// Reconcile zombie started launches without any active running attempt.
	zombieCutoff := now.Add(-2 * time.Minute)
	if _, e := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='No active runner process detected.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state = 'started' AND updated_at < $2 AND NOT EXISTS (SELECT 1 FROM task_attempts a WHERE a.task_id = task_launches.task_id AND a.status = 'running')`, now, zombieCutoff); e != nil {
		return nil, e
	}
	return scanLaunch(s.db.QueryRowContext(ctx, `WITH candidate AS (
		SELECT id FROM task_launches tl
		WHERE ($3='' OR tl.id=$3)
		  AND tl.expires_at>$1
		  AND ((tl.state='queued' AND tl.available_at<=$1) OR (tl.state='claimed' AND tl.lease_until<=$1))
		  AND NOT EXISTS (
			  SELECT 1 FROM task_launches active_tl
			  LEFT JOIN tasks active_task ON active_task.id = active_tl.task_id
			  WHERE active_tl.project_id = tl.project_id
			    AND active_tl.id != tl.id
			    AND (active_task.id IS NULL OR active_task.status NOT IN ('done', 'cancelled', 'failed', 'blocked', 'review', 'queued'))
			    AND (
				    active_tl.state = 'started'
				    OR (active_tl.state = 'claimed' AND active_tl.lease_until > $1)
			    )
		  )
		  AND NOT EXISTS (
			  SELECT 1 FROM tasks active_t
			  WHERE (active_t.project_id = tl.project_id OR EXISTS (SELECT 1 FROM task_projects tp WHERE tp.task_id = active_t.id AND tp.project_id = tl.project_id))
			    AND active_t.id != tl.task_id
			    AND active_t.status IN ('doing', 'working')
		  )
		ORDER BY tl.available_at, tl.id
		FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE task_launches SET state='claimed',lease_token=$2,lease_until=$1+INTERVAL '1 minute',checks=checks+1,updated_at=$1 WHERE id=(SELECT id FROM candidate) RETURNING `+launchColumns, now, token, id))
}
func (s *DBStore) ReconcileOrphanLaunches(ctx context.Context, now time.Time) error {
	// 1. Reconcile launches whose task is in terminal status
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Task is in terminal status.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('queued','claimed','started') AND task_id IN (SELECT id FROM tasks WHERE status IN ('done','cancelled','failed'))`, now); err != nil {
		return err
	}
	// 1b. Self-heal orphan ghost-queued tasks: if a task is queued but its launch was stopped (e.g. during restart, terminal status false alarm, or expired window), revive it back to queued with refreshed window
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='queued',safe_error='',lease_token='',lease_until=NULL,available_at=$1,expires_at=GREATEST(expires_at, $1 + INTERVAL '24 hours'),updated_at=$1 WHERE state='stopped' AND safe_error != 'Task was cancelled.' AND task_id IN (SELECT id FROM tasks WHERE status='queued')`, now); err != nil {
		return err
	}
	// 2. Reconcile zombie started launches without any active running attempt
	zombieCutoff := now.Add(-2 * time.Minute)
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Execution attempt is no longer active (orphan launch reconciled).',lease_token='',lease_until=NULL,updated_at=$1 WHERE state = 'started' AND updated_at < $2 AND NOT EXISTS (SELECT 1 FROM task_attempts a WHERE a.task_id = task_launches.task_id AND a.status = 'running')`, now, zombieCutoff); err != nil {
		return err
	}
	// 2b. Reconcile started/claimed launches that exceeded hard execution timeout (20 minutes)
	timeoutCutoff := now.Add(-20 * time.Minute)
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Execution exceeded hard timeout (20 minutes).',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('started','claimed') AND updated_at < $2`, now, timeoutCutoff); err != nil {
		return err
	}
	// 3. Auto-cancel abandoned interactive terminal tasks queued for > 5 minutes
	staleCutoff := now.Add(-5 * time.Minute)
	if _, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = 'cancelled', updated_at = $1 WHERE status = 'queued' AND title LIKE 'Terminal · %' AND created_at < $2`, now, staleCutoff); err != nil {
		return err
	}
	// 4. Mark launches for cancelled tasks as stopped
	_, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Task was cancelled.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('queued','claimed','started') AND task_id IN (SELECT id FROM tasks WHERE status = 'cancelled')`, now)
	return err
}
func (s *DBStore) ReconcileStartupLaunches(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Execution stopped during server restart.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('started','claimed')`, now)
	return err
}
func (s *DBStore) FinishTaskLaunches(ctx context.Context, taskID string, finalState string) error {
	if !validLaunchFinish(finalState) && finalState != "cancelled" {
		finalState = "stopped"
	}
	_, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state=$2, lease_token='', lease_until=NULL, updated_at=NOW() WHERE task_id=$1 AND state IN ('queued', 'claimed', 'started')`, taskID, finalState)
	return err
}
func (s *DBStore) FinishLaunch(ctx context.Context, id, token, state string, next time.Time, safeError string) error {
	if !validLaunchFinish(state) {
		return ErrInvalidTaskInput
	}
	result, e := s.db.ExecContext(ctx, `UPDATE task_launches SET state=$3,available_at=$4,safe_error=$5,lease_token='',lease_until=NULL,updated_at=NOW() WHERE id=$1 AND state='claimed' AND lease_token=$2 AND lease_until>NOW()`, id, token, state, next, safeError)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrLaunchLease
	}
	return nil
}
func (s *DBStore) CancelLaunches(ctx context.Context, taskID string, now time.Time) (int64, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "task:"+taskID); e != nil {
		return 0, e
	}
	result, e := tx.ExecContext(ctx, `UPDATE task_launches SET state='cancelled',lease_token='',lease_until=NULL,updated_at=$2 WHERE task_id=$1 AND state IN ('queued','claimed')`, taskID, now)
	if e != nil {
		return 0, e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return 0, e
	}
	return n, tx.Commit()
}

func (s *DBStore) LatestLaunch(ctx context.Context, taskID string) (*TaskLaunch, error) {
	return scanLaunch(s.db.QueryRowContext(ctx, `SELECT `+launchColumns+` FROM task_launches WHERE task_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1`, taskID))
}

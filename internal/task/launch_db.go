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
	if _, e := s.db.ExecContext(ctx, `UPDATE task_launches SET expires_at = $1::timestamptz + INTERVAL '24 hours', updated_at = $1 WHERE state = 'queued' AND expires_at <= $1 AND task_id IN (SELECT id FROM tasks WHERE status = 'queued')`, now); e != nil {
		return nil, e
	}
	// Self-heal orphan ghost-queued tasks: if a task is queued but its launch was stopped (e.g. during restart or expired window), revive it back to queued with refreshed window
	if _, e := s.db.ExecContext(ctx, `UPDATE task_launches SET state='queued',safe_error='',lease_token='',lease_until=NULL,available_at=$1,expires_at=GREATEST(expires_at, $1::timestamptz + INTERVAL '24 hours'),updated_at=$1 WHERE state='stopped' AND safe_error IN ('Task is in terminal status.', 'Execution stopped during server restart.') AND task_id IN (SELECT id FROM tasks WHERE status='queued')`, now); e != nil {
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
	if _, e := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='No active runner process detected.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state = 'started' AND updated_at < $2 AND NOT EXISTS (SELECT 1 FROM task_attempts a WHERE a.id = task_launches.attempt_id AND a.task_id = task_launches.task_id AND a.status IN ('queued','running'))`, now, zombieCutoff); e != nil {
		return nil, e
	}
	return scanLaunch(s.db.QueryRowContext(ctx, `WITH candidate AS (
		SELECT tl.id FROM task_launches tl
		JOIN tasks cand_task ON cand_task.id = tl.task_id
		WHERE ($3='' OR tl.id=$3)
		  AND tl.expires_at>$1
		  AND ((tl.state='queued' AND tl.available_at<=$1) OR (tl.state='claimed' AND tl.lease_until<=$1))
		  AND cand_task.status NOT IN ('done', 'cancelled', 'failed', 'blocked')
 AND NOT (`+WorkflowHandoffBlocksSQL("cand_task")+`)
		  -- 1. Migration barrier: Active migration blocks all candidate tasks
		  AND NOT EXISTS (
			  SELECT 1 FROM tasks mig_t
			  WHERE (mig_t.project_id = tl.project_id OR EXISTS (SELECT 1 FROM task_projects tp WHERE tp.task_id = mig_t.id AND tp.project_id = tl.project_id))
			    AND mig_t.id != tl.task_id
			    AND mig_t.status NOT IN ('done', 'cancelled', 'failed', 'blocked', 'review', 'queued')
			    AND (
				    mig_t.status IN ('doing', 'working')
				    OR EXISTS (SELECT 1 FROM task_attempts a WHERE a.task_id = mig_t.id AND a.status = 'running')
				    OR EXISTS (SELECT 1 FROM task_launches l WHERE l.task_id = mig_t.id AND l.state = 'started')
			    )
			    AND COALESCE(mig_t.execution_category, 'code') = 'code'
			    AND (
				    mig_t.instructions ILIKE '%migration%' OR mig_t.instructions ILIKE '%migrate%' OR
				    mig_t.title ILIKE '%migration%' OR mig_t.title ILIKE '%migrate%'
			    )
		  )
		  -- 2. Concurrency isolation per execution lane:
		  -- Readonly lane tasks are non-blocking and can run concurrently with other tasks (unless migration active)
		  -- Ops and Code lanes only block tasks in their own respective lane
		  AND (
			  COALESCE(cand_task.execution_category, 'code') = 'readonly'
			  OR NOT EXISTS (
				  SELECT 1 FROM task_launches active_tl
				  JOIN tasks active_task ON active_task.id = active_tl.task_id
				  WHERE active_tl.project_id = tl.project_id
				    AND active_tl.id != tl.id
				    AND active_task.status NOT IN ('done', 'cancelled', 'failed', 'blocked', 'review', 'queued')
				    AND (
					    active_tl.state = 'started'
					    OR (active_tl.state = 'claimed' AND active_tl.lease_until > $1)
				    )
				    AND COALESCE(active_task.execution_category, 'code') = COALESCE(cand_task.execution_category, 'code')
			  )
		  )
		  AND (
			  COALESCE(cand_task.execution_category, 'code') = 'readonly'
			  OR NOT EXISTS (
				  SELECT 1 FROM tasks active_t
				  WHERE (active_t.project_id = tl.project_id OR EXISTS (SELECT 1 FROM task_projects tp WHERE tp.task_id = active_t.id AND tp.project_id = tl.project_id))
				    AND active_t.id != tl.task_id
				    AND active_t.status NOT IN ('done', 'cancelled', 'failed', 'blocked', 'review', 'queued')
				    AND (
					    active_t.status IN ('doing', 'working')
					    OR EXISTS (SELECT 1 FROM task_attempts a WHERE a.task_id = active_t.id AND a.status = 'running')
				    )
				    AND COALESCE(active_t.execution_category, 'code') = COALESCE(cand_task.execution_category, 'code')
			  )
		  )
		ORDER BY tl.available_at, tl.id
		FOR UPDATE SKIP LOCKED LIMIT 1
	) UPDATE task_launches SET state='claimed',lease_token=$2,lease_until=$1::timestamptz+INTERVAL '1 minute',checks=checks+1,updated_at=$1 WHERE id=(SELECT id FROM candidate) RETURNING `+launchColumns, now, token, id))
}
func (s *DBStore) ReconcileOrphanLaunches(ctx context.Context, now time.Time) error {
	// A launch response can be lost/deferred after reserving its exact attempt.
	// Once that attempt is terminal, keeping the launch queued suppresses result
	// review and authorized recovery forever. Close only that original intent;
	// never change its task/attempt or consume a newer continuation.
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches l SET state='stopped',safe_error='Recorded execution is terminal; launch reconciled.',lease_token='',lease_until=NULL,updated_at=$1 WHERE l.state IN ('queued','claimed','started') AND EXISTS (SELECT 1 FROM task_attempts a WHERE a.id=l.attempt_id AND a.task_id=l.task_id AND a.status IN ('succeeded','failed','canceled','interrupted'))`, now); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `WITH candidate AS (
 SELECT t.id,l.id AS launch_id,a.id AS attempt_id FROM tasks t
 JOIN task_launches l ON l.task_id=t.id
 JOIN task_attempts a ON a.task_id=t.id
 WHERE t.status='blocked' AND l.state='queued' AND l.expires_at>$1
 AND COALESCE(t.project_id,'')=l.project_id AND l.instructions_hash=encode(digest(t.instructions,'sha256'),'hex')
 AND l.id=(SELECT id FROM task_launches WHERE task_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1)
 AND a.id=(SELECT id FROM task_attempts WHERE task_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1)
 AND a.id<>l.attempt_id AND a.status='failed' AND a.exit_code=75 AND a.finished_at IS NOT NULL
 AND NOT COALESCE(a.cancel_requested,false) AND a.canceled_at IS NULL
 AND a.error_message LIKE 'The AI provider reported a usage limit. The run was stopped;%'
 AND NOT EXISTS(SELECT 1 FROM task_attempts live WHERE live.task_id=t.id AND live.status IN ('queued','running'))
 AND NOT EXISTS(SELECT 1 FROM task_activities pause WHERE pause.task_id=t.id AND pause.activity_type='status_change' AND pause.new_status='blocked' AND pause.created_at>a.finished_at)
 FOR UPDATE OF t,l SKIP LOCKED
 ), resumed AS (
 UPDATE tasks t SET status='queued',updated_at=$1 FROM candidate c WHERE t.id=c.id AND t.status='blocked' RETURNING t.id,c.launch_id,c.attempt_id
 ) INSERT INTO task_activities(id,task_id,attempt_id,activity_type,author,new_status,message,created_at)
 SELECT 'act_capacity_'||substr(encode(digest(id||':'||launch_id||':'||attempt_id,'sha256'),'hex'),1,32),id,attempt_id,'execution_recovery','coordinator','queued','Restored the existing approved pending launch after an earlier capacity failure. Execution history and recovery budgets are unchanged.',$1 FROM resumed ON CONFLICT DO NOTHING`, now); err != nil {
		return err
	}

	// 1. Reconcile launches whose task is in terminal status
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Task is in terminal status.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('queued','claimed','started') AND task_id IN (SELECT id FROM tasks WHERE status IN ('done','cancelled','failed'))`, now); err != nil {
		return err
	}
	// 1b. Self-heal orphan ghost-queued tasks: if a task is queued but its launch was stopped (e.g. during restart, terminal status false alarm, or expired window), revive it back to queued with refreshed window
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='queued',safe_error='',lease_token='',lease_until=NULL,available_at=$1,expires_at=GREATEST(expires_at, $1::timestamptz + INTERVAL '24 hours'),updated_at=$1 WHERE state='stopped' AND safe_error IN ('Task is in terminal status.', 'Execution stopped during server restart.') AND task_id IN (SELECT id FROM tasks WHERE status='queued')`, now); err != nil {
		return err
	}
	// 2. Reconcile zombie started launches without any active running attempt
	zombieCutoff := now.Add(-2 * time.Minute)
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Execution attempt is no longer active (orphan launch reconciled).',lease_token='',lease_until=NULL,updated_at=$1 WHERE state = 'started' AND updated_at < $2 AND NOT EXISTS (SELECT 1 FROM task_attempts a WHERE a.id = task_launches.attempt_id AND a.task_id = task_launches.task_id AND a.status IN ('queued','running'))`, now, zombieCutoff); err != nil {
		return err
	}
	// 3. Auto-cancel abandoned interactive terminal tasks queued or blocked for > 5 minutes
	staleCutoff := now.Add(-5 * time.Minute)
	if _, err := s.db.ExecContext(ctx, `UPDATE tasks SET status = 'cancelled', updated_at = $1 WHERE status IN ('queued', 'blocked', 'backlog') AND title LIKE 'Terminal · %' AND (created_at < $2 OR updated_at < $2)`, now, staleCutoff); err != nil {
		return err
	}
	// 4. Mark launches for cancelled tasks as stopped
	_, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Task was cancelled.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('queued','claimed','started') AND task_id IN (SELECT id FROM tasks WHERE status = 'cancelled')`, now)
	return err
}
func (s *DBStore) ReconcileStartupLaunches(ctx context.Context, now time.Time) error {
	// A recorded detached execution survives API restart. Its admission receipt
	// must not be requeued or reported as a failed execution.
	const activeBound = `EXISTS (SELECT 1 FROM task_attempts a JOIN tasks t ON t.id=a.task_id WHERE a.id=task_launches.attempt_id AND a.task_id=task_launches.task_id AND t.status NOT IN ('done','cancelled','canceled','failed') AND a.status IN ('queued','running') AND NOT COALESCE(a.cancel_requested,false) AND a.canceled_at IS NULL)`
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='queued',safe_error='',lease_token='',lease_until=NULL,available_at=$1,expires_at=GREATEST(expires_at, $1::timestamptz + INTERVAL '24 hours'),updated_at=$1 WHERE (state IN ('started','claimed') OR (state='stopped' AND safe_error IN ('Task is in terminal status.', 'Execution stopped during server restart.'))) AND task_id IN (SELECT id FROM tasks WHERE status = 'queued') AND NOT `+activeBound, now); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='Execution stopped during server restart.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('started','claimed') AND NOT `+activeBound, now); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE task_launches SET state='started',safe_error='',lease_token='',lease_until=NULL,updated_at=$1 WHERE (state IN ('queued','claimed','started') OR (state='stopped' AND safe_error IN ('Execution stopped during server restart.','Execution attempt is no longer active (orphan launch reconciled).','No active runner process detected.'))) AND task_id IN (SELECT id FROM tasks WHERE status NOT IN ('done','cancelled','canceled','failed')) AND `+activeBound, now)
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
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var taskID string
	if e = tx.QueryRowContext(ctx, `SELECT task_id FROM task_launches WHERE id=$1`, id).Scan(&taskID); e != nil {
		return e
	}
	// Serialize with attempt reservation and cancellation before changing either
	// launch or board state. A stopped old launch must not block newer work.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "task:"+taskID); e != nil {
		return e
	}
	result, e := tx.ExecContext(ctx, `UPDATE task_launches SET state=$3,available_at=$4,safe_error=$5,lease_token='',lease_until=NULL,updated_at=NOW() WHERE id=$1 AND state='claimed' AND lease_token=$2 AND lease_until>NOW()`, id, token, state, next, safeError)
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
	if state == "stopped" {
		_, e = tx.ExecContext(ctx, `UPDATE tasks SET status='blocked',updated_at=NOW() WHERE id=$1 AND status='queued'
		AND $2=(SELECT id FROM task_launches WHERE task_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1)
		AND NOT EXISTS (SELECT 1 FROM task_attempts WHERE task_id=$1 AND status IN ('queued','running'))`, taskID, id)
		if e != nil {
			return e
		}
	}
	return tx.Commit()
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

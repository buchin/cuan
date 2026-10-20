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
	// Expiry is terminal even after a lost worker; a later save cannot resurrect it.
	if _, e := s.db.ExecContext(ctx, `UPDATE task_launches SET state='stopped',safe_error='The launch window expired before execution could start.',lease_token='',lease_until=NULL,updated_at=$1 WHERE state IN ('queued','claimed') AND expires_at<=$1`, now); e != nil {
		return nil, e
	}
	return scanLaunch(s.db.QueryRowContext(ctx, `WITH candidate AS (SELECT id FROM task_launches WHERE ($3='' OR id=$3) AND expires_at>$1 AND ((state='queued' AND available_at<=$1) OR (state='claimed' AND lease_until<=$1)) ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE task_launches SET state='claimed',lease_token=$2,lease_until=$1+INTERVAL '1 minute',checks=checks+1,updated_at=$1 WHERE id=(SELECT id FROM candidate) RETURNING `+launchColumns, now, token, id))
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

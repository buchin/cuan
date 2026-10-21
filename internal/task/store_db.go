package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type DBStore struct {
	db *sql.DB
}

func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) CreateTask(ctx context.Context, t *Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.CreatedAt = t.CreatedAt.Truncate(time.Microsecond)
	t.UpdatedAt = t.CreatedAt

	query := `
		INSERT INTO tasks (
			id, project_id, title, instructions, status, priority, assigned_agent,
			branch_name, pr_url, pr_number, commit_sha, diff_summary, test_summary,
			created_at, updated_at,source_room_id,source_message_id,summary,complete_note,dod_checklist,creation_hash
		) VALUES (
			$1, NULLIF($2,''), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,NULLIF($16,'')::uuid,NULLIF($17,'')::uuid,$18,$19,$20::jsonb,$21
		)
	`
	_, err = tx.ExecContext(ctx, query,
		t.ID,
		t.ProjectID,
		t.Title,
		t.Instructions,
		t.Status,
		t.Priority,
		t.AssignedAgent,
		t.BranchName,
		t.PRURL,
		t.PRNumber,
		t.CommitSHA,
		t.DiffSummary,
		t.TestSummary,
		t.CreatedAt,
		t.UpdatedAt, t.SourceRoomID, t.SourceMessageID, t.Summary, t.CompleteNote, checklistJSON(t.DoDChecklist), t.CreationHash,
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	// Synchronize task_projects relation
	pids := t.ProjectIDs
	if len(pids) == 0 && t.ProjectID != "" {
		pids = []string{t.ProjectID}
	}
	for _, pid := range pids {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO task_projects (task_id, project_id, created_at)
			VALUES ($1, $2, $3)
			ON CONFLICT (task_id, project_id) DO NOTHING
		`, t.ID, pid, t.CreatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *DBStore) GetTask(ctx context.Context, id string) (*Task, error) {
	query := `
		SELECT 
			id, COALESCE(project_id,''), title, instructions, status, priority,
			COALESCE(assigned_agent, 'agy'),
			COALESCE(branch_name, ''),
			COALESCE(pr_url, ''),
			COALESCE(pr_number, 0),
			COALESCE(commit_sha, ''),
			COALESCE(diff_summary, ''),
			COALESCE(test_summary, ''),
			created_at, updated_at,COALESCE(source_room_id::text,''),COALESCE(source_message_id::text,''),summary,complete_note,dod_checklist,creation_hash
		FROM tasks
		WHERE id = $1
	`
	var t Task
	var checklist []byte
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID,
		&t.ProjectID,
		&t.Title,
		&t.Instructions,
		&t.Status,
		&t.Priority,
		&t.AssignedAgent,
		&t.BranchName,
		&t.PRURL,
		&t.PRNumber,
		&t.CommitSHA,
		&t.DiffSummary,
		&t.TestSummary,
		&t.CreatedAt,
		&t.UpdatedAt, &t.SourceRoomID, &t.SourceMessageID, &t.Summary, &t.CompleteNote, &checklist, &t.CreationHash,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	if err := json.Unmarshal(checklist, &t.DoDChecklist); err != nil {
		return nil, fmt.Errorf("read acceptance criteria: %w", err)
	}

	// Populate linked project IDs from task_projects
	rows, qErr := s.db.QueryContext(ctx, `SELECT project_id FROM task_projects WHERE task_id = $1 ORDER BY created_at ASC`, id)
	if qErr == nil {
		defer rows.Close()
		var pids []string
		for rows.Next() {
			var pid string
			if err := rows.Scan(&pid); err == nil {
				pids = append(pids, pid)
			}
		}
		t.ProjectIDs = pids
	}
	if len(t.ProjectIDs) == 0 && t.ProjectID != "" {
		t.ProjectIDs = []string{t.ProjectID}
	}

	return &t, nil
}

func (s *DBStore) ListTasks(ctx context.Context, filter TaskFilter) ([]*Task, error) {
	query := `
		SELECT 
			id, COALESCE(project_id,''), title, instructions, status, priority,
			COALESCE(assigned_agent, 'agy'),
			COALESCE(branch_name, ''),
			COALESCE(pr_url, ''),
			COALESCE(pr_number, 0),
			COALESCE(commit_sha, ''),
			COALESCE(diff_summary, ''),
			COALESCE(test_summary, ''),
			created_at, updated_at,COALESCE(source_room_id::text,''),COALESCE(source_message_id::text,''),summary,complete_note,dod_checklist,creation_hash
		FROM tasks
		WHERE ($1 = '' OR project_id = $1 OR EXISTS (SELECT 1 FROM task_projects tp WHERE tp.task_id = tasks.id AND tp.project_id = $1))
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR LOWER(title) LIKE '%' || LOWER($3) || '%' OR LOWER(instructions) LIKE '%' || LOWER($3) || '%')
		AND ($4='' OR source_room_id::text=$4)
 ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, filter.ProjectID, filter.Status, filter.Search, filter.SourceRoomID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var result []*Task
	taskMap := make(map[string]*Task)
	for rows.Next() {
		var t Task
		var checklist []byte
		if err := rows.Scan(
			&t.ID,
			&t.ProjectID,
			&t.Title,
			&t.Instructions,
			&t.Status,
			&t.Priority,
			&t.AssignedAgent,
			&t.BranchName,
			&t.PRURL,
			&t.PRNumber,
			&t.CommitSHA,
			&t.DiffSummary,
			&t.TestSummary,
			&t.CreatedAt,
			&t.UpdatedAt, &t.SourceRoomID, &t.SourceMessageID, &t.Summary, &t.CompleteNote, &checklist, &t.CreationHash,
		); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if err := json.Unmarshal(checklist, &t.DoDChecklist); err != nil {
			return nil, fmt.Errorf("read acceptance criteria: %w", err)
		}
		if t.ProjectID != "" {
			t.ProjectIDs = []string{t.ProjectID}
		} else {
			t.ProjectIDs = []string{}
		}
		result = append(result, &t)
		taskMap[t.ID] = &t
	}

	if len(result) > 0 {
		tpRows, tpErr := s.db.QueryContext(ctx, `SELECT task_id, project_id FROM task_projects ORDER BY created_at ASC`)
		if tpErr == nil {
			defer tpRows.Close()
			pidsMap := make(map[string][]string)
			for tpRows.Next() {
				var tid, pid string
				if err := tpRows.Scan(&tid, &pid); err == nil {
					pidsMap[tid] = append(pidsMap[tid], pid)
				}
			}
			for tid, pids := range pidsMap {
				if t, ok := taskMap[tid]; ok {
					t.ProjectIDs = pids
				}
			}
		}
	}

	// Apply filter.ProjectIDs (multi-project filtering)
	if len(filter.ProjectIDs) > 0 {
		var multiFiltered []*Task
		reqSet := make(map[string]bool)
		for _, pid := range filter.ProjectIDs {
			reqSet[pid] = true
		}
		for _, t := range result {
			match := false
			if reqSet[t.ProjectID] {
				match = true
			} else {
				for _, pid := range t.ProjectIDs {
					if reqSet[pid] {
						match = true
						break
					}
				}
			}
			if match {
				multiFiltered = append(multiFiltered, t)
			}
		}
		return multiFiltered, nil
	}

	return result, nil
}

func (s *DBStore) UpdateTask(ctx context.Context, t *Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	t.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	query := `
		UPDATE tasks SET
			title = $2,
			instructions = $3,
			status = $4,
			priority = $5,
			assigned_agent = $6,
			branch_name = $7,
			pr_url = $8,
			pr_number = $9,
			commit_sha = $10,
			diff_summary = $11,
			test_summary = $12,
			updated_at = $13,summary=$14,complete_note=$15,dod_checklist=$16::jsonb,project_id=NULLIF($17,'')
		WHERE id = $1
	`
	res, err := tx.ExecContext(ctx, query,
		t.ID,
		t.Title,
		t.Instructions,
		t.Status,
		t.Priority,
		t.AssignedAgent,
		t.BranchName,
		t.PRURL,
		t.PRNumber,
		t.CommitSHA,
		t.DiffSummary,
		t.TestSummary,
		t.UpdatedAt, t.Summary, t.CompleteNote, checklistJSON(t.DoDChecklist), t.ProjectID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task rows affected: %w", err)
	}
	if rows == 0 {
		return ErrTaskNotFound
	}

	// Synchronize task_projects if ProjectIDs provided
	if t.ProjectIDs != nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM task_projects WHERE task_id = $1`, t.ID)
		if err != nil {
			return err
		}
		for _, pid := range t.ProjectIDs {
			pid = strings.TrimSpace(pid)
			if pid == "" {
				continue
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO task_projects (task_id, project_id, created_at)
				VALUES ($1, $2, $3)
				ON CONFLICT (task_id, project_id) DO NOTHING
			`, t.ID, pid, time.Now().UTC())
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *DBStore) DeleteTask(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "task:"+id); err != nil {
		return err
	}
	var active bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM task_attempts WHERE task_id=$1 AND status IN ('queued','running'))`, id).Scan(&active); err != nil {
		return err
	}
	if active {
		return ErrTaskActive
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTaskNotFound
	}
	return tx.Commit()
}

func (s *DBStore) CreateAttempt(ctx context.Context, a *TaskAttempt) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.AttemptNumber <= 0 {
		a.AttemptNumber = 1
	}
	var tx *sql.Tx
	var execer interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	} = s.db
	if a.RunnerID == "local_terminal" {
		var err error
		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "task:"+a.TaskID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `SELECT id FROM tasks WHERE id=$1 FOR UPDATE`, a.TaskID); err != nil {
			return err
		}
		var withdrawn bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM task_launches j JOIN tasks t ON t.id=j.task_id WHERE j.attempt_id=$1 AND (j.state<>'claimed' OR j.lease_until IS NULL OR j.lease_until<=NOW() OR j.project_id<>COALESCE(t.project_id,'') OR j.agent_id<>$2 OR j.instructions_hash<>encode(digest(t.instructions,'sha256'),'hex')))`, a.ID, a.AgentName).Scan(&withdrawn); err != nil {
			return err
		}
		if withdrawn {
			return ErrInvalidTaskInput
		}
		var count, maxAttempt int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE status IN ('running','queued')),COALESCE(MAX(attempt_number),0) FROM task_attempts WHERE task_id=$1`, a.TaskID).Scan(&count, &maxAttempt); err != nil {
			return err
		}
		if count > 0 {
			return ErrTaskActive
		}
		a.AttemptNumber = maxAttempt + 1
		execer = tx
	}
	query := `
		INSERT INTO task_attempts (
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes,
			lease_expires_at, cancel_requested, canceled_at, interrupted_at, attempt_number,
			started_at, finished_at, error_message, created_at,request_hash
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,$17
		)
	`
	_, err := execer.ExecContext(ctx, query,
		a.ID,
		a.TaskID,
		a.RunnerID,
		a.AgentName,
		a.Status,
		a.ExitCode,
		a.PeakRSSBytes,
		a.LeaseExpiresAt,
		a.CancelRequested,
		a.CanceledAt,
		a.InterruptedAt,
		a.AttemptNumber,
		a.StartedAt,
		a.FinishedAt,
		a.ErrorMessage,
		a.CreatedAt, a.RequestHash,
	)
	if err != nil {
		return fmt.Errorf("create attempt: %w", err)
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

func (s *DBStore) GetAttempt(ctx context.Context, id string) (*TaskAttempt, error) {
	query := `
		SELECT 
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes,
			lease_expires_at, COALESCE(cancel_requested, false), canceled_at, interrupted_at,
			COALESCE(attempt_number, 1), started_at, finished_at,
			COALESCE(error_message, ''), created_at,request_hash
		FROM task_attempts
		WHERE id = $1
	`
	var a TaskAttempt
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID,
		&a.TaskID,
		&a.RunnerID,
		&a.AgentName,
		&a.Status,
		&a.ExitCode,
		&a.PeakRSSBytes,
		&a.LeaseExpiresAt,
		&a.CancelRequested,
		&a.CanceledAt,
		&a.InterruptedAt,
		&a.AttemptNumber,
		&a.StartedAt,
		&a.FinishedAt,
		&a.ErrorMessage,
		&a.CreatedAt, &a.RequestHash,
	)
	if err == sql.ErrNoRows {
		return nil, ErrAttemptNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get attempt: %w", err)
	}
	return &a, nil
}

func (s *DBStore) GetLatestAttempt(ctx context.Context, taskID string) (*TaskAttempt, error) {
	query := `
		SELECT 
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes,
			lease_expires_at, COALESCE(cancel_requested, false), canceled_at, interrupted_at,
			COALESCE(attempt_number, 1), started_at, finished_at,
			COALESCE(error_message, ''), created_at,request_hash
		FROM task_attempts
		WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var a TaskAttempt
	err := s.db.QueryRowContext(ctx, query, taskID).Scan(
		&a.ID,
		&a.TaskID,
		&a.RunnerID,
		&a.AgentName,
		&a.Status,
		&a.ExitCode,
		&a.PeakRSSBytes,
		&a.LeaseExpiresAt,
		&a.CancelRequested,
		&a.CanceledAt,
		&a.InterruptedAt,
		&a.AttemptNumber,
		&a.StartedAt,
		&a.FinishedAt,
		&a.ErrorMessage,
		&a.CreatedAt, &a.RequestHash,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest attempt: %w", err)
	}
	return &a, nil
}

func (s *DBStore) ListAttempts(ctx context.Context, taskID string) ([]*TaskAttempt, error) {
	query := `
		SELECT 
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes,
			lease_expires_at, COALESCE(cancel_requested, false), canceled_at, interrupted_at,
			COALESCE(attempt_number, 1), started_at, finished_at,
			COALESCE(error_message, ''), created_at,request_hash
		FROM task_attempts
		WHERE task_id = $1
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}
	defer rows.Close()

	var result []*TaskAttempt
	for rows.Next() {
		var a TaskAttempt
		if err := rows.Scan(
			&a.ID,
			&a.TaskID,
			&a.RunnerID,
			&a.AgentName,
			&a.Status,
			&a.ExitCode,
			&a.PeakRSSBytes,
			&a.LeaseExpiresAt,
			&a.CancelRequested,
			&a.CanceledAt,
			&a.InterruptedAt,
			&a.AttemptNumber,
			&a.StartedAt,
			&a.FinishedAt,
			&a.ErrorMessage,
			&a.CreatedAt, &a.RequestHash,
		); err != nil {
			return nil, fmt.Errorf("scan attempt: %w", err)
		}
		result = append(result, &a)
	}

	return result, nil
}

func (s *DBStore) UpdateAttempt(ctx context.Context, a *TaskAttempt) error {
	query := `
		UPDATE task_attempts SET
			status = $2,
			exit_code = $3,
			peak_rss_bytes = $4,
			lease_expires_at = $5,
			cancel_requested = $6,
			canceled_at = $7,
			interrupted_at = $8,
			attempt_number = $9,
			started_at = $10,
			finished_at = $11,
			error_message = $12
		WHERE id = $1 AND (status IN ('queued','running') OR status=$2)
	`
	res, err := s.db.ExecContext(ctx, query,
		a.ID,
		a.Status,
		a.ExitCode,
		a.PeakRSSBytes,
		a.LeaseExpiresAt,
		a.CancelRequested,
		a.CanceledAt,
		a.InterruptedAt,
		a.AttemptNumber,
		a.StartedAt,
		a.FinishedAt,
		a.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("update attempt: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update attempt rows affected: %w", err)
	}
	if rows == 0 {
		if _, e := s.GetAttempt(ctx, a.ID); e == nil {
			return ErrAttemptFinal
		}
		return ErrAttemptNotFound
	}
	return nil
}

func (s *DBStore) CreateActivity(ctx context.Context, act *TaskActivity) error {
	if act.CreatedAt.IsZero() {
		act.CreatedAt = time.Now().UTC()
	}
	query := `
		INSERT INTO task_activities (
			id, task_id, attempt_id, activity_type, author, old_status, new_status, message, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	_, err := s.db.ExecContext(ctx, query,
		act.ID,
		act.TaskID,
		act.AttemptID,
		act.ActivityType,
		act.Author,
		act.OldStatus,
		act.NewStatus,
		act.Message,
		act.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create activity: %w", err)
	}
	return nil
}

func (s *DBStore) ListActivities(ctx context.Context, taskID string) ([]*TaskActivity, error) {
	query := `
		SELECT 
			id, task_id, attempt_id, activity_type, author,
			COALESCE(old_status, ''), COALESCE(new_status, ''), message, created_at
		FROM task_activities
		WHERE task_id = $1
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	defer rows.Close()

	var result []*TaskActivity
	for rows.Next() {
		var act TaskActivity
		if err := rows.Scan(
			&act.ID,
			&act.TaskID,
			&act.AttemptID,
			&act.ActivityType,
			&act.Author,
			&act.OldStatus,
			&act.NewStatus,
			&act.Message,
			&act.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		result = append(result, &act)
	}

	return result, nil
}

func (s *DBStore) AppendLogs(ctx context.Context, attemptID string, logs []LogRecord, maxLines int) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for logs: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO agent_logs (attempt_id, stream, line, logged_at) VALUES ($1, $2, $3, $4)")
	if err != nil {
		return fmt.Errorf("prepare log insert: %w", err)
	}
	defer stmt.Close()

	for _, entry := range logs {
		loggedAt := entry.LoggedAt
		if loggedAt.IsZero() {
			loggedAt = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx, attemptID, entry.Stream, boundedLogLine(entry.Line), loggedAt); err != nil {
			return fmt.Errorf("insert log: %w", err)
		}
	}

	if maxLines > 0 {
		rows, err := tx.QueryContext(ctx, "SELECT id, line FROM agent_logs WHERE attempt_id = $1 ORDER BY id DESC LIMIT $2", attemptID, maxLines)
		if err != nil {
			return fmt.Errorf("read log retention window: %w", err)
		}
		var oldest int64
		size := 0
		for rows.Next() {
			var id int64
			var line string
			if err := rows.Scan(&id, &line); err != nil {
				rows.Close()
				return err
			}
			if size+len(line)+1 > maxRetainedLogBytes {
				break
			}
			size += len(line) + 1
			oldest = id
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if oldest != 0 {
			if _, err := tx.ExecContext(ctx, "DELETE FROM agent_logs WHERE attempt_id = $1 AND id < $2", attemptID, oldest); err != nil {
				return fmt.Errorf("trim retained logs: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (s *DBStore) GetLogs(ctx context.Context, attemptID string, afterID int64, limit int) ([]*LogRecord, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}

	query := `
		SELECT id, attempt_id, stream, line, logged_at
		FROM agent_logs
		WHERE attempt_id = $1 AND ($2 = 0 OR id > $2)
		ORDER BY id ASC
		LIMIT $3
	`
	rows, err := s.db.QueryContext(ctx, query, attemptID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("get logs: %w", err)
	}
	defer rows.Close()

	var result []*LogRecord
	for rows.Next() {
		var rec LogRecord
		if err := rows.Scan(&rec.ID, &rec.AttemptID, &rec.Stream, &rec.Line, &rec.LoggedAt); err != nil {
			return nil, fmt.Errorf("scan log: %w", err)
		}
		result = append(result, &rec)
	}

	return result, nil
}

func checklistJSON(items []DoDItem) string {
	if items == nil {
		return "[]"
	}
	data, _ := json.Marshal(items)
	return string(data)
}

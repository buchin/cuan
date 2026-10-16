package task

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type DBStore struct {
	db *sql.DB
}

func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) CreateTask(ctx context.Context, t *Task) error {
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = t.CreatedAt

	query := `
		INSERT INTO tasks (
			id, project_id, title, instructions, status, priority, assigned_agent, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	_, err := s.db.ExecContext(ctx, query,
		t.ID,
		t.ProjectID,
		t.Title,
		t.Instructions,
		t.Status,
		t.Priority,
		t.AssignedAgent,
		t.CreatedAt,
		t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (s *DBStore) GetTask(ctx context.Context, id string) (*Task, error) {
	query := `
		SELECT 
			id, project_id, title, instructions, status, priority,
			COALESCE(assigned_agent, 'agy'), created_at, updated_at
		FROM tasks
		WHERE id = $1
	`
	var t Task
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID,
		&t.ProjectID,
		&t.Title,
		&t.Instructions,
		&t.Status,
		&t.Priority,
		&t.AssignedAgent,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return &t, nil
}

func (s *DBStore) ListTasks(ctx context.Context, filter TaskFilter) ([]*Task, error) {
	query := `
		SELECT 
			id, project_id, title, instructions, status, priority,
			COALESCE(assigned_agent, 'agy'), created_at, updated_at
		FROM tasks
		WHERE ($1 = '' OR project_id = $1)
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR LOWER(title) LIKE '%' || LOWER($3) || '%' OR LOWER(instructions) LIKE '%' || LOWER($3) || '%')
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, filter.ProjectID, filter.Status, filter.Search)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var result []*Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(
			&t.ID,
			&t.ProjectID,
			&t.Title,
			&t.Instructions,
			&t.Status,
			&t.Priority,
			&t.AssignedAgent,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		result = append(result, &t)
	}

	return result, nil
}

func (s *DBStore) UpdateTask(ctx context.Context, t *Task) error {
	t.UpdatedAt = time.Now().UTC()
	query := `
		UPDATE tasks SET
			title = $2,
			instructions = $3,
			status = $4,
			priority = $5,
			assigned_agent = $6,
			updated_at = $7
		WHERE id = $1
	`
	res, err := s.db.ExecContext(ctx, query,
		t.ID,
		t.Title,
		t.Instructions,
		t.Status,
		t.Priority,
		t.AssignedAgent,
		t.UpdatedAt,
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
	return nil
}

func (s *DBStore) DeleteTask(ctx context.Context, id string) error {
	query := "DELETE FROM tasks WHERE id = $1"
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete task rows affected: %w", err)
	}
	if rows == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (s *DBStore) CreateAttempt(ctx context.Context, a *TaskAttempt) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	query := `
		INSERT INTO task_attempts (
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes, started_at, finished_at, error_message, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`
	_, err := s.db.ExecContext(ctx, query,
		a.ID,
		a.TaskID,
		a.RunnerID,
		a.AgentName,
		a.Status,
		a.ExitCode,
		a.PeakRSSBytes,
		a.StartedAt,
		a.FinishedAt,
		a.ErrorMessage,
		a.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create attempt: %w", err)
	}
	return nil
}

func (s *DBStore) GetAttempt(ctx context.Context, id string) (*TaskAttempt, error) {
	query := `
		SELECT 
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes, started_at, finished_at,
			COALESCE(error_message, ''), created_at
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
		&a.StartedAt,
		&a.FinishedAt,
		&a.ErrorMessage,
		&a.CreatedAt,
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
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes, started_at, finished_at,
			COALESCE(error_message, ''), created_at
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
		&a.StartedAt,
		&a.FinishedAt,
		&a.ErrorMessage,
		&a.CreatedAt,
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
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes, started_at, finished_at,
			COALESCE(error_message, ''), created_at
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
			&a.StartedAt,
			&a.FinishedAt,
			&a.ErrorMessage,
			&a.CreatedAt,
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
			started_at = $5,
			finished_at = $6,
			error_message = $7
		WHERE id = $1
	`
	res, err := s.db.ExecContext(ctx, query,
		a.ID,
		a.Status,
		a.ExitCode,
		a.PeakRSSBytes,
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

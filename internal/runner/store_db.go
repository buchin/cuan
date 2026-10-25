package runner

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/task"
)

type DBStore struct {
	db *sql.DB
}

func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) RegisterOrUpdateHeartbeat(ctx context.Context, info *RunnerInfo) error {
	now := time.Now().UTC()
	if info.CreatedAt.IsZero() {
		info.CreatedAt = now
	}
	info.UpdatedAt = now
	info.LastHeartbeatAt = now
	if info.Status == "" {
		info.Status = StatusIdle
	}

	agentsStr := strings.Join(info.SupportedAgents, ",")

	query := `
		INSERT INTO runners (
			id, name, hostname, supported_agents, status, last_heartbeat_at,
			current_task_id, current_attempt_id, cpu_percent, memory_used_bytes,
			memory_total_bytes, disk_used_bytes, disk_total_bytes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (id) DO UPDATE SET
			hostname = EXCLUDED.hostname,
			supported_agents = EXCLUDED.supported_agents,
			status = EXCLUDED.status,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at,
			current_task_id = EXCLUDED.current_task_id,
			current_attempt_id = EXCLUDED.current_attempt_id,
			cpu_percent = EXCLUDED.cpu_percent,
			memory_used_bytes = EXCLUDED.memory_used_bytes,
			memory_total_bytes = EXCLUDED.memory_total_bytes,
			disk_used_bytes = EXCLUDED.disk_used_bytes,
			disk_total_bytes = EXCLUDED.disk_total_bytes,
			updated_at = EXCLUDED.updated_at
	`
	var taskID, attemptID *string
	if info.CurrentTaskID != "" {
		taskID = &info.CurrentTaskID
	}
	if info.CurrentAttemptID != "" {
		attemptID = &info.CurrentAttemptID
	}

	_, err := s.db.ExecContext(ctx, query,
		info.ID,
		info.Name,
		info.Hostname,
		agentsStr,
		info.Status,
		info.LastHeartbeatAt,
		taskID,
		attemptID,
		info.CPUPercent,
		info.MemoryUsedBytes,
		info.MemoryTotalBytes,
		info.DiskUsedBytes,
		info.DiskTotalBytes,
		info.CreatedAt,
		info.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("register/heartbeat runner: %w", err)
	}
	return nil
}

func (s *DBStore) GetRunner(ctx context.Context, id string) (*RunnerInfo, error) {
	query := `
		SELECT 
			id, name, hostname, supported_agents, status, last_heartbeat_at,
			COALESCE(current_task_id, ''), COALESCE(current_attempt_id, ''),
			cpu_percent, memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes,
			created_at, updated_at
		FROM runners
		WHERE id = $1
	`
	var r RunnerInfo
	var agentsStr string
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&r.ID,
		&r.Name,
		&r.Hostname,
		&agentsStr,
		&r.Status,
		&r.LastHeartbeatAt,
		&r.CurrentTaskID,
		&r.CurrentAttemptID,
		&r.CPUPercent,
		&r.MemoryUsedBytes,
		&r.MemoryTotalBytes,
		&r.DiskUsedBytes,
		&r.DiskTotalBytes,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrRunnerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get runner: %w", err)
	}
	if agentsStr != "" {
		r.SupportedAgents = strings.Split(agentsStr, ",")
	}
	return &r, nil
}

func (s *DBStore) ListRunners(ctx context.Context) ([]*RunnerInfo, error) {
	query := `
		SELECT 
			id, name, hostname, supported_agents, status, last_heartbeat_at,
			COALESCE(current_task_id, ''), COALESCE(current_attempt_id, ''),
			cpu_percent, memory_used_bytes, memory_total_bytes, disk_used_bytes, disk_total_bytes,
			created_at, updated_at
		FROM runners
		ORDER BY id ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list runners: %w", err)
	}
	defer rows.Close()

	var list []*RunnerInfo
	now := time.Now().UTC()
	for rows.Next() {
		var r RunnerInfo
		var agentsStr string
		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.Hostname,
			&agentsStr,
			&r.Status,
			&r.LastHeartbeatAt,
			&r.CurrentTaskID,
			&r.CurrentAttemptID,
			&r.CPUPercent,
			&r.MemoryUsedBytes,
			&r.MemoryTotalBytes,
			&r.DiskUsedBytes,
			&r.DiskTotalBytes,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan runner: %w", err)
		}
		if agentsStr != "" {
			r.SupportedAgents = strings.Split(agentsStr, ",")
		}
		if now.Sub(r.LastHeartbeatAt) > 30*time.Second {
			r.Status = StatusOffline
		}
		list = append(list, &r)
	}
	return list, nil
}

func (s *DBStore) ClaimNextQueuedAttempt(ctx context.Context, runnerID string, supportedAgents []string, leaseDuration time.Duration) (*task.TaskAttempt, *task.Task, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, nil, fmt.Errorf("begin claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. CONCURRENCY LIMIT: 1 task aktif per runner
	var activeCount int
	err = tx.QueryRowContext(ctx, "SELECT count(*) FROM task_attempts WHERE runner_id = $1 AND status = 'running'", runnerID).Scan(&activeCount)
	if err != nil {
		return nil, nil, fmt.Errorf("check runner active count: %w", err)
	}
	if activeCount > 0 {
		// Runner is already executing an active task
		return nil, nil, nil
	}

	// 2. DUPLICATE CLAIM PROTECTION: Atomically select next queued attempt using SKIP LOCKED
	query := `
		SELECT 
			a.id, a.task_id, a.runner_id, a.agent_name, a.status, a.exit_code, a.peak_rss_bytes,
			a.lease_expires_at, COALESCE(a.cancel_requested, false), a.canceled_at, a.interrupted_at,
			COALESCE(a.attempt_number, 1), a.started_at, a.finished_at, COALESCE(a.error_message, ''), a.created_at,
			t.id, t.project_id, t.title, t.instructions, t.status, t.priority, COALESCE(t.assigned_agent, 'agy'),
			COALESCE(t.branch_name, ''), COALESCE(t.pr_url, ''), COALESCE(t.pr_number, 0),
			COALESCE(t.commit_sha, ''), COALESCE(t.diff_summary, ''), COALESCE(t.test_summary, ''),
			t.created_at, t.updated_at
		FROM task_attempts a
		JOIN tasks t ON a.task_id = t.id
		WHERE a.status = 'queued'
		  AND (
		      COALESCE(t.project_id, '') = ''
		      OR (
		          NOT EXISTS (
		              SELECT 1 FROM task_attempts active_att
		              JOIN tasks active_t ON active_att.task_id = active_t.id
		              WHERE (active_t.project_id = t.project_id OR EXISTS (SELECT 1 FROM task_projects tp WHERE tp.task_id = active_t.id AND tp.project_id = t.project_id))
		                AND active_att.status = 'running'
		          )
		          AND NOT EXISTS (
		              SELECT 1 FROM tasks active_t
		              WHERE (active_t.project_id = t.project_id OR EXISTS (SELECT 1 FROM task_projects tp WHERE tp.task_id = active_t.id AND tp.project_id = t.project_id))
		                AND active_t.id != t.id
		                AND active_t.status IN ('doing', 'working')
		          )
		          AND NOT EXISTS (
		              SELECT 1 FROM task_launches active_tl
		              JOIN tasks active_t ON active_tl.task_id = active_t.id
		              WHERE active_tl.project_id = t.project_id
		                AND active_tl.task_id != t.id
		                AND active_t.status NOT IN ('done', 'cancelled', 'failed', 'blocked')
		                AND (
		                    active_tl.state = 'started'
		                    OR (active_tl.state = 'claimed' AND active_tl.lease_until > NOW())
		                )
		          )
		      )
		  )
		ORDER BY a.created_at ASC
		LIMIT 1
		FOR UPDATE OF a SKIP LOCKED
	`

	var att task.TaskAttempt
	var t task.Task
	err = tx.QueryRowContext(ctx, query).Scan(
		&att.ID,
		&att.TaskID,
		&att.RunnerID,
		&att.AgentName,
		&att.Status,
		&att.ExitCode,
		&att.PeakRSSBytes,
		&att.LeaseExpiresAt,
		&att.CancelRequested,
		&att.CanceledAt,
		&att.InterruptedAt,
		&att.AttemptNumber,
		&att.StartedAt,
		&att.FinishedAt,
		&att.ErrorMessage,
		&att.CreatedAt,
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
		&t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// No queued attempt available
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query queued attempt: %w", err)
	}

	now := time.Now().UTC()
	leaseExpires := now.Add(leaseDuration)

	// Update task_attempts
	updateAttemptQuery := `
		UPDATE task_attempts SET
			status = 'running',
			runner_id = $2,
			started_at = $3,
			lease_expires_at = $4
		WHERE id = $1
	`
	if _, err := tx.ExecContext(ctx, updateAttemptQuery, att.ID, runnerID, now, leaseExpires); err != nil {
		return nil, nil, fmt.Errorf("update attempt to running: %w", err)
	}
	att.Status = task.AttemptStatusRunning
	att.RunnerID = runnerID
	att.StartedAt = &now
	att.LeaseExpiresAt = &leaseExpires

	// Update task status to doing
	updateTaskQuery := `UPDATE tasks SET status = 'doing', updated_at = $2 WHERE id = $1`
	if _, err := tx.ExecContext(ctx, updateTaskQuery, t.ID, now); err != nil {
		return nil, nil, fmt.Errorf("update task status: %w", err)
	}
	t.Status = task.TaskStatusDoing
	t.UpdatedAt = now

	// Update runner status to busy
	updateRunnerQuery := `
		UPDATE runners SET
			status = 'busy',
			current_task_id = $2,
			current_attempt_id = $3,
			last_heartbeat_at = $4,
			updated_at = $4
		WHERE id = $1
	`
	_, _ = tx.ExecContext(ctx, updateRunnerQuery, runnerID, t.ID, att.ID, now)

	// Log task activity
	actID := generateRandomID("act")
	actQuery := `
		INSERT INTO task_activities (
			id, task_id, attempt_id, activity_type, author, old_status, new_status, message, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	_, _ = tx.ExecContext(ctx, actQuery,
		actID,
		t.ID,
		att.ID,
		task.ActivityTypeAttempt,
		runnerID,
		task.AttemptStatusQueued,
		task.AttemptStatusRunning,
		fmt.Sprintf("Attempt %s diklaim secara eksklusif oleh %s.", att.ID, runnerID),
		now,
	)

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit claim tx: %w", err)
	}

	return &att, &t, nil
}

func (s *DBStore) RenewLease(ctx context.Context, attemptID, runnerID string, leaseDuration time.Duration) (bool, error) {
	var cancelRequested bool
	var status string
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(cancel_requested, false), status FROM task_attempts WHERE id = $1", attemptID).Scan(&cancelRequested, &status)
	if err == sql.ErrNoRows {
		return false, task.ErrAttemptNotFound
	}
	if err != nil {
		return false, fmt.Errorf("query attempt for lease: %w", err)
	}

	if cancelRequested || status == task.AttemptStatusCanceled {
		return true, nil
	}

	now := time.Now().UTC()
	leaseExpires := now.Add(leaseDuration)

	_, err = s.db.ExecContext(ctx, "UPDATE task_attempts SET lease_expires_at = $2 WHERE id = $1", attemptID, leaseExpires)
	if err != nil {
		return false, fmt.Errorf("renew lease on attempt: %w", err)
	}

	_, _ = s.db.ExecContext(ctx, "UPDATE runners SET last_heartbeat_at = $2, updated_at = $2 WHERE id = $1", runnerID, now)
	return false, nil
}

func (s *DBStore) MarkInterruptedLeases(ctx context.Context) ([]*task.TaskAttempt, error) {
	now := time.Now().UTC()
	query := `
		SELECT 
			id, task_id, runner_id, agent_name, status, exit_code, peak_rss_bytes,
			lease_expires_at, cancel_requested, canceled_at, interrupted_at,
			attempt_number, started_at, finished_at, error_message, created_at
		FROM task_attempts
		WHERE status = 'running' AND lease_expires_at < $1
	`
	rows, err := s.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, fmt.Errorf("query expired attempts: %w", err)
	}
	defer rows.Close()

	var expired []*task.TaskAttempt
	for rows.Next() {
		var a task.TaskAttempt
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
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan expired attempt: %w", err)
		}
		expired = append(expired, &a)
	}

	exitCode := 137
	errMsg := "Runner heartbeat lost and lease expired; marked interrupted and recoverable without auto-rerun."

	for _, a := range expired {
		a.Status = task.AttemptStatusInterrupted
		a.FinishedAt = &now
		a.InterruptedAt = &now
		a.ExitCode = &exitCode
		a.ErrorMessage = errMsg

		_, _ = s.db.ExecContext(ctx, `
			UPDATE task_attempts SET
				status = 'interrupted',
				finished_at = $2,
				interrupted_at = $2,
				exit_code = $3,
				error_message = $4
			WHERE id = $1
		`, a.ID, now, exitCode, errMsg)

		// Set task to review
		_, _ = s.db.ExecContext(ctx, "UPDATE tasks SET status = 'review', updated_at = $2 WHERE id = $1", a.TaskID, now)

		// Set runner to offline
		_, _ = s.db.ExecContext(ctx, "UPDATE runners SET status = 'offline', current_attempt_id = NULL, current_task_id = NULL, updated_at = $2 WHERE id = $1", a.RunnerID, now)

		// Record activity
		actID := generateRandomID("act")
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO task_activities (
				id, task_id, attempt_id, activity_type, author, old_status, new_status, message, created_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9
			)
		`, actID, a.TaskID, a.ID, task.ActivityTypeInterrupted, "system", task.AttemptStatusRunning, task.AttemptStatusInterrupted,
			fmt.Sprintf("Attempt %s interrupted (lease expired). Task is in review and can be retried explicitly.", a.ID), now)
	}

	return expired, nil
}

func generateRandomID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

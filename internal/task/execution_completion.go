package task

import (
	"context"
	"time"
)

// CompletionStore atomically fences the attempt and its task result. Side effects
// and notifications must happen only after this commit succeeds.
type CompletionStore interface {
	CompleteExecution(context.Context, *TaskAttempt, *Task, *TaskAttempt, time.Time, string, bool, ...*TaskActivity) error
}

func validCompletion(current, result *TaskAttempt, currentTask, resultTask *Task, latestID string, expected time.Time, owner string, requireLease bool) error {
	if current.Status != AttemptStatusRunning && current.Status != AttemptStatusQueued {
		return ErrAttemptFinal
	}
	if current.ID != latestID || current.RunnerID != owner || current.TaskID != resultTask.ID {
		return ErrRequestConflict
	}
	if currentTask.Status != TaskStatusDoing && currentTask.Status != TaskStatusQueued && currentTask.Status != TaskStatusPending {
		return ErrRequestConflict
	}
	if !currentTask.UpdatedAt.Equal(expected) {
		return ErrRequestConflict
	}
	if requireLease && current.LeaseExpiresAt != nil && !current.LeaseExpiresAt.After(time.Now()) {
		return ErrAttemptFinal
	}
	if (requireLease || result.Status == AttemptStatusRunning) && current.ExecutionDeadline != nil && time.Now().After(current.ExecutionDeadline.Add(GetGracePeriod())) {
		return ErrAttemptFinal
	}
	if requireLease && current.ExecutionDeadline != nil && current.LeaseExpiresAt == nil {
		return ErrAttemptFinal
	}
	if !requireLease && result.Status == AttemptStatusInterrupted && (current.LeaseExpiresAt == nil || current.LeaseExpiresAt.After(time.Now())) {
		return ErrRequestConflict
	}
	if current.CancelRequested && result.Status != AttemptStatusCanceled {
		return ErrAttemptFinal
	}
	return nil
}

func (m *MemoryStore) CompleteExecution(_ context.Context, a *TaskAttempt, t *Task, next *TaskAttempt, expected time.Time, owner string, requireLease bool, activities ...*TaskActivity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.attempts[a.ID]
	original := m.tasks[t.ID]
	if current == nil || original == nil {
		return ErrAttemptNotFound
	}
	var latest *TaskAttempt
	for _, candidate := range m.attempts {
		if candidate.TaskID == t.ID && (latest == nil || candidate.AttemptNumber > latest.AttemptNumber || (candidate.AttemptNumber == latest.AttemptNumber && candidate.CreatedAt.After(latest.CreatedAt))) {
			latest = candidate
		}
	}
	if err := validCompletion(current, a, original, t, latest.ID, expected, owner, requireLease); err != nil {
		return err
	}
	ac, tc := *a, *t
	m.attempts[a.ID], m.tasks[t.ID] = &ac, &tc
	if next != nil {
		nc := *next
		m.attempts[nc.ID] = &nc
	}
	for _, activity := range activities {
		cp := *activity
		m.activities[cp.ID] = &cp
	}
	return nil
}

func (s *DBStore) CompleteExecution(ctx context.Context, a *TaskAttempt, t *Task, next *TaskAttempt, expected time.Time, owner string, requireLease bool, activities ...*TaskActivity) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "task:"+t.ID); err != nil {
		return err
	}
	var original Task
	if err = tx.QueryRowContext(ctx, `SELECT status,updated_at FROM tasks WHERE id=$1 FOR UPDATE`, t.ID).Scan(&original.Status, &original.UpdatedAt); err != nil {
		return err
	}
	var current TaskAttempt
	current.ID = a.ID
	if err = tx.QueryRowContext(ctx, `SELECT task_id,runner_id,status,lease_expires_at,execution_deadline,COALESCE(cancel_requested,false) FROM task_attempts WHERE id=$1 FOR UPDATE`, a.ID).Scan(&current.TaskID, &current.RunnerID, &current.Status, &current.LeaseExpiresAt, &current.ExecutionDeadline, &current.CancelRequested); err != nil {
		return err
	}
	var latestID string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM task_attempts WHERE task_id=$1 ORDER BY attempt_number DESC,created_at DESC,id DESC LIMIT 1`, t.ID).Scan(&latestID); err != nil {
		return err
	}
	if err = validCompletion(&current, a, &original, t, latestID, expected, owner, requireLease); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE task_attempts SET status=$2,exit_code=$3,peak_rss_bytes=$4,finished_at=$5,error_message=$6,interrupted_at=$7 WHERE id=$1`, a.ID, a.Status, a.ExitCode, a.PeakRSSBytes, a.FinishedAt, a.ErrorMessage, a.InterruptedAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tasks SET status=$2,summary=$3,complete_note=$4,dod_checklist=$5::jsonb,branch_name=$6,pr_url=$7,pr_number=$8,commit_sha=$9,diff_summary=$10,test_summary=$11,updated_at=$12 WHERE id=$1`, t.ID, t.Status, t.Summary, t.CompleteNote, checklistJSON(t.DoDChecklist), t.BranchName, t.PRURL, t.PRNumber, t.CommitSHA, t.DiffSummary, t.TestSummary, t.UpdatedAt); err != nil {
		return err
	}
	if next != nil {
		if _, err = tx.ExecContext(ctx, `INSERT INTO task_attempts(id,task_id,runner_id,agent_name,status,attempt_number,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, next.ID, next.TaskID, next.RunnerID, next.AgentName, next.Status, next.AttemptNumber, next.CreatedAt); err != nil {
			return err
		}
	}
	for _, activity := range activities {
		if _, err = tx.ExecContext(ctx, `INSERT INTO task_activities(id,task_id,attempt_id,activity_type,author,old_status,new_status,message,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, activity.ID, activity.TaskID, activity.AttemptID, activity.ActivityType, activity.Author, activity.OldStatus, activity.NewStatus, activity.Message, activity.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

var _ CompletionStore = (*DBStore)(nil)
var _ CompletionStore = (*MemoryStore)(nil)

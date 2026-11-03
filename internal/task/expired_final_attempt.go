package task

import (
	"context"
	"time"
)

const AcceptedTaskLeaseCleanup = "Execution lease expired after task acceptance; accepted result preserved."

type AcceptedAttemptRetirer interface {
	RetireExpiredAcceptedAttempt(context.Context, string, string) (bool, error)
}

// Only retires an expired process lease. Never rewrites task acceptance/evidence.
func (s *DBStore) RetireExpiredAcceptedAttempt(ctx context.Context, id, runner string) (bool, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `WITH retired AS (
 UPDATE task_attempts a SET status='interrupted',finished_at=$3,interrupted_at=$3,exit_code=137,error_message=$4
 FROM tasks t WHERE a.id=$1 AND a.runner_id=$2 AND a.task_id=t.id AND t.status='done'
 AND a.status='running' AND a.lease_expires_at < $3 AND NOT COALESCE(a.cancel_requested,false)
 RETURNING a.id,a.task_id)
 INSERT INTO task_activities(id,task_id,attempt_id,activity_type,author,old_status,new_status,message,created_at)
 SELECT $5,task_id,id,'attempt_lease_cleanup','recovery','running','interrupted',$4,$3 FROM retired`, id, runner, now, AcceptedTaskLeaseCleanup, generateID("act"))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (m *MemoryStore) RetireExpiredAcceptedAttempt(_ context.Context, id, runner string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.attempts[id]
	if current == nil {
		return false, nil
	}
	item := m.tasks[current.TaskID]
	now := time.Now().UTC()
	if item == nil || item.Status != TaskStatusDone || current.RunnerID != runner || current.Status != AttemptStatusRunning || current.LeaseExpiresAt == nil || !current.LeaseExpiresAt.Before(now) || current.CancelRequested {
		return false, nil
	}
	next := *current
	next.Status = AttemptStatusInterrupted
	next.FinishedAt = &now
	next.InterruptedAt = &now
	code := 137
	next.ExitCode = &code
	next.ErrorMessage = AcceptedTaskLeaseCleanup
	m.attempts[id] = &next
	activity := &TaskActivity{ID: generateID("act"), TaskID: item.ID, AttemptID: &next.ID, ActivityType: "attempt_lease_cleanup", Author: "recovery", OldStatus: AttemptStatusRunning, NewStatus: AttemptStatusInterrupted, Message: AcceptedTaskLeaseCleanup, CreatedAt: now}
	m.activities[activity.ID] = activity
	return true, nil
}

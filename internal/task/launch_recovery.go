package task

import (
	"context"
	"database/sql"
	"github.com/buchin/cuan/internal/executionpolicy"
	"strings"
	"time"
)

const MissingWorkspaceLaunchError = "Original task workspace is missing; its branch was retained for recovery"
const missingWorkspaceRecoveryKey = "missing-workspace-v3"
const IsolatedWorkspaceLaunchError = "Could not prepare the isolated task workspace"
const isolatedWorkspaceRecoveryKey = "isolated-workspace-recovery-v1"
const StandaloneArtifactRecoveryKind = "standalone-artifact-workspace-v1"
const InvalidRepositoryLaunchError = "invalid GitHub repository URL. Use format 'https://github.com/owner/repo' or 'owner/repo'"
const ActiveAttemptLaunchError = "This task already has an active attempt. Check the session before retrying."

// Only known pre-reservation failures with a shipped repair are replayable.
func UnstartedLaunchRecoveryKind(j TaskLaunch, tasks ...*Task) string {
	if j.SafeError == ActiveAttemptLaunchError {
		return "active-attempt-admission-v1"
	}
	if len(tasks) > 0 && tasks[0] != nil && j.SafeError == InvalidRepositoryLaunchError &&
		executionpolicy.StandaloneArtifactWork(tasks[0].Instructions) &&
		InferExecutionCategory(tasks[0].Title, tasks[0].Instructions) == ExecutionCategoryOps {
		return StandaloneArtifactRecoveryKind
	}
	if j.SafeError == MissingWorkspaceLaunchError {
		return missingWorkspaceRecoveryKey
	}
	if j.SafeError == IsolatedWorkspaceLaunchError {
		return isolatedWorkspaceRecoveryKey
	}
	if j.SafeError == "create attempt: Invalid task input" {
		switch strings.ToLower(strings.TrimPrefix(j.AgentID, "@")) {
		case "sm", "agent_sm", "dev", "agent_dev", "support", "agent_support":
			return "legacy-agent-routing-v1"
		}
	}
	return ""
}

type launchRecoveryStore interface {
	RecoverUnstartedLaunch(context.Context, string, time.Time, time.Time) (bool, error)
}

func (s *Service) RecoverUnstartedLaunch(ctx context.Context, id string, expected, now time.Time) (bool, error) {
	if store, ok := s.store.(launchRecoveryStore); ok {
		return store.RecoverUnstartedLaunch(ctx, id, expected, now)
	}
	return false, nil
}
func (m *MemoryStore) RecoverUnstartedLaunch(_ context.Context, id string, expected, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.launches[id]
	if j == nil {
		return false, ErrTaskNotFound
	}
	t := m.tasks[j.TaskID]
	kind := UnstartedLaunchRecoveryKind(*j, t)
	key := id + "\x00" + kind
	if m.launchRecoveries[key] || j.State != "stopped" || kind == "" || !j.UpdatedAt.Equal(expected) || !j.ExpiresAt.After(now) || (j.Revision == "" && kind != StandaloneArtifactRecoveryKind) || t == nil || (t.Status != TaskStatusQueued && t.Status != TaskStatusBlocked) || t.ProjectID != j.ProjectID || LaunchInstructionsHash(t.Instructions) != j.InstructionsHash {
		return false, nil
	}
	for _, other := range m.launches {
		if other.TaskID == j.TaskID && (other.CreatedAt.After(j.CreatedAt) || (other.CreatedAt.Equal(j.CreatedAt) && other.ID > j.ID)) {
			return false, nil
		}
	}
	for _, a := range m.attempts {
		if a.TaskID == j.TaskID && (kind == StandaloneArtifactRecoveryKind || a.ID == attemptRequestID(j.TaskID, j.RequestID) || a.Status == AttemptStatusQueued || a.Status == AttemptStatusRunning) {
			return false, nil
		}
	}
	if m.launchRecoveries == nil {
		m.launchRecoveries = make(map[string]bool)
	}
	m.launchRecoveries[key] = true
	j.State = "queued"
	j.SafeError = ""
	j.LeaseToken = ""
	j.LeaseUntil = nil
	j.AvailableAt = now
	j.UpdatedAt = now
	if kind == StandaloneArtifactRecoveryKind {
		t.ExecutionCategory = ExecutionCategoryOps
		t.UpdatedAt = now
	}
	if t.Status == TaskStatusBlocked {
		t.Status = TaskStatusQueued
		t.UpdatedAt = now
	}
	activity := &TaskActivity{ID: generateID("act"), TaskID: t.ID, ActivityType: "execution_recovery", Author: "coordinator", NewStatus: TaskStatusQueued, Message: "Requeued the existing approved launch for one repaired pre-launch check. No execution attempt has started.", CreatedAt: now}
	m.activities[activity.ID] = activity
	return true, nil
}
func (s *DBStore) RecoverUnstartedLaunch(ctx context.Context, id string, expected, now time.Time) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var taskID string
	var observed TaskLaunch
	if err = tx.QueryRowContext(ctx, `SELECT task_id,agent_id,safe_error FROM task_launches WHERE id=$1`, id).Scan(&taskID, &observed.AgentID, &observed.SafeError); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "task:"+taskID); err != nil {
		return false, err
	}
	var scope Task
	if err = tx.QueryRowContext(ctx, `SELECT title,instructions FROM tasks WHERE id=$1`, taskID).Scan(&scope.Title, &scope.Instructions); err != nil {
		return false, err
	}
	kind := UnstartedLaunchRecoveryKind(observed, &scope)
	if kind == "" {
		return false, nil
	}
	var recovered string
	err = tx.QueryRowContext(ctx, `WITH eligible AS (
 SELECT j.id,j.task_id FROM task_launches j JOIN tasks t ON t.id=j.task_id
 WHERE j.id=$1 AND j.updated_at=$2 AND j.state='stopped' AND j.safe_error=$3 AND j.expires_at>$4 AND ($5='standalone-artifact-workspace-v1' OR btrim(j.revision)<>'')
 AND t.status IN ('queued','blocked') AND COALESCE(t.project_id,'')=j.project_id
 AND j.instructions_hash=encode(digest(t.instructions,'sha256'),'hex')
 AND j.id=(SELECT id FROM task_launches WHERE task_id=j.task_id ORDER BY created_at DESC,id DESC LIMIT 1)
 AND NOT EXISTS(SELECT 1 FROM task_attempts a WHERE a.task_id=j.task_id AND ($5='standalone-artifact-workspace-v1' OR a.id=j.attempt_id OR a.status IN ('queued','running')))
 FOR UPDATE OF j,t
 ), receipt AS (
 INSERT INTO task_launch_recoveries(launch_id,recovery_key,created_at)
 SELECT id,$5,$4 FROM eligible ON CONFLICT DO NOTHING RETURNING launch_id
 ), resumed AS (
 UPDATE task_launches SET state='queued',safe_error='',lease_token='',lease_until=NULL,available_at=$4,updated_at=$4
 WHERE id IN (SELECT launch_id FROM receipt) RETURNING task_id
 ) UPDATE tasks SET status='queued',execution_category=CASE WHEN $5='standalone-artifact-workspace-v1' THEN 'ops' ELSE execution_category END,updated_at=$4 WHERE id IN (SELECT task_id FROM resumed) AND status IN ('queued','blocked') RETURNING id`, id, expected, observed.SafeError, now, kind).Scan(&recovered)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO task_activities(id,task_id,activity_type,author,new_status,message,created_at) VALUES($1,$2,'execution_recovery','coordinator','queued',$3,$4)`, generateID("act"), taskID, "Requeued the existing approved launch for one repaired pre-launch check. No execution attempt has started.", now); err != nil {
		return false, err
	}
	return recovered != "", tx.Commit()
}

package task

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// InspectionIdentityStore restores only the retained implementation identity.
// It never supplies a completion receipt, changes history, or starts execution.
type InspectionIdentityStore interface {
	RestoreInspectionIdentity(context.Context, *Task, *TaskAttempt, *LiveInspectionIntent, *TaskActivity) (bool, error)
}

// ReconcileInspectionIdentity repairs the historical report bug where a
// read-only inspector replaced the implementation commit with its target.
func (s *Service) ReconcileInspectionIdentity(ctx context.Context, id string) (bool, error) {
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	writer, ok := s.store.(InspectionIdentityStore)
	if !ok {
		return false, nil
	}
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return false, err
	}
	a, err := s.store.GetLatestAttempt(ctx, id)
	if err != nil {
		return false, err
	}
	if t.Status != TaskStatusBlocked || a == nil || a.Status != AttemptStatusInterrupted || a.FinishedAt == nil || a.CancelRequested || a.CanceledAt != nil {
		return false, nil
	}
	proof, err := LiveInspectionForAttempt(ctx, s.store, t, a.ID)
	if err != nil {
		return false, err
	}
	if proof == nil || !fullObjectID(proof.SourceCommit) || proof.SourceCommit == proof.TargetCommit || t.CommitSHA != proof.TargetCommit {
		return false, nil
	}
	restored := *t
	restored.CommitSHA = proof.SourceCommit
	if ValidateLiveInspectionSource(ctx, s.store, &restored, proof) != nil {
		return false, nil
	}
	source, err := s.store.GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil {
		return false, err
	}
	events, err := s.store.ListActivities(ctx, id)
	if err != nil {
		return false, err
	}
	// The launch alone cannot invent an implementation. Require its original
	// native runner transition, before this inspection, with the exact commit.
	native := false
	for _, e := range events {
		if e != nil && e.TaskID == id && e.AttemptID != nil && *e.AttemptID == source.ID && e.Author == source.RunnerID && e.ActivityType == "attempt_transition" && !e.CreatedAt.After(proof.ObservedAt) && strings.HasSuffix(e.Message, "(commit: "+proof.SourceCommit+").") {
			native = true
		}
	}
	if !native {
		return false, nil
	}
	raw, _ := json.Marshal(map[string]string{"source_attempt_id": proof.SourceAttemptID, "inspection_attempt_id": a.ID, "previous_commit": t.CommitSHA, "restored_source_commit": proof.SourceCommit, "target_commit": proof.TargetCommit, "reason": "Restore retained source from immutable inspection intent and native source receipt; interrupted inspection remains unverified."})
	activity := &TaskActivity{ID: "inspection-identity:" + a.ID, TaskID: id, AttemptID: &a.ID, ActivityType: "inspection_identity_reconciled", Author: "system", Message: string(raw), CreatedAt: time.Now().UTC()}
	return writer.RestoreInspectionIdentity(ctx, t, a, proof, activity)
}

func (s *DBStore) RestoreInspectionIdentity(ctx context.Context, t *Task, a *TaskAttempt, proof *LiveInspectionIntent, activity *TaskActivity) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "task:"+t.ID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT id FROM tasks WHERE id=$1 FOR UPDATE`, t.ID); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE tasks t SET commit_sha=$4,updated_at=NOW()
 WHERE t.id=$1 AND t.status='blocked' AND t.updated_at=$2 AND t.commit_sha=$3
 AND (SELECT id FROM task_attempts WHERE task_id=t.id ORDER BY attempt_number DESC,created_at DESC,id DESC LIMIT 1)=$5
 AND EXISTS(SELECT 1 FROM task_attempts WHERE id=$5 AND task_id=t.id AND status='interrupted' AND finished_at IS NOT NULL AND NOT COALESCE(cancel_requested,false) AND canceled_at IS NULL)
 AND EXISTS(SELECT 1 FROM task_attempts WHERE id=$6 AND task_id=t.id AND status IN ('failed','succeeded') AND NOT COALESCE(cancel_requested,false) AND canceled_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM task_attempts WHERE task_id=t.id AND status IN ('queued','running'))
 AND NOT EXISTS(SELECT 1 FROM task_launches WHERE task_id=t.id AND state IN ('queued','claimed','started'))`, t.ID, t.UpdatedAt, t.CommitSHA, proof.SourceCommit, a.ID, proof.SourceAttemptID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO task_activities(id,task_id,attempt_id,activity_type,author,message,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, activity.ID, activity.TaskID, activity.AttemptID, activity.ActivityType, activity.Author, activity.Message, activity.CreatedAt); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (m *MemoryStore) RestoreInspectionIdentity(_ context.Context, t *Task, a *TaskAttempt, proof *LiveInspectionIntent, activity *TaskActivity) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.tasks[t.ID]
	attempt := m.attempts[a.ID]
	if current == nil || attempt == nil || current.Status != TaskStatusBlocked || !current.UpdatedAt.Equal(t.UpdatedAt) || current.CommitSHA != t.CommitSHA || attempt.Status != AttemptStatusInterrupted || attempt.FinishedAt == nil || attempt.CancelRequested || attempt.CanceledAt != nil {
		return false, nil
	}
	source := m.attempts[proof.SourceAttemptID]
	if source == nil || source.TaskID != t.ID || (source.Status != AttemptStatusFailed && source.Status != AttemptStatusSucceeded) || source.CancelRequested || source.CanceledAt != nil {
		return false, nil
	}
	for _, candidate := range m.attempts {
		if candidate.TaskID == t.ID && (candidate.Status == AttemptStatusRunning || candidate.Status == AttemptStatusQueued || candidate.AttemptNumber > a.AttemptNumber || (candidate.AttemptNumber == a.AttemptNumber && (candidate.CreatedAt.After(a.CreatedAt) || candidate.CreatedAt.Equal(a.CreatedAt) && candidate.ID > a.ID))) {
			return false, nil
		}
	}
	for _, launch := range m.launches {
		if launch.TaskID == t.ID && (launch.State == "queued" || launch.State == "claimed" || launch.State == "started") {
			return false, nil
		}
	}
	copyTask := *current
	copyTask.CommitSHA = proof.SourceCommit
	copyTask.UpdatedAt = time.Now().UTC()
	m.tasks[t.ID] = &copyTask
	copyActivity := *activity
	m.activities[activity.ID] = &copyActivity
	return true, nil
}

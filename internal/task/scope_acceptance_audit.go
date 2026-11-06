package task

import (
	"context"
	"sort"
	"strings"
	"time"
)

const ActivityScopeAcceptanceAudit = "scope_acceptance_audit"
const LegacyForeignPageRequirement = "Kepatuhan Landing Page: task memperkenalkan fitur/kapabilitas baru tetapi Landing Page Cuan"
const LegacyForeignPagePath = "web/src/features/landing/LandingPage.tsx"

type ScopeAcceptanceAuditStore interface {
	LegacyScopeAcceptanceTaskIDs(context.Context, string) ([]string, error)
	RequeueScopeAcceptance(context.Context, string, string, time.Time, string) (bool, error)
}

func (s *DBStore) LegacyScopeAcceptanceTaskIDs(ctx context.Context, ownRepo string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id FROM tasks t JOIN projects p ON p.id=t.project_id
 JOIN LATERAL (SELECT id,status,cancel_requested,canceled_at FROM task_attempts WHERE task_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1) a ON true
 WHERE t.status='done' AND t.source_room_id IS NOT NULL AND t.instructions LIKE 'Cuan managed delegation v1.%'
 AND lower(p.repo_url) NOT IN(lower($1),lower($1||'.git'))
 AND strpos(t.diff_summary,$2)>0 AND a.status='succeeded' AND NOT COALESCE(a.cancel_requested,false) AND a.canceled_at IS NULL
 AND (SELECT r.author='coordinator' AND r.new_status='done' FROM task_activities r WHERE r.task_id=t.id AND r.activity_type='status_change' ORDER BY r.created_at DESC,r.id DESC LIMIT 1)=true
 AND EXISTS(SELECT 1 FROM tasks source JOIN task_activities r ON r.task_id=source.id WHERE source.project_id=t.project_id AND source.source_room_id=t.source_room_id AND r.activity_type='attempt_transition' AND r.author='cuan_dod_evaluator' AND r.new_status='queued' AND strpos(r.message,$3)>0)
 AND NOT EXISTS(SELECT 1 FROM task_activities x WHERE x.task_id=t.id AND x.attempt_id=a.id AND x.activity_type=$4)
 ORDER BY t.created_at,t.id LIMIT 50`, ownRepo, LegacyForeignPagePath, LegacyForeignPageRequirement, ActivityScopeAcceptanceAudit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *DBStore) RequeueScopeAcceptance(ctx context.Context, id, attemptID string, updatedAt time.Time, message string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `WITH reopened AS (
 UPDATE tasks t SET status='review',updated_at=NOW() WHERE t.id=$1 AND t.status='done' AND t.updated_at=$3
 AND (SELECT a.id FROM task_attempts a WHERE a.task_id=t.id ORDER BY a.created_at DESC,a.id DESC LIMIT 1)=$2
 AND EXISTS(SELECT 1 FROM task_attempts a WHERE a.id=$2 AND a.task_id=t.id AND a.status='succeeded' AND NOT COALESCE(a.cancel_requested,false) AND a.canceled_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM task_launches l WHERE l.task_id=t.id AND l.state IN ('queued','claimed'))
 AND NOT EXISTS(SELECT 1 FROM task_attempts active WHERE active.task_id=t.id AND active.status IN ('queued','running'))
 AND NOT EXISTS(SELECT 1 FROM task_activities x WHERE x.task_id=t.id AND x.attempt_id=$2 AND x.activity_type=$4)
 RETURNING t.id)
 INSERT INTO task_activities(id,task_id,attempt_id,activity_type,author,new_status,message,created_at)
 SELECT $5,id,$2,$4,'coordinator','review',$6,NOW() FROM reopened`, id, attemptID, updatedAt, ActivityScopeAcceptanceAudit, generateID("scope_audit"), message)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

// Memory discovery cannot inspect project URLs; the server verifies repository
// identity before any mutation, as it also does for SQL candidates.
func (m *MemoryStore) LegacyScopeAcceptanceTaskIDs(ctx context.Context, _ string) ([]string, error) {
	items, err := m.ListTasks(ctx, TaskFilter{})
	if err != nil {
		return nil, err
	}
	affected := map[string]bool{}
	for _, item := range items {
		events, _ := m.ListActivities(ctx, item.ID)
		for _, e := range events {
			if e.Author == "cuan_dod_evaluator" && e.ActivityType == "attempt_transition" && e.NewStatus == TaskStatusQueued && strings.Contains(e.Message, LegacyForeignPageRequirement) {
				affected[item.ProjectID+"\x00"+item.SourceRoomID] = true
			}
		}
	}
	var ids []string
	created := map[string]time.Time{}
	for _, item := range items {
		created[item.ID] = item.CreatedAt
		if item.Status != TaskStatusDone || item.SourceRoomID == "" || !strings.HasPrefix(item.Instructions, "Cuan managed delegation v1.\n") || !strings.Contains(item.DiffSummary, LegacyForeignPagePath) || !affected[item.ProjectID+"\x00"+item.SourceRoomID] {
			continue
		}
		a, err := m.GetLatestAttempt(ctx, item.ID)
		if err != nil || a == nil || a.Status != AttemptStatusSucceeded || a.CancelRequested || a.CanceledAt != nil {
			continue
		}
		events, _ := m.ListActivities(ctx, item.ID)
		audited := false
		var accepted *TaskActivity
		for _, e := range events {
			if e.ActivityType == ActivityScopeAcceptanceAudit && e.AttemptID != nil && *e.AttemptID == a.ID {
				audited = true
			}
			if e.ActivityType == ActivityTypeStatusChange && (accepted == nil || e.CreatedAt.After(accepted.CreatedAt) || (e.CreatedAt.Equal(accepted.CreatedAt) && e.ID > accepted.ID)) {
				accepted = e
			}
		}
		if !audited && accepted != nil && accepted.Author == "coordinator" && accepted.NewStatus == TaskStatusDone {
			ids = append(ids, item.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if created[ids[i]].Equal(created[ids[j]]) {
			return ids[i] < ids[j]
		}
		return created[ids[i]].Before(created[ids[j]])
	})
	if len(ids) > 50 {
		ids = ids[:50]
	}
	return ids, nil
}

func (m *MemoryStore) RequeueScopeAcceptance(_ context.Context, id, attemptID string, updatedAt time.Time, message string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.tasks[id]
	if item == nil || item.Status != TaskStatusDone || !item.UpdatedAt.Equal(updatedAt) {
		return false, nil
	}
	var latest *TaskAttempt
	for _, a := range m.attempts {
		if a.TaskID == id && (a.Status == AttemptStatusRunning || a.Status == AttemptStatusQueued) {
			return false, nil
		}
		if a.TaskID == id && (latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID)) {
			latest = a
		}
	}
	if latest == nil || latest.ID != attemptID || latest.Status != AttemptStatusSucceeded || latest.CancelRequested || latest.CanceledAt != nil {
		return false, nil
	}
	for _, l := range m.launches {
		if l.TaskID == id && (l.State == "queued" || l.State == "claimed") {
			return false, nil
		}
	}
	for _, e := range m.activities {
		if e.TaskID == id && e.AttemptID != nil && *e.AttemptID == attemptID && e.ActivityType == ActivityScopeAcceptanceAudit {
			return false, nil
		}
	}
	now := time.Now().UTC()
	item.Status, item.UpdatedAt = TaskStatusReview, now
	e := &TaskActivity{ID: generateID("scope_audit"), TaskID: id, AttemptID: &attemptID, ActivityType: ActivityScopeAcceptanceAudit, Author: "coordinator", NewStatus: TaskStatusReview, Message: message, CreatedAt: now}
	m.activities[e.ID] = e
	return true, nil
}

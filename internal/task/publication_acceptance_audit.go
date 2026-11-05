package task

import (
	"context"
	"sort"
	"strings"
	"time"
)

const ActivityPublicationAcceptanceAudit = "publication_acceptance_audit"

type PublicationAcceptanceAuditStore interface {
	LegacyPublicationAcceptanceTaskIDs(context.Context) ([]string, error)
	RequeueAcceptedSnapshot(context.Context, string, string, time.Time) (bool, error)
}

func (s *DBStore) LegacyPublicationAcceptanceTaskIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id FROM tasks t
 JOIN LATERAL (SELECT id,status,cancel_requested,canceled_at FROM task_attempts WHERE task_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1) a ON true
 WHERE t.status='done' AND COALESCE(t.execution_category,'') IN ('','code')
 AND t.instructions LIKE 'Cuan managed delegation v1.%' AND strpos(t.instructions,'Stay within this scope; do not deploy unless authorized.')>0
 AND a.status='succeeded' AND NOT COALESCE(a.cancel_requested,false) AND a.canceled_at IS NULL
 AND EXISTS(SELECT 1 FROM task_activities r WHERE r.task_id=t.id AND r.attempt_id=a.id AND r.activity_type='coordinator_review_assessment' AND r.author='coordinator')
 AND (SELECT r.author FROM task_activities r WHERE r.task_id=t.id AND r.activity_type='status_change' ORDER BY r.created_at DESC,r.id DESC LIMIT 1)='coordinator'
 AND NOT EXISTS(SELECT 1 FROM task_activities x WHERE x.task_id=t.id AND x.attempt_id=a.id AND x.activity_type=$1 AND x.author='coordinator')
 ORDER BY t.id LIMIT 50`, ActivityPublicationAcceptanceAudit)
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

func (m *MemoryStore) LegacyPublicationAcceptanceTaskIDs(ctx context.Context) ([]string, error) {
	items, err := m.ListTasks(ctx, TaskFilter{Status: TaskStatusDone})
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, t := range items {
		if (t.ExecutionCategory != "" && t.ExecutionCategory != ExecutionCategoryCode) || !strings.HasPrefix(t.Instructions, "Cuan managed delegation v1.\n") || !strings.Contains(t.Instructions, "Stay within this scope; do not deploy unless authorized.") {
			continue
		}
		a, err := m.GetLatestAttempt(ctx, t.ID)
		if err != nil || a == nil || a.Status != AttemptStatusSucceeded || a.CancelRequested || a.CanceledAt != nil {
			continue
		}
		activities, err := m.ListActivities(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		audited, assessed := false, false
		var accepted *TaskActivity
		for _, x := range activities {
			if x.ActivityType == ActivityTypeStatusChange && (accepted == nil || x.CreatedAt.After(accepted.CreatedAt) || (x.CreatedAt.Equal(accepted.CreatedAt) && x.ID > accepted.ID)) {
				accepted = x
			}
			if x.ActivityType == "coordinator_review_assessment" && x.Author == "coordinator" && x.AttemptID != nil && *x.AttemptID == a.ID {
				assessed = true
			}
			if x.ActivityType == ActivityPublicationAcceptanceAudit && x.Author == "coordinator" && x.AttemptID != nil && *x.AttemptID == a.ID {
				audited = true
			}
		}
		if !audited && assessed && accepted != nil && accepted.Author == "coordinator" && accepted.NewStatus == TaskStatusDone {
			ids = append(ids, t.ID)
		}
	}
	sort.Strings(ids)
	if len(ids) > 50 {
		ids = ids[:50]
	}
	return ids, nil
}

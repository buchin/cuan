package task

import "context"

// Accepted work is never reopened to deliver its customer-facing reply.
const ActivityCustomerReplyPrepared = "customer_reply_prepared"

type CustomerReplyCandidateStore interface {
	CustomerReplyTaskIDs(context.Context) ([]string, error)
}

func (s *DBStore) CustomerReplyTaskIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id FROM tasks t JOIN project_rooms r ON r.id=t.source_room_id
 JOIN LATERAL(SELECT id,status FROM task_attempts WHERE task_id=t.id ORDER BY created_at DESC LIMIT 1) a ON true
 WHERE t.status='done' AND t.instructions LIKE $1 AND COALESCE(r.topic,'') NOT LIKE 'agent:%' AND COALESCE(r.topic,'') NOT LIKE 'agent-topic:%'
 AND a.status='succeeded'
 AND EXISTS(SELECT 1 FROM task_activities e WHERE e.task_id=t.id AND e.attempt_id=a.id AND e.activity_type='coordinator_review_assessment' AND e.author='coordinator' AND e.message LIKE '%"decision":"pass"%')
 AND NOT EXISTS(SELECT 1 FROM task_activities e WHERE e.task_id=t.id AND e.activity_type=$2 AND e.author='coordinator'
 AND e.attempt_id=a.id
 AND EXISTS(SELECT 1 FROM project_messages m WHERE m.id::text=(CASE WHEN e.activity_type=$2 THEN e.message::jsonb ELSE '{}'::jsonb END->>'reply_id') AND m.room_id=t.source_room_id AND m.author_type='agent'))
 ORDER BY t.updated_at DESC,t.id LIMIT 32`, managedInboxTaskPrefix+"%", ActivityCustomerReplyPrepared)
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

func (m *MemoryStore) CustomerReplyTaskIDs(ctx context.Context) ([]string, error) {
	rows, err := m.ListTasks(ctx, TaskFilter{Status: TaskStatusDone})
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, t := range rows {
		if !managedInboxDeliveryTask(t) {
			continue
		}
		ids = append(ids, t.ID)
	}
	return ids, nil
}

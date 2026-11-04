package task

import (
	"context"
	"errors"
	"strings"
)

const managedInboxTaskPrefix = "Cuan managed delegation v1.\n"

// Delivery needs raw reconciliation candidates, not the board's project names,
// result classifications, live progress and all accepted execution history.
type InboxDeliveryCandidateStore interface {
	InboxDeliveryTaskIDs(context.Context) ([]string, error)
}

func (s *Service) InboxDeliveryTasks(ctx context.Context) ([]*Task, error) {
	if reader, ok := s.store.(InboxDeliveryCandidateStore); ok {
		ids, err := reader.InboxDeliveryTaskIDs(ctx)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return []*Task{}, nil
		}
		return s.store.ListTasks(ctx, TaskFilter{IDs: ids})
	}
	// Custom legacy stores keep the prior behavior without requiring a new API.
	rows, err := s.store.ListTasks(ctx, TaskFilter{})
	if err != nil {
		return nil, err
	}
	var out []*Task
	for _, t := range rows {
		if !managedInboxDeliveryTask(t) {
			continue
		}
		if t.Status == TaskStatusDone {
			a, err := s.store.GetLatestAttempt(ctx, t.ID)
			if err != nil && !errors.Is(err, ErrAttemptNotFound) {
				return nil, err
			}
			if a != nil {
				continue
			}
		}
		out = append(out, t)
	}
	return out, nil
}
func managedInboxDeliveryTask(t *Task) bool {
	return t != nil && t.SourceRoomID != "" && strings.HasPrefix(t.Instructions, managedInboxTaskPrefix) && (t.Status != TaskStatusDone || (strings.TrimSpace(t.Summary) != "" && strings.TrimSpace(t.CompleteNote) != ""))
}
func (s *DBStore) InboxDeliveryTaskIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id FROM tasks t
 WHERE t.source_room_id IS NOT NULL AND t.instructions LIKE $1
 AND (t.status<>'done' OR (btrim(t.summary)<>'' AND btrim(t.complete_note)<>'' AND NOT EXISTS(SELECT 1 FROM task_attempts a WHERE a.task_id=t.id)))`, managedInboxTaskPrefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (m *MemoryStore) InboxDeliveryTaskIDs(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	withAttempts := map[string]bool{}
	for _, a := range m.attempts {
		withAttempts[a.TaskID] = true
	}
	var ids []string
	for _, t := range m.tasks {
		if managedInboxDeliveryTask(t) && (t.Status != TaskStatusDone || !withAttempts[t.ID]) {
			ids = append(ids, t.ID)
		}
	}
	return ids, nil
}

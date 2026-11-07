package server

import (
	"context"

	"github.com/buchin/cuan/internal/task"
)

// Review-only historical facts do not resurrect a failed native launch or grant
// publication. Current live publication and original acceptance stay separate.
func (s *Server) inboxNativePreparationHistory(ctx context.Context, d *task.TaskDetailResponse) any {
	if d == nil || s.taskSvc == nil {
		return nil
	}
	for _, attempt := range d.Attempts {
		if attempt.Status != task.AttemptStatusSucceeded && attempt.Status != task.AttemptStatusFailed {
			continue
		}
		found := false
		for _, event := range d.Activities {
			if event != nil && event.ID == "native-release-preparation:"+attempt.ID {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		prepared, err := task.HistoricalNativeReleasePreparationForAttempt(ctx, s.taskSvc.Store(), &d.Task, attempt.ID)
		if err != nil || prepared == nil {
			continue
		}
		return map[string]any{"preparation": prepared, "attempt_status": attempt.Status, "preparation_only": true, "delivery_not_established_by_preparation": true}
	}
	return nil
}

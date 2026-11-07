package server

import (
	"context"
	"net/http"

	"github.com/buchin/cuan/internal/task"
)

// An attempt becomes claimable before the launch handler finishes its task
// metadata write. Reconcile the exact worker-owned attempt instead of rewinding
// running work to queued or marking it failed after a late write conflict.
func (s *Server) acknowledgeClaimedWorkerAttempt(w http.ResponseWriter, ctx context.Context, taskID, attemptID string) bool {
	detail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil || detail == nil || detail.LatestAttempt == nil {
		return false
	}
	a := detail.LatestAttempt
	if a.ID != attemptID || a.TaskID != taskID || a.RunnerID == "" || a.StartedAt == nil || a.CancelRequested || a.CanceledAt != nil {
		return false
	}
	switch a.Status {
	case task.AttemptStatusRunning, task.AttemptStatusSucceeded, task.AttemptStatusFailed, task.AttemptStatusInterrupted:
	default:
		return false
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": a.Status, "attempt_id": a.ID, "task_id": taskID, "runner_id": a.RunnerID, "already_started": true})
	return true
}

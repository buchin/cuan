package server

import (
	"context"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// This is a size exception for a claimed server-generated envelope, never a new
// execution grant. The normal launch/source/cancellation checks still apply.
func (s *Server) allowsSavedExecutionRevision(ctx context.Context, taskID, ownerID, requestID, revision string) bool {
	if s.taskSvc == nil || requestID == "" || len([]rune(revision)) > task.MaxTaskExecutionRevisionRunes {
		return false
	}
	j, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(taskID, requestID))
	if err != nil || j == nil || j.OwnerID != ownerID || j.TaskID != taskID || j.Revision != revision || j.State != "claimed" || j.LeaseUntil == nil || !j.LeaseUntil.After(time.Now()) {
		return false
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil || d == nil {
		return false
	}
	if p, err := task.LiveInspectionForLaunch(&d.Task, j); err != nil {
		return false
	} else if p != nil {
		return true
	}
	if p, err := task.VerificationIntentForLaunch(&d.Task, j); err != nil {
		return false
	} else if p != nil {
		return true
	}
	p, err := task.NativeReleaseIntentForLaunch(&d.Task, j)
	return err == nil && p != nil
}

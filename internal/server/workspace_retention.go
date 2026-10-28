package server

import (
	"context"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"io"
	"time"
)

// StartWorkspaceRetention limits cleanup to ten completed tasks per hourly pass.
// It never deletes task records, history, branch refs or shared project clones.
func (s *Server) StartWorkspaceRetention(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		cursor := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			cursor = s.runWorkspaceRetention(ctx, cursor, time.Now())
		}
	}()
}

func (s *Server) runWorkspaceRetention(ctx context.Context, cursor int, now time.Time) int {
	if s.projectSvc == nil || s.taskSvc == nil || s.terminalManager == nil {
		return cursor
	}
	doneItems, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: "done"})
	cancelledItems, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: "cancelled"})
	var items []*task.TaskItemResponse
	items = append(items, doneItems...)
	items = append(items, cancelledItems...)
	if len(items) == 0 {
		return cursor
	}
	count := len(items)
	if count > 10 {
		count = 10
	}
	period := project.WorkspaceRetentionPeriod()
	for i := 0; i < count; i++ {
		item := items[cursor%len(items)]
		cursor++
		if item.Status == "done" && now.Sub(item.UpdatedAt) < period {
			continue
		}
		if item.ProjectID == "" {
			continue
		}
		runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		p, err := s.projectSvc.TaskWorkspace(runCtx, item.ProjectID, item.Instructions)
		if err == nil && p.RepoURL != "" {
			_, _ = s.projectSvc.RetireExecutionWorkspace(runCtx, item.ID, p, now, func() (io.Closer, error) { return s.terminalManager.AcquireTaskExecutionLease(item.ID) }, func() (project.WorkspaceRetirementState, error) {
				latest, e := s.taskSvc.GetTaskDetail(runCtx, item.ID)
				if e != nil {
					return project.WorkspaceRetirementState{}, e
				}
				state := project.WorkspaceRetirementState{Status: latest.Status, UpdatedAt: latest.UpdatedAt}
				if latest.Status != "done" && latest.Status != "cancelled" && latest.Status != "canceled" {
					return state, nil
				}
				if latest.Status == "done" && now.Sub(latest.UpdatedAt) < period {
					return state, nil
				}
				if latest.Status == "done" {
					state.RemoteHead, e = s.projectSvc.ConfirmTaskRemoteHead(runCtx, item.ID, p)
				}
				return state, e
			})
		}
		cancel()
	}
	return cursor
}

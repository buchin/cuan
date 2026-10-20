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
			if s.projectSvc == nil || s.taskSvc == nil || s.terminalManager == nil {
				continue
			}
			items, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: "done"})
			if err != nil || len(items) == 0 {
				continue
			}
			count := len(items)
			if count > 10 {
				count = 10
			}
			for i := 0; i < count; i++ {
				item := items[cursor%len(items)]
				cursor++
				if time.Since(item.UpdatedAt) < 7*24*time.Hour || item.ProjectID == "" {
					continue
				}
				runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				p, err := s.projectSvc.TaskWorkspace(runCtx, item.ProjectID, item.Instructions)
				if err == nil && p.RepoURL != "" {
					_, _ = s.projectSvc.RetireExecutionWorkspace(runCtx, item.ID, p, time.Now(), func() (io.Closer, error) { return s.terminalManager.AcquireTaskExecutionLease(item.ID) }, func() (project.WorkspaceRetirementState, error) {
						latest, e := s.taskSvc.GetTaskDetail(runCtx, item.ID)
						if e != nil {
							return project.WorkspaceRetirementState{}, e
						}
						state := project.WorkspaceRetirementState{Status: latest.Status, UpdatedAt: latest.UpdatedAt}
						if latest.Status != "done" || time.Since(latest.UpdatedAt) < 7*24*time.Hour {
							return state, nil
						}
						state.RemoteHead, e = s.projectSvc.ConfirmTaskRemoteHead(runCtx, item.ID, p)
						return state, e
					})
				}
				cancel()
			}
		}
	}()
}

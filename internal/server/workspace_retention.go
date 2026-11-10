package server

import (
	"context"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"io"
	"log"
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
			s.pruneExpiredTaskEvidence(ctx, time.Now())
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
	resolved := func(lookupCtx context.Context, roomID string) bool {
		if roomID == "" || s.projectStudioRepo == nil {
			return false
		}
		room, err := s.projectStudioRepo.GetRoomByID(lookupCtx, roomID)
		if err != nil || room.ResolvedAt == nil {
			return false
		}
		messages, err := s.projectStudioRepo.ListMessagesByRoomID(lookupCtx, roomID)
		if err != nil {
			return false
		}
		for _, message := range messages {
			if message.CreatedAt.After(*room.ResolvedAt) {
				return false
			}
		}
		if reason, err := s.conversationResolutionBlocker(lookupCtx, roomID); err != nil || reason != "" {
			return false
		}
		return true
	}

	for i := 0; i < count; i++ {
		item := items[cursor%len(items)]
		cursor++
		if item.Status == "done" && now.Sub(item.UpdatedAt) < period && !resolved(ctx, item.SourceRoomID) {
			continue
		}
		if item.ProjectID == "" {
			continue
		}
		runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		p, err := s.projectSvc.TaskWorkspace(runCtx, item.ProjectID, item.Instructions)
		if err == nil && p.RepoURL != "" {
			decision, retireErr := s.projectSvc.RetireExecutionWorkspace(runCtx, item.ID, p, now, func() (io.Closer, error) { return s.terminalManager.AcquireTaskExecutionLease(item.ID) }, func() (project.WorkspaceRetirementState, error) {
				latest, e := s.taskSvc.GetTaskDetail(runCtx, item.ID)
				if e != nil {
					return project.WorkspaceRetirementState{}, e
				}
				state := project.WorkspaceRetirementState{Status: latest.Status, UpdatedAt: latest.UpdatedAt, Resolved: resolved(runCtx, latest.SourceRoomID)}
				if latest.Status != "done" && latest.Status != "cancelled" && latest.Status != "canceled" {
					return state, nil
				}
				if latest.Status == "done" && now.Sub(latest.UpdatedAt) < period && !state.Resolved {
					return state, nil
				}
				if latest.Status == "done" && !state.Resolved {
					state.RemoteHead, e = s.projectSvc.ConfirmTaskRemoteHead(runCtx, item.ID, p)
				}
				return state, e
			})
			if retireErr != nil {
				log.Printf("[retention] task checkout retained: %s", item.ID)
			} else if decision.Eligible {
				log.Printf("[retention] task checkout released: %s", item.ID)
			}
		}
		if resolved(runCtx, item.SourceRoomID) {
			s.releaseResolvedTaskCopies(runCtx, item)
		}
		cancel()
	}
	return cursor
}

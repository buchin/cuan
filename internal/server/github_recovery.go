package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"strings"
	"time"
)

func githubRecoveryRequestID(eventID string) string {
	digest := sha256.Sum256([]byte("cuan-github-recovery:" + eventID))
	digest[6] = (digest[6] & 0x0f) | 0x50
	digest[8] = (digest[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", digest[:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}

const githubContinuation = "The owner's Cuan GitHub connection is now available; repository permission must still be verified. Continue the original approved task from its retained workspace and previous progress using the registered project connection, never ambient host gh credentials. Do not repeat implementation or recreate artifacts. Inspect existing files, branches and commits, complete only unfinished approved work, verify the result and report actual evidence. Push only when the original scope requires saving the task branch. Do not merge, deploy or expand scope beyond the original authorization and publication limits."

// QueueGitHubRecovery records continuation only for a waiting task with its
// original Inbox authorization, latest attempt, and unchanged task instructions.
func (s *Server) queueGitHubRecovery(ctx context.Context, detail *task.TaskDetailResponse, owner string) error {
	if detail.Status != "blocked" || detail.SourceRoomID == "" || detail.LatestAttempt == nil {
		return nil
	}
	var event *task.TaskActivity
	for _, a := range detail.Activities {
		if (a.ActivityType == "github_connection_required" || a.ActivityType == "github_connection_resolved") && (event == nil || a.CreatedAt.After(event.CreatedAt)) {
			event = a
		}
	}
	if event == nil || event.ActivityType != "github_connection_required" || event.AttemptID == nil || *event.AttemptID != detail.LatestAttempt.ID {
		return nil
	}
	if detail.LatestAttempt.Status != task.AttemptStatusFailed && detail.LatestAttempt.Status != task.AttemptStatusSucceeded && detail.LatestAttempt.Status != task.AttemptStatusInterrupted {
		return nil
	}
	if !strings.HasSuffix(event.Message, "\nContext fingerprint: "+task.LaunchInstructionsHash(detail.Instructions)) {
		return nil
	}
	reader, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return nil
	}
	requests, err := reader.ListInboxRequests(ctx, detail.SourceRoomID)
	if err != nil {
		return err
	}
	for _, request := range requests {
		if request.OwnerID != owner || request.MessageID != detail.SourceMessageID || request.AgentID != detail.AssignedAgent || request.State == "cancelled" || request.State == "canceled" || request.State == "cancelling" {
			continue
		}
		now := time.Now().UTC()
		_, err = s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: detail.ID, ProjectID: detail.ProjectID, RequestID: githubRecoveryRequestID(event.ID), OwnerID: owner, RoomID: detail.SourceRoomID, SourceRequestID: request.RequestID, AgentID: detail.AssignedAgent, Revision: githubContinuation, InstructionsHash: task.LaunchInstructionsHash(detail.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(24 * time.Hour)})
		return err
	}
	return nil
}
func (s *Server) StartGitHubRecovery(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		cursor := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if s.authSvc == nil || s.taskSvc == nil {
				continue
			}
			owner, err := s.authSvc.OwnerID(ctx)
			if err != nil {
				continue
			}
			path, err := githubConnectionPath(owner)
			if err != nil {
				continue
			}
			githubConnectionMu.Lock()
			_, _, err = git.LoadConnection(path, s.cfg.AuthSecret)
			githubConnectionMu.Unlock()
			if err != nil {
				continue
			}
			items, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{Status: "blocked"})
			if err != nil || len(items) == 0 {
				continue
			}
			count := len(items)
			if count > 20 {
				count = 20
			}
			for i := 0; i < count; i++ {
				item := items[cursor%len(items)]
				cursor++
				if item.SourceRoomID == "" {
					continue
				}
				detail, e := s.taskSvc.GetTaskDetail(ctx, item.ID)
				if e == nil {
					_ = s.queueGitHubRecovery(ctx, detail, owner)
				}
			}
			s.kickTaskLaunches()
		}
	}()
}

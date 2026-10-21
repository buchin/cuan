package server

import (
	"context"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// A terminated execution is an infrastructure gap, not a new scope decision.
// Reuse the durable approval and launch guards instead of asking the chat model
// to run tools or reconstruct a task. Other failures still need result review.
func interruptedInboxTaskCanResume(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.Status != task.TaskStatusBlocked || taskRevisionAttemptCount(d) >= 3 {
		return false
	}
	a := d.LatestAttempt
	return a.Status == task.AttemptStatusFailed && a.ExitCode != nil && *a.ExitCode == 137 && !a.CancelRequested && a.CanceledAt == nil
}

func (s *Server) recoverInterruptedInboxTask(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if !interruptedInboxTaskCanResume(d) || origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || origin.AgentID != d.AssignedAgent || !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") {
		return false
	}
	key := "task-interruption-recovery:" + d.LatestAttempt.ID
	revision := "Gap: the previous execution was interrupted (exit 137); completion and verification are not established. Area: this task's existing project workspace and original approved deliverables. Inspect and preserve existing files before making changes; do not rebuild in provider HOME or another workspace. Reconcile uncertain external effects before repeating them. Continue only unfinished approved work. Verify every original acceptance criterion with targeted checks and report actual evidence and user-accessible results. Finish only when the original scope and verification are complete." + taskRecoveryContext(d)
	output := s.startAuthorizedLaunch(ctx, origin, d, delegationMessageID(key), origin.RequestID, revision)
	// The launch path revalidates approval, cancellation, scope, ownership and
	// active execution, and persists an idempotent launch before starting work.
	now := time.Now().UTC()
	message := &db.ProjectMessage{ID: delegationMessageID(key + ":notice"), RoomID: d.SourceRoomID, AuthorType: "system", AuthorName: "Cuan", Content: "The previous run was interrupted. Your approved work and context are saved.\n\n" + output, CreatedAt: now, UpdatedAt: now}
	if s.projectStudioRepo.CreateMessage(ctx, message) == nil {
		s.publishInbox(d.SourceRoomID)
	}
	return strings.Contains(output, "Work started") || strings.Contains(output, "already handled") || strings.Contains(output, "waiting") || strings.Contains(output, "pending launch")
}

// Cancellation already requires a fresh authenticated user instruction. It is
// not an automated correction attempt and must not permanently exhaust that
// user's ability to continue the original task after resolving a blocker.
func taskRevisionAttemptCount(d *task.TaskDetailResponse) int {
	if d == nil {
		return 0
	}
	count := 0
	for _, a := range d.Attempts {
		if a != nil && a.Status != task.AttemptStatusCanceled && !a.CancelRequested && a.CanceledAt == nil {
			count++
		}
	}
	return count
}

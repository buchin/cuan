package server

import (
	"context"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// A terminated execution is an infrastructure gap, not a new scope decision.
// Reuse the durable approval and launch guards instead of asking the chat model
// to run tools or reconstruct a task. Other failures still need result review.
func interruptedInboxTaskCanResume(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.Status != task.TaskStatusBlocked {
		return false
	}
	a := d.LatestAttempt
	storageFailure := diagnosedStorageInterruption(d, a)
	if a.CancelRequested || a.CanceledAt != nil {
		return false
	}
	if d.CommitSHA != "" && len(d.CommitSHA) >= 7 && !storageFailure {
		return false
	}
	if taskInfrastructureAttemptCount(d) >= 5 {
		return false
	}
	if storageFailure {
		return taskRevisionAttemptCount(d) < 3
	}
	if a.Status == task.AttemptStatusInterrupted {
		return taskRevisionAttemptCount(d) < 5
	}
	if taskRevisionAttemptCount(d) >= 3 {
		return false
	}
	return a.Status == task.AttemptStatusFailed && a.ExitCode != nil && *a.ExitCode == 137
}

func providerLimitedInboxTaskCanResume(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.Status != task.TaskStatusBlocked || taskRevisionAttemptCount(d) >= 3 {
		return false
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.CancelRequested || a.CanceledAt != nil {
		return false
	}
	return strings.HasPrefix(a.ErrorMessage, "The AI provider reported a usage limit. The run was stopped;") || strings.HasPrefix(a.ErrorMessage, "The AI provider reported a location restriction. The run was stopped;")
}

func (s *Server) recoverInterruptedInboxTask(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if (!interruptedInboxTaskCanResume(d) && !providerLimitedInboxTaskCanResume(d)) || origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || origin.AgentID != d.AssignedAgent || !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") {
		return false
	}
	key := "task-interruption-recovery:" + d.LatestAttempt.ID
	gap := "the previous execution was interrupted (exit 137)"
	notice := "Pekerjaan sebelumnya sempat terhenti. Konteks dan pekerjaan yang disetujui tersimpan aman."
	if d.LatestAttempt != nil && (d.LatestAttempt.Status == task.AttemptStatusInterrupted || diagnosedStorageInterruption(d, d.LatestAttempt)) {
		gap = "the previous execution was interrupted during server restart or system update; working tree and files are preserved"
		notice = "Sistem baru saja diperbarui. Pekerjaan Anda tersimpan aman dan dilanjutkan secara otomatis..."
		if d.LatestAttempt.ErrorMessage == runner.StorageInterruptionMessage || diagnosedStorageInterruption(d, d.LatestAttempt) {
			key = "task-storage-recovery:" + d.LatestAttempt.ID
			gap = "the provider could not save its local execution files because server storage was exhausted; preserve files and reconcile prior effects before continuing when storage admission permits"
			notice = "Penyimpanan server sempat penuh. Pekerjaan tersimpan dan menunggu ruang tersedia untuk dilanjutkan."
		}
	} else if providerLimitedInboxTaskCanResume(d) {
		key = "task-provider-recovery:" + d.LatestAttempt.ID
		gap = "the provider stopped the previous execution because of an access limit; its failed route is cooling down"
		notice = "Kapasitas AI sedang disesuaikan. Pekerjaan tersimpan dan akan dilanjutkan otomatis."
	}
	revision := "Gap: " + gap + "; completion and verification are not established. Area: this task's existing project workspace and original approved deliverables. Inspect and preserve existing files before making changes; do not rebuild in provider HOME or another workspace. Reconcile uncertain external effects before repeating them. Continue only unfinished approved work. Verify every original acceptance criterion with targeted checks and report actual evidence and user-accessible results. Finish only when the original scope and verification are complete." + taskRecoveryContext(d)
	output := s.startAuthorizedLaunch(ctx, origin, d, delegationMessageID(key), origin.RequestID, revision)
	// The launch path revalidates approval, cancellation, scope, ownership and
	// active execution, and persists an idempotent launch before starting work.
	now := time.Now().UTC()
	message := &db.ProjectMessage{ID: delegationMessageID(key + ":notice"), RoomID: d.SourceRoomID, AuthorType: "system", AuthorName: "Cuan", Content: notice + "\n\n" + output, CreatedAt: now, UpdatedAt: now}
	if s.projectStudioRepo.CreateMessage(ctx, message) == nil {
		s.publishInbox(d.SourceRoomID)
	}
	saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, delegationMessageID(key)))
	return err == nil && saved != nil && (saved.State == "queued" || saved.State == "claimed" || saved.State == "started")
}

// Infrastructure failures do not spend the business correction budget, but
// still need their own finite recovery bound.
func taskInfrastructureAttemptCount(d *task.TaskDetailResponse) int {
	if d == nil {
		return 0
	}
	count := 0
	seenLatest := false
	for _, a := range d.Attempts {
		if a == nil {
			continue
		}
		if d.LatestAttempt != nil && (a == d.LatestAttempt || (a.ID != "" && a.ID == d.LatestAttempt.ID)) {
			seenLatest = true
		}
		if a.Status != task.AttemptStatusSucceeded && a.Status != task.AttemptStatusCanceled && !a.CancelRequested && a.CanceledAt == nil {
			exit := 0
			if a.ExitCode != nil {
				exit = *a.ExitCode
			}
			if runner.IsInfraFault(a.Status, a.ErrorMessage, exit) || diagnosedStorageInterruption(d, a) {
				count++
			}
		}
	}
	if !seenLatest && d.LatestAttempt != nil && d.LatestAttempt.Status == task.AttemptStatusInterrupted && !d.LatestAttempt.CancelRequested && d.LatestAttempt.CanceledAt == nil {
		count++
	}
	return count
}

// Cancellation already requires a fresh authenticated user instruction. It is
// not an automated correction attempt and must not permanently exhaust that
// user's ability to continue the original task after resolving a blocker.
// Runner connection and infrastructure disconnects (interrupted, lease expired,
// heartbeat lost, runner killed/restarted) are also technical faults and MUST NOT
// consume the 3x business revision quota.
func taskRevisionAttemptCount(d *task.TaskDetailResponse) int {
	if d == nil {
		return 0
	}
	count := 0
	for _, a := range d.Attempts {
		if a == nil {
			continue
		}
		if a.Status == task.AttemptStatusCanceled || a.CancelRequested || a.CanceledAt != nil {
			continue
		}
		// Intentional PTY shutdown after a recorded success is a work attempt,
		// not an infrastructure failure merely because its signal code is137.
		if a.Status == task.AttemptStatusSucceeded {
			count++
			continue
		}
		var exitCode int
		if a.ExitCode != nil {
			exitCode = *a.ExitCode
		}
		if strings.HasPrefix(a.ErrorMessage, "The AI provider reported a usage limit.") {
			continue
		}
		if a.Status == task.AttemptStatusInterrupted || runner.IsInfraFault(a.Status, a.ErrorMessage, exitCode) || diagnosedStorageInterruption(d, a) {
			continue
		}
		count++
	}
	return count
}

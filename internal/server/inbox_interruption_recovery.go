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
	if d != nil && d.LatestAttempt != nil {
		if proof, err := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err != nil || proof != nil {
			return false // A supplemental verification has no automatic execution replay.
		}
		if proof, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err != nil || proof != nil {
			return false // Live inspections use their own exact-result recovery contract.
		}
	}
	if (!interruptedInboxTaskCanResume(d) && !providerLimitedInboxTaskCanResume(d)) || origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) || !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") {
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
	var deadline []time.Time
	// Repair only the proven pre-attempt phase-approval failure. The rejected
	// launch stays stopped with its original source; a deterministic successor
	// retains the exact saved revision and expiry and revalidates real approval.
	if saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, delegationMessageID(key))); err == nil && saved != nil && saved.State == "stopped" && saved.SafeError == "The original user authorization is unavailable or cancelled." && saved.SourceRequestID == origin.RequestID && saved.OwnerID == origin.OwnerID && saved.RoomID == origin.RoomID && saved.ProjectID == d.ProjectID && saved.InstructionsHash == task.LaunchInstructionsHash(d.Instructions) && saved.ExpiresAt.After(time.Now()) {
		source, e := s.projectStudioRepo.GetMessageByID(ctx, origin.MessageID)
		if e != nil || source == nil || source.AuthorType != "system" || source.AuthorID != "cuan_auto_delegation" {
			return false
		}
		approval, e := s.phaseApproval(ctx, d, origin.OwnerID, origin.RoomID)
		if e != nil || approval.Request.RequestID == saved.SourceRequestID {
			return false
		}
		key += ":phase-approval:v1"
		revision = saved.Revision
		deadline = append(deadline, saved.ExpiresAt)
	}
	output := s.startAuthorizedLaunch(ctx, origin, d, delegationMessageID(key), origin.RequestID, revision, deadline...)
	// The launch path revalidates approval, cancellation, scope, ownership and
	// active execution, and persists an idempotent launch before starting work.
	_ = output
	// Deliver system update / auto-resume notices as transient toast events
	// instead of persisting chat bubbles in the conversation timeline.
	s.recordSystemNotice(d.SourceRoomID, delegationMessageID(key+":notice"), notice)
	s.publishInbox(d.SourceRoomID)
	saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, delegationMessageID(key)))
	return err == nil && saved != nil && (saved.State == "queued" || saved.State == "claimed" || saved.State == "started")
}

// SystemNoticeEvent represents a transient system lifecycle notice delivered
// to active UI clients as a temporary toast rather than a persisted chat bubble.
type SystemNoticeEvent struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

const systemNoticeTTL = 60 * time.Second

func (s *Server) recordSystemNotice(roomID, id, message string) {
	if s == nil || roomID == "" || id == "" || strings.TrimSpace(message) == "" {
		return
	}
	now := time.Now().UTC()
	cutoff := now.Add(-systemNoticeTTL)
	notice := SystemNoticeEvent{
		ID:        id,
		Message:   message,
		CreatedAt: now,
	}

	s.systemNoticesMu.Lock()
	if s.systemNotices == nil {
		s.systemNotices = make(map[string][]SystemNoticeEvent)
	}
	existing := s.systemNotices[roomID]
	kept := make([]SystemNoticeEvent, 0, len(existing)+1)
	duplicate := false
	for _, item := range existing {
		if item.CreatedAt.Before(cutoff) {
			continue
		}
		if item.ID == id {
			duplicate = true
		}
		kept = append(kept, item)
	}
	if !duplicate {
		kept = append(kept, notice)
	}
	s.systemNotices[roomID] = kept
	s.systemNoticesMu.Unlock()

	if !duplicate && s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(roomID, "system.notice", notice)
	}
}

func (s *Server) activeSystemNotices(roomID string, now time.Time) []SystemNoticeEvent {
	if s == nil || roomID == "" {
		return nil
	}
	cutoff := now.Add(-systemNoticeTTL)
	s.systemNoticesMu.Lock()
	defer s.systemNoticesMu.Unlock()
	if s.systemNotices == nil {
		return nil
	}
	existing := s.systemNotices[roomID]
	if len(existing) == 0 {
		return nil
	}
	kept := make([]SystemNoticeEvent, 0, len(existing))
	for _, item := range existing {
		if !item.CreatedAt.Before(cutoff) {
			kept = append(kept, item)
		}
	}
	if len(kept) == 0 {
		delete(s.systemNotices, roomID)
		return nil
	}
	s.systemNotices[roomID] = kept
	out := make([]SystemNoticeEvent, len(kept))
	copy(out, kept)
	return out
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

// Persisted legacy retries already bind their predecessor in their deterministic
// request identity. Recognize that identity instead of parsing model prose.
func automaticRecoverySource(d *task.TaskDetailResponse, requestID string) *task.TaskAttempt {
	if d == nil || requestID == "" {
		return nil
	}
	attempts := append([]*task.TaskAttempt{}, d.Attempts...)
	attempts = append(attempts, d.LatestAttempt)
	for _, a := range attempts {
		if a == nil || a.ID == "" {
			continue
		}
		for _, kind := range []string{"task-interruption-recovery:", "task-storage-recovery:", "task-provider-recovery:"} {
			if requestID == delegationMessageID(kind+a.ID) || requestID == delegationMessageID(kind+a.ID+":phase-approval:v1") {
				return a
			}
		}
	}
	return nil
}

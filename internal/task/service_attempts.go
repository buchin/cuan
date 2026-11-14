package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) CreateAttempt(ctx context.Context, taskID string, author string, req CreateAttemptRequest) (*TaskAttempt, error) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	var launchIsTypedContinuation bool
	if req.RequestID != "" {
		if existing, err := s.FindRequestAttempt(ctx, taskID, req.RequestID, req.RequestHash); err != nil {
			return nil, err
		} else if existing != nil {
			return existing, nil
		}
		if reader, ok := s.store.(LaunchStore); ok {
			launch, err := reader.GetLaunch(ctx, LaunchID(t.ID, req.RequestID))
			if err != nil && !errors.Is(err, ErrTaskNotFound) {
				return nil, err
			}
			if launch != nil {
				release, err := NativeReleaseIntentForLaunch(t, launch)
				if err != nil {
					return nil, err
				}
				if release != nil && req.ExpectedPredecessorID != release.PredecessorAttemptID() {
					return nil, ErrRequestConflict
				}
				if release != nil {
					launchIsTypedContinuation = true
				}
				if v, _ := VerificationIntentForLaunch(t, launch); v != nil {
					launchIsTypedContinuation = true
				}
				if i, _ := LiveInspectionForLaunch(t, launch); i != nil {
					launchIsTypedContinuation = true
				}
			}
		}
	}
	if t.Status == TaskStatusDone {
		return nil, fmt.Errorf("%w: reopen the completed task before starting a new session", ErrInvalidTaskInput)
	}
	// One shared attempt budget governs automatic work retries. Two kinds of call
	// are not automatic repeats: typed native continuations (verification,
	// inspection, release handoff) are new stages of an approved workflow, and an
	// owner-authorized continuation carries fresh consent from a later user turn.
	typedContinuation := req.ExpectedPredecessorID != "" || req.InitializeQueuedTask || launchIsTypedContinuation
	if !typedContinuation && !req.OwnerAuthorized && author != "owner" {
		if attempts, listErr := s.store.ListAttempts(ctx, taskID); listErr == nil && len(attempts) > 0 {
			business, latest, previous := businessAttempts(attempts)
			if business >= MaxAttemptsFor() {
				return nil, fmt.Errorf("%w: task reached its %d-attempt limit; retry explicitly with a reason to continue", ErrInvalidTaskInput, MaxAttemptsFor())
			}
			if latest != nil && !RetryWorthy(latest.ErrorMessage) && latest.ErrorMessage != "" {
				return nil, fmt.Errorf("%w: the previous attempt stopped on an unattended cause (%s); fix it instead of retrying", ErrInvalidTaskInput, ClassifyAttemptFailure(latest.ErrorMessage))
			}
			if RepeatedFailure(latest, previous) {
				return nil, fmt.Errorf("%w: the previous attempts failed with the same reason; resolve it instead of retrying", ErrInvalidTaskInput)
			}
		}
	}
	runnerID := strings.TrimSpace(req.RunnerID)
	if runnerID == "" {
		runnerID = "cuan-runner"
	}

	agentName := strings.TrimSpace(req.AgentName)
	if agentName == "" {
		agentName = t.AssignedAgent
		if agentName == "" {
			agentName = "agy"
		}
	}

	att := &TaskAttempt{
		ID:           attemptRequestID(t.ID, req.RequestID),
		RequestHash:  req.RequestHash,
		TaskID:       t.ID,
		RunnerID:     runnerID,
		AgentName:    agentName,
		Status:       AttemptStatusQueued,
		PeakRSSBytes: 0,
		CreatedAt:    time.Now().UTC(),
	}

	if runnerID == "local_terminal" {
		lease := time.Now().UTC().Add(11 * time.Minute)
		att.LeaseExpiresAt = &lease
	}
	var createErr error
	if req.InitializeQueuedTask && runnerID != "cuan-runner" {
		return nil, ErrInvalidTaskInput
	}
	if req.ExpectedPredecessorID != "" {
		// Successful predecessors require an authenticated typed native
		// continuation, never a generic implementation retry.
		if reader, ok := s.store.(LaunchStore); ok && req.RequestID != "" {
			launch, launchErr := reader.GetLaunch(ctx, LaunchID(t.ID, req.RequestID))
			if launchErr == nil && launch != nil {
				proof, proofErr := LiveInspectionForLaunch(t, launch)
				if proofErr != nil {
					return nil, proofErr
				}
				if proof != nil {
					if proof.PredecessorAttemptID() != req.ExpectedPredecessorID {
						return nil, ErrRequestConflict
					}
					if err := ValidateLiveInspectionSource(ctx, s.store, t, proof); err != nil {
						return nil, err
					}
					ctx = context.WithValue(ctx, inspectionAdmissionKey{}, inspectionAdmission{LaunchID: launch.ID, Revision: launch.Revision, Predecessor: req.ExpectedPredecessorID})
				}
				release, releaseErr := NativeReleaseIntentForLaunch(t, launch)
				if releaseErr != nil {
					return nil, releaseErr
				}
				if release != nil {
					if release.PredecessorAttemptID() != req.ExpectedPredecessorID {
						return nil, ErrRequestConflict
					}
					if err := ValidateNativeReleaseSource(ctx, s.store, t, release); err != nil {
						return nil, err
					}
					ctx = context.WithValue(ctx, inspectionAdmissionKey{}, inspectionAdmission{LaunchID: launch.ID, Revision: launch.Revision, Predecessor: req.ExpectedPredecessorID})
				}
			}
		}
		if queued, ok := s.store.(interface {
			CreateQueuedAttempt(context.Context, *TaskAttempt, string, bool) error
		}); req.InitializeQueuedTask && ok {
			createErr = queued.CreateQueuedAttempt(ctx, att, req.ExpectedPredecessorID, !req.PreserveQueuedReport)
		} else if req.InitializeQueuedTask {
			createErr = ErrInvalidTaskInput
		} else if guarded, ok := s.store.(interface {
			CreateAttemptAfter(context.Context, *TaskAttempt, string) error
		}); ok {
			createErr = guarded.CreateAttemptAfter(ctx, att, req.ExpectedPredecessorID)
		} else {
			createErr = ErrRequestConflict
		}
	} else {
		if req.InitializeQueuedTask {
			if queued, ok := s.store.(interface {
				CreateQueuedAttempt(context.Context, *TaskAttempt, string, bool) error
			}); ok {
				createErr = queued.CreateQueuedAttempt(ctx, att, "", !req.PreserveQueuedReport)
			} else {
				createErr = ErrInvalidTaskInput
			}
		} else {
			createErr = s.store.CreateAttempt(ctx, att)
		}
	}
	if createErr != nil {
		return nil, fmt.Errorf("create attempt: %w", createErr)
	}

	// Record attempt_transition activity
	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       t.ID,
		AttemptID:    &att.ID,
		ActivityType: ActivityTypeAttempt,
		Author:       author,
		NewStatus:    att.Status,
		Message:      fmt.Sprintf("Attempt %s queued (agent: %s, runner: %s).", att.ID, att.AgentName, att.RunnerID),
		CreatedAt:    att.CreatedAt,
	}
	_ = s.store.CreateActivity(ctx, act)

	return att, nil
}

func (s *Service) UpdateAttemptStatus(ctx context.Context, attemptID string, author string, req UpdateAttemptRequest) (*TaskAttempt, error) {
	return s.updateAttemptStatus(ctx, attemptID, author, req, false)
}

// FinishReportedAttempt is an internal runner action, never a public status
// override. A validated final report intentionally closes an interactive PTY;
// its actual signal exit code is retained without claiming a provider failure.
func (s *Service) FinishReportedAttempt(ctx context.Context, attemptID string, exitCode int) (*TaskAttempt, error) {
	return s.updateAttemptStatus(ctx, attemptID, "runner", UpdateAttemptRequest{Status: AttemptStatusSucceeded, ExitCode: &exitCode}, true)
}

func (s *Service) updateAttemptStatus(ctx context.Context, attemptID string, author string, req UpdateAttemptRequest, reportedStop bool) (*TaskAttempt, error) {
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	att, err := s.store.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	if reportedStop {
		latest, e := s.store.GetLatestAttempt(ctx, att.TaskID)
		if e != nil {
			return nil, e
		}
		t, e := s.store.GetTask(ctx, att.TaskID)
		if e != nil {
			return nil, e
		}
		acts, e := s.store.ListActivities(ctx, att.TaskID)
		if e != nil {
			return nil, e
		}
		var final *TaskActivity
		for _, a := range acts {
			if a.AttemptID != nil && *a.AttemptID == attemptID && a.ActivityType == "agent_report" && (final == nil || a.CreatedAt.After(final.CreatedAt)) {
				final = a
			}
		}
		if latest.ID != attemptID || final == nil || (final.NewStatus != "review" && final.NewStatus != "blocked" && final.NewStatus != "done") || t.Summary == "" || t.CompleteNote == "" {
			return nil, ErrRequestConflict
		}
	}
	oldStatus := att.Status
	if att.Status != "queued" && att.Status != "running" {
		return att, nil
	}
	newStatus := strings.ToLower(strings.TrimSpace(req.Status))

	// CRITICAL DoD Rule:
	// "Kegagalan attempt tidak ditampilkan sebagai sukses. Agent tidak otomatis menganggap hasil accepted hanya karena exit code nol."
	if req.ExitCode != nil && *req.ExitCode != 0 && !reportedStop {
		newStatus = AttemptStatusFailed
	}
	if newStatus == AttemptStatusFailed && req.ExitCode == nil {
		exitFail := 1
		att.ExitCode = &exitFail
	} else if req.ExitCode != nil {
		att.ExitCode = req.ExitCode
	}

	switch newStatus {
	case AttemptStatusQueued, AttemptStatusRunning, AttemptStatusSucceeded, AttemptStatusFailed, AttemptStatusCanceled, AttemptStatusInterrupted:
		att.Status = newStatus
	default:
		return nil, fmt.Errorf("%w: invalid attempt status '%s'", ErrInvalidTaskInput, newStatus)
	}

	now := time.Now().UTC()
	if newStatus == AttemptStatusRunning && att.StartedAt == nil {
		att.StartedAt = &now
		if t, e := s.store.GetTask(ctx, att.TaskID); e == nil {
			deadline := now.Add(CalculateTaskTimeout(t.ExecutionCategory, att.AttemptNumber, t.Title, t.Instructions))
			att.ExecutionDeadline = &deadline
		}
	}
	if (newStatus == AttemptStatusSucceeded || newStatus == AttemptStatusFailed || newStatus == AttemptStatusCanceled || newStatus == AttemptStatusInterrupted) && att.FinishedAt == nil {
		att.FinishedAt = &now
	}

	if req.PeakRSSBytes != nil {
		att.PeakRSSBytes = *req.PeakRSSBytes
	}

	if req.ErrorMessage != "" {
		att.ErrorMessage = req.ErrorMessage
	}

	if err := s.store.UpdateAttempt(ctx, att); err != nil {
		if errors.Is(err, ErrAttemptFinal) {
			return s.store.GetAttempt(ctx, attemptID)
		}
		return nil, fmt.Errorf("update attempt: %w", err)
	}

	if newStatus == AttemptStatusSucceeded || newStatus == AttemptStatusFailed || newStatus == AttemptStatusCanceled || newStatus == AttemptStatusInterrupted {
		_ = s.FinishTaskLaunches(ctx, att.TaskID, "stopped")
	}

	// Audit activity log
	msg := fmt.Sprintf("Attempt %s changed state: '%s' → '%s'", att.ID, oldStatus, newStatus)
	if reportedStop {
		msg += " (interactive session closed after validated agent report)"
	}
	if att.ExitCode != nil {
		msg += fmt.Sprintf(" (exit code: %d)", *att.ExitCode)
	}
	if att.ErrorMessage != "" {
		msg += fmt.Sprintf(" [Message: %s]", att.ErrorMessage)
	}

	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       att.TaskID,
		AttemptID:    &att.ID,
		ActivityType: ActivityTypeAttempt,
		Author:       author,
		OldStatus:    oldStatus,
		NewStatus:    newStatus,
		Message:      msg,
		CreatedAt:    now,
	}
	_ = s.store.CreateActivity(ctx, act)

	return att, nil
}

// CancelTask marks the active attempt for a task as canceled and triggers process cancellation.
func (s *Service) CancelTask(ctx context.Context, taskID string, author string) (*TaskAttempt, error) {
	s.attemptMu.Lock()
	var transitionEvent *Task
	var previousStatus string
	defer func() {
		s.attemptMu.Unlock()
		if transitionEvent != nil {
			s.extensionTransition(ctx, previousStatus, transitionEvent)
		}
	}()
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	latest, err := s.store.GetLatestAttempt(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, fmt.Errorf("%w: no attempt is available to cancel", ErrAttemptNotFound)
	}

	if latest.Status != AttemptStatusRunning && latest.Status != AttemptStatusQueued {
		return nil, fmt.Errorf("%w: cannot cancel an attempt with status '%s'", ErrInvalidTaskInput, latest.Status)
	}

	now := time.Now().UTC()
	latest.CancelRequested = true
	latest.Status = AttemptStatusCanceled
	latest.CanceledAt = &now
	latest.FinishedAt = &now
	exitCanceled := 130 // SIGINT exit code standard
	latest.ExitCode = &exitCanceled
	latest.ErrorMessage = fmt.Sprintf("Cancelled by %s", author)

	if err := s.store.UpdateAttempt(ctx, latest); err != nil {
		return nil, fmt.Errorf("update cancel attempt: %w", err)
	}

	// Cancellation is an owner decision, not a defect in the work. Sending the
	// task back to the queue keeps it runnable, which is what "cancel" means to
	// the person who asked; only a genuine failure may block a task.
	oldTaskStatus := t.Status
	t.Status = TaskStatusBacklog
	t.UpdatedAt = now
	_ = s.FinishTaskLaunches(ctx, taskID, "stopped")
	if err := s.store.UpdateTask(ctx, t); err == nil {
		snapshot := *t
		transitionEvent, previousStatus = &snapshot, oldTaskStatus
	}

	// Record activity
	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       taskID,
		AttemptID:    &latest.ID,
		ActivityType: ActivityTypeCancelRequested,
		Author:       author,
		OldStatus:    oldTaskStatus,
		NewStatus:    TaskStatusBacklog,
		Message:      fmt.Sprintf("Cancellation of attempt %s requested by %s. The task is back in the queue and can run again.", latest.ID, author),
		CreatedAt:    now,
	}
	_ = s.store.CreateActivity(ctx, act)

	return latest, nil
}

// RetryTask creates a brand-new execution attempt for a failed, canceled, or interrupted task.
func (s *Service) RetryTask(ctx context.Context, taskID string, author string, runnerID string, agentName string, reason string) (*TaskAttempt, error) {
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	latest, err := s.store.GetLatestAttempt(ctx, taskID)
	if err != nil {
		return nil, err
	}

	nextAttemptNum := 1
	if latest != nil {
		if latest.Status == AttemptStatusRunning || latest.Status == AttemptStatusQueued {
			return nil, fmt.Errorf("%w: task is running or already queued", ErrInvalidTaskInput)
		}
		attempts, _ := s.store.ListAttempts(ctx, taskID)
		business, _, _ := businessAttempts(attempts)
		nextAttemptNum = business + 1
	}

	// Retries cannot silently exceed the shared attempt budget. An owner may
	// still force one past the limit, but only with a recorded reason, so the
	// task ledger shows why the ceiling was crossed instead of looping quietly.
	if latest != nil && !CanRequeue(nextAttemptNum-1) && strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("%w: task reached its %d-attempt limit; supply a reason to retry beyond it", ErrInvalidTaskInput, MaxAttemptsFor())
	}

	if runnerID == "" {
		runnerID = "runner_default"
	}
	if agentName == "" {
		agentName = t.AssignedAgent
		if agentName == "" {
			agentName = "agy"
		}
	}

	now := time.Now().UTC()
	newAttempt := &TaskAttempt{
		ID:            generateID("att"),
		TaskID:        taskID,
		RunnerID:      runnerID,
		AgentName:     agentName,
		Status:        AttemptStatusQueued,
		AttemptNumber: nextAttemptNum,
		CreatedAt:     now,
	}

	if err := s.store.CreateAttempt(ctx, newAttempt); err != nil {
		return nil, fmt.Errorf("create retry attempt: %w", err)
	}

	// Update task status back to doing
	t.Status = TaskStatusDoing
	t.UpdatedAt = now
	_ = s.store.UpdateTask(ctx, t)

	// Record activity
	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       taskID,
		AttemptID:    &newAttempt.ID,
		ActivityType: ActivityTypeRetried,
		Author:       author,
		NewStatus:    AttemptStatusQueued,
		Message:      fmt.Sprintf("Attempt %s explicitly retried (attempt #%d) by %s.", newAttempt.ID, nextAttemptNum, author),
		CreatedAt:    now,
	}
	if strings.TrimSpace(reason) != "" {
		act.Message = fmt.Sprintf("Attempt %s explicitly retried (attempt #%d) by %s. Reason: %s", newAttempt.ID, nextAttemptNum, author, strings.TrimSpace(reason))
	}
	_ = s.store.CreateActivity(ctx, act)

	return newAttempt, nil
}

func (s *Service) AppendAttemptLogs(ctx context.Context, attemptID string, logs []LogRecord, maxLines int) error {
	return s.store.AppendLogs(ctx, attemptID, logs, maxLines)
}

func (s *Service) GetAttemptLogs(ctx context.Context, attemptID string, afterID int64, limit int) ([]*LogRecord, error) {
	return s.store.GetLogs(ctx, attemptID, afterID, limit)
}

func (s *Service) GetRecentAttemptLogs(ctx context.Context, attemptID string, limit int) ([]*LogRecord, error) {
	reader, ok := s.store.(RecentLogStore)
	if !ok {
		return nil, fmt.Errorf("recent attempt log reads unavailable")
	}
	return reader.GetRecentLogs(ctx, attemptID, limit)
}

// RenewLocalAttemptLease keeps a known live local PTY valid without extending
// leases for a crashed process, another attempt or an already-final result.
func (s *Service) RenewLocalAttemptLease(ctx context.Context, attemptID string) error {
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	a, err := s.store.GetAttempt(ctx, attemptID)
	if err != nil {
		return err
	}
	if a.RunnerID != "local_terminal" || a.Status != AttemptStatusRunning {
		return nil
	}
	latest, err := s.store.GetLatestAttempt(ctx, a.TaskID)
	if err != nil {
		return err
	}
	if latest.ID != a.ID {
		return nil
	}
	now := time.Now().UTC()
	if a.LeaseExpiresAt != nil && a.LeaseExpiresAt.After(now.Add(10*time.Minute)) {
		return nil
	}
	lease := now.Add(11 * time.Minute)
	a.LeaseExpiresAt = &lease
	return s.store.UpdateAttempt(ctx, a)
}

// RecoverStartupLocalAttempts recovers any in-flight local_terminal attempts
// left behind by a server restart or crash without waiting for lease expiration,
// since local terminal PTY processes cannot survive across server restarts.
func (s *Service) RecoverStartupLocalAttempts(ctx context.Context) error {
	tasks, err := s.RecoveryTasks(ctx)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		a, err := s.store.GetLatestAttempt(ctx, t.ID)
		if err != nil || a == nil || a.RunnerID != "local_terminal" {
			continue
		}
		if a.Status == AttemptStatusRunning || a.Status == AttemptStatusQueued {
			if _, err = s.UpdateAttemptStatus(ctx, a.ID, "recovery", UpdateAttemptRequest{
				Status:       AttemptStatusInterrupted,
				ErrorMessage: "Execution was interrupted during server restart.",
			}); err != nil {
				return err
			}
			if err = s.ReconcileLocalTask(ctx, t.ID, a.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// RecoverLocalAttempts exposes an unknown result after the local runner lease
// expires. It never launches the previous attempt again.
func (s *Service) RecoverLocalAttempts(ctx context.Context) error {
	tasks, err := s.RecoveryTasks(ctx)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		a, err := s.store.GetLatestAttempt(ctx, t.ID)
		if err != nil || a == nil || a.RunnerID != "local_terminal" {
			continue
		}
		if a.Status != "running" && a.Status != "queued" {
			if t.Status == TaskStatusDoing {
				if err = s.ReconcileLocalTask(ctx, t.ID, a.ID); err != nil {
					return err
				}
			}
			continue
		}
		if a.LeaseExpiresAt != nil && time.Now().Before(*a.LeaseExpiresAt) {
			continue
		}
		if _, err = s.UpdateAttemptStatus(ctx, a.ID, "recovery", UpdateAttemptRequest{Status: AttemptStatusInterrupted, ErrorMessage: "The local session was interrupted or its lease expired. The result is unknown. It will not be rerun automatically."}); err != nil {
			return err
		}
		if err = s.ReconcileLocalTask(ctx, t.ID, a.ID); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileZombieAttempts sweeps task attempts and launches whose OS process
// is dead or has exceeded the hard execution timeout. It marks them failed / stopped
// and frees the project lane so subsequent tasks can run automatically.
func (s *Service) ReconcileZombieAttempts(ctx context.Context, timeout time.Duration, isTaskActiveOnHost func(taskID string) bool) error {
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	tasks, err := s.RecoveryTasks(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()

	for _, t := range tasks {
		a, err := s.store.GetLatestAttempt(ctx, t.ID)
		if err != nil || a == nil {
			continue
		}

		if a.Status != "running" && a.Status != "queued" {
			_ = s.ReconcileLocalTask(ctx, t.ID, a.ID)
			continue
		}

		// Queue age is not execution time. Claiming the attempt assigns its
		// immutable execution deadline; queued work has no process to reap.
		if a.Status == AttemptStatusQueued {
			continue
		}
		deadline := AttemptDeadline(t, a)
		exceededTimeout := now.After(deadline.Add(GetGracePeriod()))

		// Check 2: OS process is dead (only for local_terminal attempts currently running;
		// decoupled runner daemon attempts are tracked via lease heartbeats, not local terminalManager)
		processDead := false
		if a.Status == "running" && (a.RunnerID == "local_terminal" || a.RunnerID == "") {
			if isTaskActiveOnHost != nil && !isTaskActiveOnHost(t.ID) {
				processDead = true
			}
		}

		if exceededTimeout || processDead {
			msg := "Task runner process terminated or inactive (zombie attempt reaped)."
			if exceededTimeout {
				msg = fmt.Sprintf("Task execution deadline exceeded (%s).", deadline.UTC().Format(time.RFC3339))
			}

			expected := t.UpdatedAt
			a.Status = AttemptStatusFailed
			a.ErrorMessage = msg
			a.FinishedAt = &now
			exitCode := 124
			a.ExitCode = &exitCode
			t.Status = TaskStatusBlocked
			t.UpdatedAt = now
			completion, ok := s.store.(CompletionStore)
			if !ok {
				continue
			}
			if err := completion.CompleteExecution(ctx, a, t, nil, expected, a.RunnerID, false); err != nil {
				continue
			}
			_ = s.store.CreateActivity(ctx, &TaskActivity{ID: generateID("act"), TaskID: t.ID, AttemptID: &a.ID, ActivityType: ActivityTypeAttempt, Author: "zombie_reaper", NewStatus: TaskStatusBlocked, Message: msg, CreatedAt: now})

			// Mark task launches stopped
			_ = s.FinishTaskLaunches(ctx, t.ID, "stopped")
		}
	}

	return nil
}

func attemptRequestID(taskID, requestID string) string {
	if requestID == "" {
		return generateID("att")
	}
	sum := sha256.Sum256([]byte(taskID + "\x00" + strings.ToLower(requestID)))
	return "att_" + hex.EncodeToString(sum[:16])
}

// FindRequestAttempt supports durable retries, including after a fast process exit
// or server restart. A replay returns recorded state and never launches work.
func (s *Service) FindRequestAttempt(ctx context.Context, taskID, requestID, hash string) (*TaskAttempt, error) {
	if requestID == "" {
		return nil, nil
	}
	if !validRequestUUID(requestID) {
		return nil, fmt.Errorf("%w: request_id must be a UUID", ErrInvalidTaskInput)
	}
	att, err := s.store.GetAttempt(ctx, attemptRequestID(taskID, requestID))
	if errors.Is(err, ErrAttemptNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if att.TaskID != taskID || att.RequestHash != hash {
		return nil, ErrRequestConflict
	}
	return att, nil
}

func validRequestUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	raw := strings.ReplaceAll(value, "-", "")
	if len(raw) != 32 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

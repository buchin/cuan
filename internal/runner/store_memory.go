package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/task"
)

type MemoryStore struct {
	mu        sync.Mutex
	runners   map[string]*RunnerInfo
	taskStore task.Store
}

func NewMemoryStore(taskStore task.Store) *MemoryStore {
	return &MemoryStore{
		runners:   make(map[string]*RunnerInfo),
		taskStore: taskStore,
	}
}

func (m *MemoryStore) RegisterOrUpdateHeartbeat(ctx context.Context, info *RunnerInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.runners[info.ID]
	now := time.Now().UTC()
	if !exists {
		info.CreatedAt = now
		info.UpdatedAt = now
		if info.LastHeartbeatAt.IsZero() {
			info.LastHeartbeatAt = now
		}
		if info.Status == "" {
			info.Status = StatusIdle
		}
		cp := *info
		m.runners[info.ID] = &cp
		return nil
	}

	existing.RuntimeEvidence = info.RuntimeEvidence
	existing.Hostname = info.Hostname
	if len(info.SupportedAgents) > 0 {
		existing.SupportedAgents = info.SupportedAgents
	}
	existing.Status = info.Status
	if info.LastHeartbeatAt.IsZero() {
		existing.LastHeartbeatAt = now
	} else {
		existing.LastHeartbeatAt = info.LastHeartbeatAt
	}
	existing.CurrentTaskID = info.CurrentTaskID
	existing.CurrentAttemptID = info.CurrentAttemptID
	existing.CPUPercent = info.CPUPercent
	existing.MemoryUsedBytes = info.MemoryUsedBytes
	existing.MemoryTotalBytes = info.MemoryTotalBytes
	existing.DiskUsedBytes = info.DiskUsedBytes
	existing.DiskTotalBytes = info.DiskTotalBytes
	existing.UpdatedAt = now

	return nil
}

func (m *MemoryStore) GetRunner(ctx context.Context, id string) (*RunnerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, exists := m.runners[id]
	if !exists {
		return nil, ErrRunnerNotFound
	}
	cp := *r
	return &cp, nil
}

func (m *MemoryStore) ListRunners(ctx context.Context) ([]*RunnerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var list []*RunnerInfo
	now := time.Now().UTC()
	for _, r := range m.runners {
		cp := *r
		// Mark runner as offline if last heartbeat > 30s ago
		if now.Sub(cp.LastHeartbeatAt) > 30*time.Second {
			cp.Status = StatusOffline
		}
		list = append(list, &cp)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list, nil
}

func (m *MemoryStore) ClaimNextQueuedAttempt(ctx context.Context, runnerID string, supportedAgents []string, leaseDuration time.Duration, availableLanes ...[]string) (*task.TaskAttempt, *task.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	// Check if this runner already has a running attempt
	runner, exists := m.runners[runnerID]
	if !exists {
		runner = &RunnerInfo{
			ID:              runnerID,
			Name:            runnerID,
			Status:          StatusIdle,
			LastHeartbeatAt: now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		m.runners[runnerID] = runner
	}

	tasks, err := m.taskStore.ListTasks(ctx, task.TaskFilter{})
	if err != nil {
		return nil, nil, err
	}

	// 1. Calculate active executions per lane for this runner
	activeCodeCount := 0
	activeOpsCount := 0
	activeReadCount := 0

	for _, t := range tasks {
		attempts, err := m.taskStore.ListAttempts(ctx, t.ID)
		if err != nil {
			continue
		}
		tLane := t.ExecutionCategory
		if tLane == "" {
			tLane = task.ExecutionCategoryCode
		}
		for _, a := range attempts {
			if a.RunnerID == runnerID && a.Status == task.AttemptStatusRunning {
				switch tLane {
				case task.ExecutionCategoryCode:
					activeCodeCount++
				case task.ExecutionCategoryOps:
					activeOpsCount++
				case task.ExecutionCategoryReadOnly:
					activeReadCount++
				}
			}
		}
	}

	canClaimCode := activeCodeCount == 0
	canClaimOps := activeOpsCount == 0
	canClaimRead := activeReadCount < 4

	// If caller specified availableLanes, filter by that
	if len(availableLanes) > 0 && len(availableLanes[0]) > 0 {
		requested := make(map[string]bool)
		for _, l := range availableLanes[0] {
			requested[task.NormalizeExecutionCategory(l)] = true
		}
		canClaimCode = canClaimCode && requested[task.ExecutionCategoryCode]
		canClaimOps = canClaimOps && requested[task.ExecutionCategoryOps]
		canClaimRead = canClaimRead && requested[task.ExecutionCategoryReadOnly]
	}

	if !canClaimCode && !canClaimOps && !canClaimRead {
		return nil, nil, nil
	}

	// 2. Find the oldest queued attempt matching lane and project lock rules
	var oldestQueued *task.TaskAttempt
	var matchedTask *task.Task

	for _, t := range tasks {
		candidateLane := t.ExecutionCategory
		if candidateLane == "" {
			candidateLane = task.ExecutionCategoryCode
		}

		switch candidateLane {
		case task.ExecutionCategoryCode:
			if !canClaimCode {
				continue
			}
		case task.ExecutionCategoryOps:
			if !canClaimOps {
				continue
			}
		case task.ExecutionCategoryReadOnly:
			if !canClaimRead {
				continue
			}
		}

		// Project Concurrency Lock separation:
		// - Git deployment lock: serializes code tasks
		// - Operational lock: serializes ops tasks
		// - Shared reader pool: permits concurrent readonly tasks
		// - Schema migration: exclusive barrier blocking all lanes
		if t.ProjectID != "" {
			projectBusy := false
			isCandidateMigration := task.IsTaskSchemaMigration(t)

			for _, otherTask := range tasks {
				if otherTask.ID != t.ID && (otherTask.ProjectID == t.ProjectID || containsString(otherTask.ProjectIDs, t.ProjectID)) {
					isOtherActive := false
					if otherTask.Status == task.TaskStatusDoing || otherTask.Status == "working" {
						isOtherActive = true
					} else {
						otherAttempts, _ := m.taskStore.ListAttempts(ctx, otherTask.ID)
						for _, oa := range otherAttempts {
							if oa.Status == task.AttemptStatusRunning {
								isOtherActive = true
								break
							}
						}
					}
					if !isOtherActive {
						continue
					}

					otherLane := otherTask.ExecutionCategory
					if otherLane == "" {
						otherLane = task.ExecutionCategoryCode
					}

					// Active schema migration blocks ALL lanes
					if task.IsTaskSchemaMigration(otherTask) {
						projectBusy = true
						break
					}

					// If candidate is schema migration, any active task blocks it
					if isCandidateMigration {
						projectBusy = true
						break
					}

					// Git deployment lock: code tasks cannot run concurrently with other code tasks
					if candidateLane == task.ExecutionCategoryCode && otherLane == task.ExecutionCategoryCode {
						projectBusy = true
						break
					}

					// Operational lock: ops tasks cannot run concurrently with other ops tasks
					if candidateLane == task.ExecutionCategoryOps && otherLane == task.ExecutionCategoryOps {
						projectBusy = true
						break
					}
				}
			}
			if projectBusy {
				continue
			}
		}

		attempts, err := m.taskStore.ListAttempts(ctx, t.ID)
		if err != nil {
			continue
		}
		for _, a := range attempts {
			if a.Status == task.AttemptStatusQueued {
				isNativeRelease := false
				if reader, ok := m.taskStore.(task.AttemptLaunchStore); ok {
					launch, err := reader.LaunchForAttempt(ctx, a.ID)
					if err != nil {
						return nil, nil, err
					}
					if launch != nil && strings.Contains(launch.Revision, "Cuan native-release intent") {
						if ctx.Value(nativeReleaseClaimKey{}) != true {
							continue
						}
						proof, err := task.NativeReleaseIntentForLaunch(t, launch)
						if err != nil {
							return nil, nil, err
						}
						isNativeRelease = proof != nil
					}
				}
				if ctx.Value(verificationFinalizationClaimKey{}) != true || ctx.Value(dependencyPreparationClaimKey{}) != true {
					proof, err := task.VerificationIntentForAttempt(ctx, m.taskStore, t, a.ID)
					if err != nil {
						return nil, nil, err
					}
					if proof != nil && ((proof.FinalizeAttemptID != "" && ctx.Value(verificationFinalizationClaimKey{}) != true) || (proof.RecoveryCause == "php_preparation" && ctx.Value(dependencyPreparationClaimKey{}) != true)) {
						continue
					}
				}
				if ctx.Value(liveInspectionClaimKey{}) != true || ctx.Value(liveInspectionRecoveryClaimKey{}) != true || ctx.Value(currentOutcomeClaimKey{}) != true || ctx.Value(evidenceRevalidationClaimKey{}) != true {
					proof, err := task.LiveInspectionForAttempt(ctx, m.taskStore, t, a.ID)
					if err != nil {
						return nil, nil, err
					}
					if proof != nil && (ctx.Value(liveInspectionClaimKey{}) != true || (proof.PreviousAttemptID != "" && ctx.Value(liveInspectionRecoveryClaimKey{}) != true) || (proof.Purpose == task.CurrentOutcomeInspection && ctx.Value(currentOutcomeClaimKey{}) != true) || (proof.RevalidationKind == task.EvidenceRevalidation && ctx.Value(evidenceRevalidationClaimKey{}) != true)) {
						continue
					}
				}

				// Check agent compatibility if specified
				targetAgent := a.AgentName
				if targetAgent == "" {
					targetAgent = t.AssignedAgent
					if targetAgent == "" {
						targetAgent = "agy"
					}
				}
				if len(supportedAgents) > 0 && !isNativeRelease {
					supported := false
					for _, sa := range supportedAgents {
						if strings.EqualFold(sa, targetAgent) {
							supported = true
							break
						}
					}
					if !supported {
						continue
					}
				}

				if oldestQueued == nil || a.CreatedAt.Before(oldestQueued.CreatedAt) {
					oldestQueued = a
					matchedTask = t
				}
			}
		}
	}

	if oldestQueued == nil {
		return nil, nil, nil
	}

	var revision string
	if launches, ok := m.taskStore.(task.AttemptLaunchStore); ok {
		launch, err := launches.LaunchForAttempt(ctx, oldestQueued.ID)
		if err != nil {
			return nil, nil, err
		}
		if launch != nil {
			revision = launch.Revision
		}
	}
	// Atomically claim the attempt
	leaseExpires := now.Add(leaseDuration)
	oldestQueued.Status = task.AttemptStatusRunning
	oldestQueued.RunnerID = runnerID
	oldestQueued.StartedAt = &now
	oldestQueued.LeaseExpiresAt = &leaseExpires
	deadline := claimExecutionDeadline(matchedTask, oldestQueued, now, revision)
	oldestQueued.ExecutionDeadline = &deadline

	if err := m.taskStore.UpdateAttempt(ctx, oldestQueued); err != nil {
		return nil, nil, err
	}

	// Update task status to doing
	matchedTask.Status = task.TaskStatusDoing
	matchedTask.UpdatedAt = now
	_ = m.taskStore.UpdateTask(ctx, matchedTask)

	// Update runner status to busy
	runner.Status = StatusBusy
	runner.CurrentTaskID = matchedTask.ID
	runner.CurrentAttemptID = oldestQueued.ID
	runner.UpdatedAt = now

	// Log activity
	act := &task.TaskActivity{
		ID:           fmt.Sprintf("act_%d", time.Now().UnixNano()),
		TaskID:       matchedTask.ID,
		AttemptID:    &oldestQueued.ID,
		ActivityType: task.ActivityTypeAttempt,
		Author:       runnerID,
		OldStatus:    task.AttemptStatusQueued,
		NewStatus:    task.AttemptStatusRunning,
		Message:      fmt.Sprintf("Attempt %s diklaim secara eksklusif oleh %s.", oldestQueued.ID, runnerID),
		CreatedAt:    now,
	}
	_ = m.taskStore.CreateActivity(ctx, act)

	return oldestQueued, matchedTask, nil
}

func (m *MemoryStore) RenewLease(ctx context.Context, attemptID, runnerID string, leaseDuration time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	att, err := m.taskStore.GetAttempt(ctx, attemptID)
	if err != nil {
		return false, err
	}

	if att.RunnerID != runnerID || att.Status != task.AttemptStatusRunning || (att.LeaseExpiresAt != nil && !att.LeaseExpiresAt.After(time.Now())) {
		return false, task.ErrAttemptFinal
	}
	if att.CancelRequested || att.Status == task.AttemptStatusCanceled {
		return true, nil
	}

	now := time.Now().UTC()
	leaseExpires := now.Add(leaseDuration)
	att.LeaseExpiresAt = &leaseExpires
	if err := m.taskStore.UpdateAttempt(ctx, att); err != nil {
		return false, err
	}

	if r, exists := m.runners[runnerID]; exists {
		r.LastHeartbeatAt = now
		r.UpdatedAt = now
	}

	return false, nil
}

func (m *MemoryStore) MarkInterruptedLeases(ctx context.Context) ([]*task.TaskAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	var interrupted []*task.TaskAttempt

	tasks, err := m.taskStore.ListTasks(ctx, task.TaskFilter{})
	if err != nil {
		return nil, err
	}

	for _, t := range tasks {
		attempts, err := m.taskStore.ListAttempts(ctx, t.ID)
		if err != nil {
			continue
		}
		for _, a := range attempts {
			if a.Status == task.AttemptStatusRunning && a.LeaseExpiresAt != nil && a.LeaseExpiresAt.Before(now) {
				if t.Status == task.TaskStatusDone {
					if retire, ok := m.taskStore.(task.AcceptedAttemptRetirer); ok {
						retired, err := retire.RetireExpiredAcceptedAttempt(ctx, a.ID, a.RunnerID)
						if err != nil {
							return nil, err
						}
						if retired {
							a.Status = task.AttemptStatusInterrupted
							a.ErrorMessage = task.AcceptedTaskLeaseCleanup
							a.FinishedAt = &now
							a.InterruptedAt = &now
							interrupted = append(interrupted, a)
						}
					}
					continue
				}
				// Expired lease! Mark interrupted and recoverable
				a.Status = task.AttemptStatusInterrupted
				a.FinishedAt = &now
				a.InterruptedAt = &now
				exitErr := 137
				a.ExitCode = &exitErr
				a.ErrorMessage = "Runner heartbeat lost and lease expired; marked interrupted and recoverable without auto-rerun."

				expected := t.UpdatedAt
				t.Status = task.TaskStatusBlocked
				t.UpdatedAt = now
				completion, ok := m.taskStore.(task.CompletionStore)
				if !ok {
					continue
				}
				if err := completion.CompleteExecution(ctx, a, t, nil, expected, a.RunnerID, false); err != nil {
					continue
				}

				if ls, ok := m.taskStore.(task.LaunchStore); ok {
					_ = ls.FinishTaskLaunches(ctx, t.ID, "stopped")
				}

				// Update runner
				if r, exists := m.runners[a.RunnerID]; exists && r.CurrentAttemptID == a.ID {
					r.Status = StatusOffline
					r.CurrentAttemptID = ""
					r.CurrentTaskID = ""
					r.UpdatedAt = now
				}

				// Log activity
				act := &task.TaskActivity{
					ID:           fmt.Sprintf("act_%d", time.Now().UnixNano()),
					TaskID:       t.ID,
					AttemptID:    &a.ID,
					ActivityType: task.ActivityTypeInterrupted,
					Author:       "system",
					OldStatus:    task.AttemptStatusRunning,
					NewStatus:    task.AttemptStatusInterrupted,
					Message:      fmt.Sprintf("Attempt %s interrupted (lease expired). Task is in status '%s' and can be retried explicitly.", a.ID, t.Status),
					CreatedAt:    now,
				}
				_ = m.taskStore.CreateActivity(ctx, act)

				interrupted = append(interrupted, a)
			}
		}
	}

	return interrupted, nil
}

func containsString(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func (m *MemoryStore) ReleaseAssignment(_ context.Context, runnerID, attemptID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.runners[runnerID]; ok && r.CurrentAttemptID == attemptID {
		r.Status = StatusIdle
		r.CurrentTaskID = ""
		r.CurrentAttemptID = ""
		r.UpdatedAt = time.Now().UTC()
	}
	return nil
}

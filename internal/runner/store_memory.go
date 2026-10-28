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
		info.LastHeartbeatAt = now
		if info.Status == "" {
			info.Status = StatusIdle
		}
		cp := *info
		m.runners[info.ID] = &cp
		return nil
	}

	existing.Hostname = info.Hostname
	if len(info.SupportedAgents) > 0 {
		existing.SupportedAgents = info.SupportedAgents
	}
	existing.Status = info.Status
	existing.LastHeartbeatAt = now
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

func (m *MemoryStore) ClaimNextQueuedAttempt(ctx context.Context, runnerID string, supportedAgents []string, leaseDuration time.Duration) (*task.TaskAttempt, *task.Task, error) {
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

	// CONCURRENCY LIMIT: 1 task aktif per runner
	tasks, err := m.taskStore.ListTasks(ctx, task.TaskFilter{})
	if err != nil {
		return nil, nil, err
	}

	for _, t := range tasks {
		attempts, err := m.taskStore.ListAttempts(ctx, t.ID)
		if err != nil {
			continue
		}
		for _, a := range attempts {
			if a.RunnerID == runnerID && a.Status == task.AttemptStatusRunning {
				// Runner already has an active running attempt!
				return nil, nil, nil
			}
		}
	}

	// Find the oldest queued attempt
	var oldestQueued *task.TaskAttempt
	var matchedTask *task.Task

	for _, t := range tasks {
		// Strict Project Concurrency Lock: only 1 active runner per project
		if t.ProjectID != "" {
			projectBusy := false
			for _, otherTask := range tasks {
				if otherTask.ID != t.ID && (otherTask.ProjectID == t.ProjectID || containsString(otherTask.ProjectIDs, t.ProjectID)) {
					if otherTask.Status == task.TaskStatusDoing || otherTask.Status == "working" {
						projectBusy = true
						break
					}
					otherAttempts, _ := m.taskStore.ListAttempts(ctx, otherTask.ID)
					for _, oa := range otherAttempts {
						if oa.Status == task.AttemptStatusRunning {
							projectBusy = true
							break
						}
					}
					if projectBusy {
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
				// Check agent compatibility if specified
				targetAgent := a.AgentName
				if targetAgent == "" {
					targetAgent = t.AssignedAgent
					if targetAgent == "" {
						targetAgent = "agy"
					}
				}
				if len(supportedAgents) > 0 {
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

	// Atomically claim the attempt
	leaseExpires := now.Add(leaseDuration)
	oldestQueued.Status = task.AttemptStatusRunning
	oldestQueued.RunnerID = runnerID
	oldestQueued.StartedAt = &now
	oldestQueued.LeaseExpiresAt = &leaseExpires

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
				// Expired lease! Mark interrupted and recoverable
				a.Status = task.AttemptStatusInterrupted
				a.FinishedAt = &now
				a.InterruptedAt = &now
				exitErr := 137
				a.ExitCode = &exitErr
				a.ErrorMessage = "Runner heartbeat lost and lease expired; marked interrupted and recoverable without auto-rerun."

				_ = m.taskStore.UpdateAttempt(ctx, a)

				// Update task to review/blocked (recoverable)
				t.Status = task.TaskStatusReview
				t.UpdatedAt = now
				_ = m.taskStore.UpdateTask(ctx, t)
				if ls, ok := m.taskStore.(task.LaunchStore); ok {
					_ = ls.FinishTaskLaunches(ctx, t.ID, "stopped")
				}

				// Update runner
				if r, exists := m.runners[a.RunnerID]; exists {
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

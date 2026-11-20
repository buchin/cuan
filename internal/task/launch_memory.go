package task

import (
	"context"
	"sort"
	"strings"
	"time"
)

func copyLaunch(j *TaskLaunch) *TaskLaunch {
	if j == nil {
		return nil
	}
	c := *j
	if j.LeaseUntil != nil {
		v := *j.LeaseUntil
		c.LeaseUntil = &v
	}
	return &c
}
func (m *MemoryStore) SaveLaunch(_ context.Context, j TaskLaunch) (*TaskLaunch, error) {
	j, e := normalizeLaunch(j)
	if e != nil {
		return nil, e
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if old := m.launches[j.ID]; old != nil {
		if !sameLaunch(*old, j) {
			return nil, ErrRequestConflict
		}
		return copyLaunch(old), nil
	}
	if m.tasks[j.TaskID] == nil {
		return nil, ErrTaskNotFound
	}
	m.launches[j.ID] = copyLaunch(&j)
	return copyLaunch(&j), nil
}
func (m *MemoryStore) ClaimLaunch(_ context.Context, now time.Time, token string, wanted ...string) (*TaskLaunch, error) {
	if token == "" {
		return nil, ErrInvalidTaskInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := []string{}
	for id, j := range m.launches {
		if len(wanted) > 0 && wanted[0] != "" && id != wanted[0] {
			continue
		}
		if t := m.tasks[j.TaskID]; t != nil && t.Status == TaskStatusQueued && (j.State == "queued" || j.State == "claimed") && !j.ExpiresAt.After(now) {
			j.ExpiresAt = now.Add(24 * time.Hour)
			j.UpdatedAt = now
		}
		if (j.State == "queued" || j.State == "claimed") && !j.ExpiresAt.After(now) {
			j.State = "stopped"
			j.SafeError = "The launch window expired before execution could start."
			j.UpdatedAt = now
			j.LeaseToken = ""
			j.LeaseUntil = nil
			continue
		}
		if t := m.tasks[j.TaskID]; t != nil && (t.Status == TaskStatusDone || t.Status == "cancelled" || t.Status == "failed") {
			if j.State == "queued" || j.State == "claimed" || j.State == "started" {
				if len(wanted) == 0 {
					j.State = "stopped"
					j.SafeError = "Task is in terminal status."
					j.UpdatedAt = now
					j.LeaseToken = ""
					j.LeaseUntil = nil
					continue
				}
			}
		}
		if j.State == "queued" && !j.AvailableAt.After(now) || j.State == "claimed" && j.LeaseUntil != nil && !j.LeaseUntil.After(now) {
			projID := j.ProjectID
			if projID == "" {
				if t := m.tasks[j.TaskID]; t != nil {
					projID = t.ProjectID
				}
			}
			if projID != "" {
				candLane := ExecutionCategoryCode
				if t := m.tasks[j.TaskID]; t != nil && t.ExecutionCategory != "" {
					candLane = NormalizeExecutionCategory(t.ExecutionCategory)
				}
				projectBusy := false
				for _, other := range m.tasks {
					if other.ID == j.TaskID {
						continue
					}
					if other.ProjectID == projID || containsString(other.ProjectIDs, projID) {
						if strings.HasPrefix(other.Title, "Terminal · ") {
							continue
						}
						if candLane == ExecutionCategoryCode && WorkflowPrecedes(other, m.tasks[j.TaskID]) && m.workflowHandoffLocked(other, now) {
							projectBusy = true
							break
						}
						if other.Status == TaskStatusDone || other.Status == TaskStatusBlocked || other.Status == TaskStatusReview || other.Status == "failed" || other.Status == "cancelled" || other.Status == TaskStatusQueued {
							continue
						}
						taskActive := (other.Status == TaskStatusDoing || other.Status == "working")
						if !taskActive {
							for _, a := range m.attempts {
								if a.TaskID == other.ID && a.Status == AttemptStatusRunning {
									taskActive = true
									break
								}
							}
						}
						if !taskActive {
							for _, l := range m.launches {
								if l.TaskID == other.ID && l.State == "started" {
									taskActive = true
									break
								}
							}
						}
						if !taskActive {
							continue
						}
						if IsTaskSchemaMigration(other) {
							projectBusy = true
							break
						}
						if candLane == ExecutionCategoryReadOnly {
							continue
						}
						otherLane := other.ExecutionCategory
						if otherLane == "" {
							otherLane = ExecutionCategoryCode
						}
						if NormalizeExecutionCategory(otherLane) == candLane {
							projectBusy = true
							break
						}
					}
				}
				if projectBusy {
					continue
				}
			}
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, k int) bool {
		a, b := m.launches[ids[i]], m.launches[ids[k]]
		if a.AvailableAt.Equal(b.AvailableAt) {
			return a.ID < b.ID
		}
		return a.AvailableAt.Before(b.AvailableAt)
	})
	if len(ids) == 0 {
		return nil, nil
	}
	j := m.launches[ids[0]]
	until := now.Add(time.Minute)
	j.State = "claimed"
	j.LeaseUntil = &until
	j.LeaseToken = token
	j.Checks++
	j.UpdatedAt = now
	return copyLaunch(j), nil
}
func (m *MemoryStore) FinishLaunch(_ context.Context, id, token, state string, next time.Time, safeError string) error {
	if !validLaunchFinish(state) {
		return ErrInvalidTaskInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.launches[id]
	if j == nil || j.State != "claimed" || j.LeaseToken != token || j.LeaseUntil == nil || !j.LeaseUntil.After(time.Now()) {
		return ErrLaunchLease
	}
	j.State = state
	j.AvailableAt = next
	j.SafeError = safeError
	j.UpdatedAt = time.Now().UTC()
	j.LeaseToken = ""
	j.LeaseUntil = nil
	if state == "stopped" {
		latest := true
		for _, other := range m.launches {
			if other.TaskID == j.TaskID && (other.CreatedAt.After(j.CreatedAt) || (other.CreatedAt.Equal(j.CreatedAt) && other.ID > j.ID)) {
				latest = false
			}
		}
		for _, attempt := range m.attempts {
			if attempt.TaskID == j.TaskID && (attempt.Status == AttemptStatusQueued || attempt.Status == AttemptStatusRunning) {
				latest = false
			}
		}
		if t := m.tasks[j.TaskID]; latest && t != nil && t.Status == TaskStatusQueued {
			t.Status = TaskStatusBlocked
			t.UpdatedAt = j.UpdatedAt
		}
	}
	return nil
}
func (m *MemoryStore) FinishTaskLaunches(_ context.Context, taskID string, finalState string) error {
	if !validLaunchFinish(finalState) && finalState != "cancelled" {
		finalState = "stopped"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for _, j := range m.launches {
		if j.TaskID == taskID && (j.State == "queued" || j.State == "claimed" || j.State == "started") {
			j.State = finalState
			j.LeaseToken = ""
			j.LeaseUntil = nil
			j.UpdatedAt = now
		}
	}
	return nil
}

func (m *MemoryStore) CancelLaunches(_ context.Context, taskID string, now time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, j := range m.launches {
		if j.TaskID == taskID && (j.State == "queued" || j.State == "claimed") {
			j.State = "cancelled"
			j.LeaseToken = ""
			j.LeaseUntil = nil
			j.UpdatedAt = now
			count++
		}
	}
	return count, nil
}
func (m *MemoryStore) GetLaunch(_ context.Context, id string) (*TaskLaunch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j := m.launches[id]
	if j == nil {
		return nil, ErrTaskNotFound
	}
	return copyLaunch(j), nil
}

func (m *MemoryStore) LatestLaunch(_ context.Context, taskID string) (*TaskLaunch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest *TaskLaunch
	for _, j := range m.launches {
		if j.TaskID == taskID && (latest == nil || j.CreatedAt.After(latest.CreatedAt) || (j.CreatedAt.Equal(latest.CreatedAt) && j.ID > latest.ID)) {
			latest = j
		}
	}
	return copyLaunch(latest), nil
}

func (m *MemoryStore) ReconcileOrphanLaunches(_ context.Context, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.launches {
		if a := m.attempts[attemptRequestID(j.TaskID, j.RequestID)]; a != nil && a.TaskID == j.TaskID && (a.Status == AttemptStatusSucceeded || a.Status == AttemptStatusFailed || a.Status == AttemptStatusCanceled || a.Status == AttemptStatusInterrupted) && (j.State == "queued" || j.State == "claimed" || j.State == "started") {
			j.State, j.SafeError, j.LeaseToken, j.LeaseUntil, j.UpdatedAt = "stopped", "Recorded execution is terminal; launch reconciled.", "", nil, now
		}
		t := m.tasks[j.TaskID]
		if t != nil && (t.Status == TaskStatusDone || t.Status == "cancelled" || t.Status == "failed") {
			if j.State == "queued" || j.State == "claimed" || j.State == "started" {
				j.State = "stopped"
				j.SafeError = "Task is in terminal status."
				j.UpdatedAt = now
				j.LeaseToken = ""
				j.LeaseUntil = nil
			}
		}
		// Self-heal orphan ghost-queued tasks: if task is queued but launch was stopped (e.g. restart, terminal status false alarm, or expired window), revive it
		if t != nil && t.Status == TaskStatusQueued && j.State == "stopped" && recoverableStoppedLaunch(j.SafeError) {
			j.State = "queued"
			j.SafeError = ""
			j.LeaseToken = ""
			j.LeaseUntil = nil
			j.AvailableAt = now
			if j.ExpiresAt.Before(now.Add(24 * time.Hour)) {
				j.ExpiresAt = now.Add(24 * time.Hour)
			}
			j.UpdatedAt = now
		}
		// Auto-cancel abandoned interactive terminal tasks queued or blocked > 5 min
		if t != nil && (t.Status == TaskStatusQueued || t.Status == TaskStatusBlocked || t.Status == TaskStatusBacklog) && strings.HasPrefix(t.Title, "Terminal · ") && (t.CreatedAt.Before(now.Add(-5*time.Minute)) || t.UpdatedAt.Before(now.Add(-5*time.Minute))) {
			t.Status = "cancelled"
			t.UpdatedAt = now
			if j.State == "queued" || j.State == "claimed" || j.State == "started" {
				j.State = "stopped"
				j.SafeError = "Interactive terminal session expired while waiting in queue."
				j.UpdatedAt = now
				j.LeaseToken = ""
				j.LeaseUntil = nil
			}
		}
	}
	for _, t := range m.tasks {
		var latest *TaskAttempt
		active := false
		for _, a := range m.attempts {
			if a.TaskID != t.ID {
				continue
			}
			if a.Status == AttemptStatusQueued || a.Status == AttemptStatusRunning {
				active = true
			}
			if latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID) {
				latest = a
			}
		}
		var pending *TaskLaunch
		for _, j := range m.launches {
			if j.TaskID == t.ID && (pending == nil || j.CreatedAt.After(pending.CreatedAt) || (j.CreatedAt.Equal(pending.CreatedAt) && j.ID > pending.ID)) {
				pending = j
			}
		}
		if active || !pendingCapacityContinuation(t, pending, latest, now) {
			continue
		}
		paused := false
		for _, a := range m.activities {
			if a.TaskID == t.ID && a.ActivityType == ActivityTypeStatusChange && a.NewStatus == TaskStatusBlocked && a.CreatedAt.After(*latest.FinishedAt) {
				paused = true
				break
			}
		}
		if paused {
			continue
		}
		t.Status, t.UpdatedAt = TaskStatusQueued, now
		event := &TaskActivity{ID: generateID("act"), TaskID: t.ID, AttemptID: &latest.ID, ActivityType: "execution_recovery", Author: "coordinator", NewStatus: TaskStatusQueued, Message: "Restored the existing approved pending launch after an earlier capacity failure. Execution history and recovery budgets are unchanged.", CreatedAt: now}
		m.activities[event.ID] = event
	}

	return nil
}

func (m *MemoryStore) ReconcileStartupLaunches(_ context.Context, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.launches {
		t := m.tasks[j.TaskID]
		a := m.attempts[attemptRequestID(j.TaskID, j.RequestID)]
		if t != nil && t.Status != TaskStatusDone && t.Status != "cancelled" && t.Status != "canceled" && t.Status != "failed" && a != nil && a.TaskID == j.TaskID && !a.CancelRequested && a.CanceledAt == nil && (a.Status == AttemptStatusQueued || a.Status == AttemptStatusRunning) {
			if j.State == "queued" || j.State == "claimed" || j.State == "started" || (j.State == "stopped" && (j.SafeError == "Execution stopped during server restart." || j.SafeError == "Execution attempt is no longer active (orphan launch reconciled)." || j.SafeError == "No active runner process detected.")) {
				j.State, j.SafeError, j.LeaseToken, j.LeaseUntil, j.UpdatedAt = "started", "", "", nil, now
			}
			continue
		}
		if t != nil && t.Status == TaskStatusQueued {
			if j.State == "started" || j.State == "claimed" || (j.State == "stopped" && recoverableStoppedLaunch(j.SafeError)) {
				j.State = "queued"
				j.SafeError = ""
				j.LeaseToken = ""
				j.LeaseUntil = nil
				j.AvailableAt = now
				if j.ExpiresAt.Before(now.Add(24 * time.Hour)) {
					j.ExpiresAt = now.Add(24 * time.Hour)
				}
				j.UpdatedAt = now
				continue
			}
		}
		if j.State == "started" || j.State == "claimed" {
			j.State = "stopped"
			j.SafeError = "Execution stopped during server restart."
			j.UpdatedAt = now
			j.LeaseToken = ""
			j.LeaseUntil = nil
		}
	}
	return nil
}

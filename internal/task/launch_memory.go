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
		if (j.State == "queued" || j.State == "claimed") && !j.ExpiresAt.After(now) {
			j.State = "stopped"
			j.SafeError = "The launch window expired before execution could start."
			j.UpdatedAt = now
			j.LeaseToken = ""
			j.LeaseUntil = nil
			continue
		}
		if t := m.tasks[j.TaskID]; t != nil && (t.Status == TaskStatusDone || t.Status == "cancelled" || t.Status == "failed" || t.Status == TaskStatusBlocked || t.Status == TaskStatusReview) {
			if j.State == "queued" || j.State == "claimed" || j.State == "started" {
				j.State = "stopped"
				j.SafeError = "Task is in terminal status."
				j.UpdatedAt = now
				j.LeaseToken = ""
				j.LeaseUntil = nil
				continue
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
				projectBusy := false
				for _, other := range m.launches {
					if other.ID == id {
						continue
					}
					otherProj := other.ProjectID
					if otherProj == "" {
						if ot := m.tasks[other.TaskID]; ot != nil {
							otherProj = ot.ProjectID
						}
					}
					if otherProj == projID {
						ot := m.tasks[other.TaskID]
						if ot == nil || (ot.Status != TaskStatusDone && ot.Status != "cancelled" && ot.Status != "failed" && ot.Status != TaskStatusBlocked && ot.Status != TaskStatusReview) {
							if other.State == "started" || (other.State == "claimed" && other.LeaseUntil != nil && other.LeaseUntil.After(now)) {
								projectBusy = true
								break
							}
						}
					}
				}
				if !projectBusy {
					for _, ot := range m.tasks {
						if ot.ID == j.TaskID {
							continue
						}
						if (ot.ProjectID == projID || containsString(ot.ProjectIDs, projID)) && (ot.Status == TaskStatusDoing || ot.Status == "working") {
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
		t := m.tasks[j.TaskID]
		if t != nil && (t.Status == TaskStatusDone || t.Status == "cancelled" || t.Status == "failed" || t.Status == TaskStatusBlocked || t.Status == TaskStatusReview) {
			if j.State == "queued" || j.State == "claimed" || j.State == "started" {
				j.State = "stopped"
				j.SafeError = "Task is in terminal status."
				j.UpdatedAt = now
				j.LeaseToken = ""
				j.LeaseUntil = nil
			}
		}
		// Auto-cancel abandoned interactive terminal tasks queued > 5 min
		if t != nil && t.Status == TaskStatusQueued && strings.HasPrefix(t.Title, "Terminal · ") && t.CreatedAt.Before(now.Add(-5*time.Minute)) {
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
	return nil
}

func (m *MemoryStore) ReconcileStartupLaunches(_ context.Context, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.launches {
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

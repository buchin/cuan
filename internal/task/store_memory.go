package task

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu         sync.RWMutex
	tasks      map[string]*Task
	attempts   map[string]*TaskAttempt
	activities map[string]*TaskActivity
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks:      make(map[string]*Task),
		attempts:   make(map[string]*TaskAttempt),
		activities: make(map[string]*TaskActivity),
	}
}

func (m *MemoryStore) CreateTask(ctx context.Context, t *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := *t
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	cp.UpdatedAt = cp.CreatedAt
	m.tasks[cp.ID] = &cp
	return nil
}

func (m *MemoryStore) GetTask(ctx context.Context, id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	cp := *t
	return &cp, nil
}

func (m *MemoryStore) ListTasks(ctx context.Context, filter TaskFilter) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Task
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))

	for _, t := range m.tasks {
		if filter.ProjectID != "" && t.ProjectID != filter.ProjectID {
			continue
		}
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if searchLower != "" {
			titleMatch := strings.Contains(strings.ToLower(t.Title), searchLower)
			instrMatch := strings.Contains(strings.ToLower(t.Instructions), searchLower)
			if !titleMatch && !instrMatch {
				continue
			}
		}
		cp := *t
		result = append(result, &cp)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (m *MemoryStore) UpdateTask(ctx context.Context, t *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.tasks[t.ID]
	if !exists {
		return ErrTaskNotFound
	}

	cp := *t
	cp.CreatedAt = existing.CreatedAt
	cp.UpdatedAt = time.Now().UTC()
	m.tasks[t.ID] = &cp
	return nil
}

func (m *MemoryStore) DeleteTask(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[id]; !exists {
		return ErrTaskNotFound
	}

	delete(m.tasks, id)

	// Cascade delete attempts and activities
	for attID, att := range m.attempts {
		if att.TaskID == id {
			delete(m.attempts, attID)
		}
	}
	for actID, act := range m.activities {
		if act.TaskID == id {
			delete(m.activities, actID)
		}
	}

	return nil
}

func (m *MemoryStore) CreateAttempt(ctx context.Context, a *TaskAttempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := *a
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	m.attempts[cp.ID] = &cp
	return nil
}

func (m *MemoryStore) GetAttempt(ctx context.Context, id string) (*TaskAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, exists := m.attempts[id]
	if !exists {
		return nil, ErrAttemptNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *MemoryStore) GetLatestAttempt(ctx context.Context, taskID string) (*TaskAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *TaskAttempt
	for _, a := range m.attempts {
		if a.TaskID == taskID {
			if latest == nil || a.CreatedAt.After(latest.CreatedAt) {
				latest = a
			}
		}
	}

	if latest == nil {
		return nil, nil
	}
	cp := *latest
	return &cp, nil
}

func (m *MemoryStore) ListAttempts(ctx context.Context, taskID string) ([]*TaskAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*TaskAttempt
	for _, a := range m.attempts {
		if a.TaskID == taskID {
			cp := *a
			result = append(result, &cp)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (m *MemoryStore) UpdateAttempt(ctx context.Context, a *TaskAttempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.attempts[a.ID]
	if !exists {
		return ErrAttemptNotFound
	}

	cp := *a
	cp.CreatedAt = existing.CreatedAt
	m.attempts[a.ID] = &cp
	return nil
}

func (m *MemoryStore) CreateActivity(ctx context.Context, act *TaskActivity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := *act
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	m.activities[cp.ID] = &cp
	return nil
}

func (m *MemoryStore) ListActivities(ctx context.Context, taskID string) ([]*TaskActivity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*TaskActivity
	for _, act := range m.activities {
		if act.TaskID == taskID {
			cp := *act
			result = append(result, &cp)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt) // Chronological order
	})

	return result, nil
}

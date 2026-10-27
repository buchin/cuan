package deployer

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory thread-safe implementation of Store, useful for tests.
type MemoryStore struct {
	mu          sync.RWMutex
	deployments map[string]*Deployment
	slugMap     map[string]string // slug -> ID
	logs        map[string][]LogEntry
	nextLogID   int64
}

// NewMemoryStore creates a clean in-memory deployment store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		deployments: make(map[string]*Deployment),
		slugMap:     make(map[string]string),
		logs:        make(map[string][]LogEntry),
		nextLogID:   1,
	}
}

func (m *MemoryStore) CreateDeployment(ctx context.Context, dep *Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.deployments[dep.ID]; exists {
		return ErrSlugConflict
	}
	if _, exists := m.slugMap[dep.Slug]; exists {
		return ErrSlugConflict
	}

	now := time.Now().UTC()
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = now
	}
	dep.UpdatedAt = now

	cp := *dep
	m.deployments[dep.ID] = &cp
	m.slugMap[dep.Slug] = dep.ID
	return nil
}

func (m *MemoryStore) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dep, exists := m.deployments[id]
	if !exists {
		return nil, ErrDeploymentNotFound
	}
	cp := *dep
	return &cp, nil
}

func (m *MemoryStore) GetDeploymentBySlug(ctx context.Context, slug string) (*Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, exists := m.slugMap[slug]
	if !exists {
		return nil, ErrDeploymentNotFound
	}
	dep := m.deployments[id]
	cp := *dep
	return &cp, nil
}

func (m *MemoryStore) UpdateDeployment(ctx context.Context, dep *Deployment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.deployments[dep.ID]
	if !exists {
		return ErrDeploymentNotFound
	}

	// If slug changed, update mapping
	if existing.Slug != dep.Slug {
		if _, taken := m.slugMap[dep.Slug]; taken {
			return ErrSlugConflict
		}
		delete(m.slugMap, existing.Slug)
		m.slugMap[dep.Slug] = dep.ID
	}

	dep.UpdatedAt = time.Now().UTC()
	cp := *dep
	m.deployments[dep.ID] = &cp
	return nil
}

func (m *MemoryStore) UpdateDeploymentStatus(ctx context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dep, exists := m.deployments[id]
	if !exists {
		return ErrDeploymentNotFound
	}

	dep.Status = status
	dep.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *MemoryStore) ListDeployments(ctx context.Context) ([]Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []Deployment
	for _, dep := range m.deployments {
		list = append(list, *dep)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	return list, nil
}

func (m *MemoryStore) DeleteDeployment(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dep, exists := m.deployments[id]
	if !exists {
		return ErrDeploymentNotFound
	}

	delete(m.slugMap, dep.Slug)
	delete(m.deployments, id)
	delete(m.logs, id)
	return nil
}

func (m *MemoryStore) AppendLog(ctx context.Context, log *LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.deployments[log.DeploymentID]; !exists {
		return ErrDeploymentNotFound
	}

	log.ID = m.nextLogID
	m.nextLogID++
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}

	m.logs[log.DeploymentID] = append(m.logs[log.DeploymentID], *log)
	return nil
}

func (m *MemoryStore) GetLogs(ctx context.Context, deploymentID string, limit int) ([]LogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, exists := m.logs[deploymentID]
	if !exists {
		return []LogEntry{}, nil
	}

	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}

	// Return last 'limit' entries
	start := len(entries) - limit
	result := make([]LogEntry, limit)
	copy(result, entries[start:])
	return result, nil
}

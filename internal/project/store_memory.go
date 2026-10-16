package project

import (
	"context"
	"strings"
	"sync"
)

type MemoryStore struct {
	mu       sync.RWMutex
	projects map[string]*Project
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		projects: make(map[string]*Project),
	}
}

func (m *MemoryStore) CreateProject(ctx context.Context, p *Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.projects {
		if strings.EqualFold(existing.RepoURL, p.RepoURL) ||
			(strings.EqualFold(existing.RepoOwner, p.RepoOwner) && strings.EqualFold(existing.RepoName, p.RepoName)) {
			return ErrProjectAlreadyExists
		}
	}

	clone := *p
	m.projects[p.ID] = &clone
	return nil
}

func (m *MemoryStore) GetProject(ctx context.Context, id string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}

	clone := *p
	return &clone, nil
}

func (m *MemoryStore) GetProjectByRepo(ctx context.Context, owner, repo string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.projects {
		if strings.EqualFold(p.RepoOwner, owner) && strings.EqualFold(p.RepoName, repo) {
			clone := *p
			return &clone, nil
		}
	}

	return nil, ErrProjectNotFound
}

func (m *MemoryStore) ListProjects(ctx context.Context) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*Project
	for _, p := range m.projects {
		clone := *p
		list = append(list, &clone)
	}
	return list, nil
}

func (m *MemoryStore) UpdateProject(ctx context.Context, p *Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[p.ID]; !exists {
		return ErrProjectNotFound
	}

	clone := *p
	m.projects[p.ID] = &clone
	return nil
}

func (m *MemoryStore) DeleteProject(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[id]; !exists {
		return ErrProjectNotFound
	}

	delete(m.projects, id)
	return nil
}

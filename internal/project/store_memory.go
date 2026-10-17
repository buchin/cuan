package project

import (
	"context"
	"encoding/json"
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

func cloneMemoryProject(p *Project) *Project {
	if p == nil {
		return nil
	}
	cp := *p
	if cp.ProjectType == "" {
		cp.ProjectType = "custom"
	}
	if len(cp.Metadata) > 0 {
		cp.Metadata = make(json.RawMessage, len(p.Metadata))
		copy(cp.Metadata, p.Metadata)
	} else {
		cp.Metadata = json.RawMessage("{}")
	}
	if p.LastFetchedAt != nil {
		t := *p.LastFetchedAt
		cp.LastFetchedAt = &t
	}
	return &cp
}

func (m *MemoryStore) CreateProject(ctx context.Context, p *Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.projects {
		if strings.EqualFold(existing.RepoURL, p.RepoURL) ||
			(strings.EqualFold(existing.RepoOwner, p.RepoOwner) && strings.EqualFold(existing.RepoName, p.RepoName) && p.RepoOwner != "") {
			return ErrProjectAlreadyExists
		}
	}

	m.projects[p.ID] = cloneMemoryProject(p)
	return nil
}

func (m *MemoryStore) GetProject(ctx context.Context, id string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}

	return cloneMemoryProject(p), nil
}

func (m *MemoryStore) GetProjectByRepo(ctx context.Context, owner, repo string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.projects {
		if strings.EqualFold(p.RepoOwner, owner) && strings.EqualFold(p.RepoName, repo) {
			return cloneMemoryProject(p), nil
		}
	}

	return nil, ErrProjectNotFound
}

func (m *MemoryStore) ListProjects(ctx context.Context) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*Project
	for _, p := range m.projects {
		list = append(list, cloneMemoryProject(p))
	}
	return list, nil
}

func (m *MemoryStore) UpdateProject(ctx context.Context, p *Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[p.ID]; !exists {
		return ErrProjectNotFound
	}

	m.projects[p.ID] = cloneMemoryProject(p)
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

package project

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
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
	if len(cp.EnabledModules) > 0 {
		cp.EnabledModules = make(json.RawMessage, len(p.EnabledModules))
		copy(cp.EnabledModules, p.EnabledModules)
	} else {
		cp.EnabledModules = json.RawMessage(`["tasks", "discussions"]`)
	}
	if cp.SourceType == "" {
		cp.SourceType = "blank"
	}
	if len(cp.SourceMetadata) > 0 {
		cp.SourceMetadata = make(json.RawMessage, len(p.SourceMetadata))
		copy(cp.SourceMetadata, p.SourceMetadata)
	} else {
		cp.SourceMetadata = json.RawMessage("{}")
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

	if _, exists := m.projects[p.ID]; exists {
		return ErrProjectAlreadyExists
	}
	for _, existing := range m.projects {
		if (strings.TrimSpace(p.RepoURL) != "" && strings.EqualFold(existing.RepoURL, p.RepoURL)) ||
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
	sortProjects(list)
	return list, nil
}

// ListProjectsFiltered returns projects according to the archive filter,
// consistent with the DB store's behavior.
func (m *MemoryStore) ListProjectsFiltered(ctx context.Context, filter ArchivedFilter) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*Project
	for _, p := range m.projects {
		switch filter {
		case ArchivedFilterOnly:
			if p.IsArchived {
				list = append(list, cloneMemoryProject(p))
			}
		case ArchivedFilterAll:
			list = append(list, cloneMemoryProject(p))
		default: // ArchivedFilterActive
			if !p.IsArchived {
				list = append(list, cloneMemoryProject(p))
			}
		}
	}
	sortProjects(list)
	return list, nil
}

func sortProjects(list []*Project) {
	sort.Slice(list, func(i, j int) bool {
		if !list[i].UpdatedAt.Equal(list[j].UpdatedAt) {
			return list[i].UpdatedAt.After(list[j].UpdatedAt)
		}
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.After(list[j].CreatedAt)
		}
		return list[i].ID < list[j].ID
	})
}

func (m *MemoryStore) TouchProject(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.projects[id]
	if !exists {
		return ErrProjectNotFound
	}
	p.UpdatedAt = time.Now().UTC()
	return nil
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

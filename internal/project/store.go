package project

import (
	"context"
	"errors"
)

var (
	ErrProjectNotFound      = errors.New("project not found")
	ErrProjectAlreadyExists = errors.New("a project with this repository URL already exists")
	ErrInvalidProjectInput  = errors.New("invalid project input")
)

// ArchivedFilter controls which projects the store returns.
type ArchivedFilter int

const (
	// ArchivedFilterActive returns only non-archived projects (default).
	ArchivedFilterActive ArchivedFilter = iota
	// ArchivedFilterOnly returns only archived projects.
	ArchivedFilterOnly
	// ArchivedFilterAll returns all projects regardless of archive state.
	ArchivedFilterAll
)

type Store interface {
	CreateProject(ctx context.Context, p *Project) error
	GetProject(ctx context.Context, id string) (*Project, error)
	GetProjectByRepo(ctx context.Context, owner, repo string) (*Project, error)
	// ListProjects returns all projects without archive filtering (backward-compatible).
	ListProjects(ctx context.Context) ([]*Project, error)
	// ListProjectsFiltered returns projects according to the archive filter,
	// pushing the predicate to the database layer to avoid in-memory scanning.
	ListProjectsFiltered(ctx context.Context, filter ArchivedFilter) ([]*Project, error)
	UpdateProject(ctx context.Context, p *Project) error
	TouchProject(ctx context.Context, id string) error
	DeleteProject(ctx context.Context, id string) error
}

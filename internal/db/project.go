package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrProjectNotFound      = errors.New("project not found")
	ErrProjectAlreadyExists = errors.New("a project with this repository URL or name already exists")
	ErrEmptyProjectName     = errors.New("project name cannot be empty")
	ErrEmptyRepoURL         = errors.New("repository url cannot be empty")
)

// Project represents the core project entity in the database.
type Project struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	RepoURL        string          `json:"repo_url"`
	RepoOwner      string          `json:"repo_owner"`
	RepoName       string          `json:"repo_name"`
	DefaultBranch  string          `json:"default_branch"`
	LocalPath      string          `json:"local_path"`
	EncryptedToken string          `json:"-"` // Never serialized to JSON
	TokenPreview   string          `json:"token_preview"`
	CloneStatus    string          `json:"clone_status"`    // pending, connected, synced, failed
	ProjectType    string          `json:"project_type"`    // custom, blog, youtube, olshop, affiliate
	Metadata       json.RawMessage `json:"metadata"`        // blueprint config & workflow metadata
	EnabledModules json.RawMessage `json:"enabled_modules"` // e.g. ["tasks", "discussions"]
	SourceType     string          `json:"source_type"`     // blank, template, zip
	SourceMetadata json.RawMessage `json:"source_metadata"` // metadata for zip unpack, template source, etc.
	IsArchived     bool            `json:"is_archived"`
	LastFetchedAt  *time.Time      `json:"last_fetched_at,omitempty"`
	LastError      string          `json:"last_error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ProjectDTO is the public Data Transfer Object for Project.
type ProjectDTO struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	RepoURL        string          `json:"repo_url"`
	RepoOwner      string          `json:"repo_owner"`
	RepoName       string          `json:"repo_name"`
	DefaultBranch  string          `json:"default_branch"`
	LocalPath      string          `json:"local_path,omitempty"`
	HasToken       bool            `json:"has_token"`
	TokenPreview   string          `json:"token_preview,omitempty"`
	CloneStatus    string          `json:"clone_status"`
	ProjectType    string          `json:"project_type"`
	Metadata       json.RawMessage `json:"metadata"`
	EnabledModules json.RawMessage `json:"enabled_modules"`
	SourceType     string          `json:"source_type"`
	SourceMetadata json.RawMessage `json:"source_metadata"`
	IsArchived     bool            `json:"is_archived"`
	LastFetchedAt  *time.Time      `json:"last_fetched_at,omitempty"`
	LastError      string          `json:"last_error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// NormalizeProjectType normalizes project type string to lowercase, defaulting to "custom".
func NormalizeProjectType(pt string) string {
	norm := strings.ToLower(strings.TrimSpace(pt))
	if norm == "" {
		return "custom"
	}
	return norm
}

// NormalizeSourceType normalizes source type string to lowercase, defaulting to "blank".
func NormalizeSourceType(st string) string {
	norm := strings.ToLower(strings.TrimSpace(st))
	if norm == "" {
		return "blank"
	}
	return norm
}

// DefaultEnabledModules returns the standard default modules for any new workspace.
func DefaultEnabledModules() json.RawMessage {
	return json.RawMessage(`["tasks", "discussions"]`)
}

// ToDTO converts a Project entity to its sanitized public DTO.
func (p *Project) ToDTO() ProjectDTO {
	var metadataCopy json.RawMessage
	if len(p.Metadata) > 0 {
		metadataCopy = make(json.RawMessage, len(p.Metadata))
		copy(metadataCopy, p.Metadata)
	} else {
		metadataCopy = json.RawMessage("{}")
	}

	var modulesCopy json.RawMessage
	if len(p.EnabledModules) > 0 {
		modulesCopy = make(json.RawMessage, len(p.EnabledModules))
		copy(modulesCopy, p.EnabledModules)
	} else {
		modulesCopy = DefaultEnabledModules()
	}

	var sourceMetaCopy json.RawMessage
	if len(p.SourceMetadata) > 0 {
		sourceMetaCopy = make(json.RawMessage, len(p.SourceMetadata))
		copy(sourceMetaCopy, p.SourceMetadata)
	} else {
		sourceMetaCopy = json.RawMessage("{}")
	}

	var lastFetched *time.Time
	if p.LastFetchedAt != nil {
		t := *p.LastFetchedAt
		lastFetched = &t
	}

	pt := NormalizeProjectType(p.ProjectType)
	st := NormalizeSourceType(p.SourceType)

	return ProjectDTO{
		ID:             p.ID,
		Name:           p.Name,
		RepoURL:        p.RepoURL,
		RepoOwner:      p.RepoOwner,
		RepoName:       p.RepoName,
		DefaultBranch:  p.DefaultBranch,
		LocalPath:      p.LocalPath,
		HasToken:       p.EncryptedToken != "" || p.TokenPreview != "",
		TokenPreview:   p.TokenPreview,
		CloneStatus:    p.CloneStatus,
		ProjectType:    pt,
		Metadata:       metadataCopy,
		EnabledModules: modulesCopy,
		SourceType:     st,
		SourceMetadata: sourceMetaCopy,
		IsArchived:     p.IsArchived,
		LastFetchedAt:  lastFetched,
		LastError:      p.LastError,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

// ProjectRepository defines data operations for Projects.
type ProjectRepository interface {
	CreateProject(ctx context.Context, p *Project) error
	GetProjectByID(ctx context.Context, id string) (*Project, error)
	GetProjectByRepo(ctx context.Context, owner, repo string) (*Project, error)
	ListProjects(ctx context.Context) ([]*Project, error)
	ListProjectsByType(ctx context.Context, projectType string) ([]*Project, error)
	ListProjectsBySourceType(ctx context.Context, sourceType string) ([]*Project, error)
	UpdateProject(ctx context.Context, p *Project) error
	DeleteProject(ctx context.Context, id string) error
}

// SQLProjectRepository implements ProjectRepository using PostgreSQL.
type SQLProjectRepository struct {
	db *sql.DB
}

// NewSQLProjectRepository creates a new SQLProjectRepository.
func NewSQLProjectRepository(db *sql.DB) *SQLProjectRepository {
	return &SQLProjectRepository{db: db}
}

func (r *SQLProjectRepository) CreateProject(ctx context.Context, p *Project) error {
	if p == nil {
		return errors.New("project cannot be nil")
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrEmptyProjectName
	}
	if strings.TrimSpace(p.RepoURL) == "" {
		return ErrEmptyRepoURL
	}
	if strings.TrimSpace(p.ID) == "" {
		p.ID = GenerateUUID()
	}

	p.ProjectType = NormalizeProjectType(p.ProjectType)
	p.SourceType = NormalizeSourceType(p.SourceType)
	if len(p.Metadata) == 0 {
		p.Metadata = json.RawMessage("{}")
	}
	if len(p.EnabledModules) == 0 {
		p.EnabledModules = DefaultEnabledModules()
	}
	if len(p.SourceMetadata) == 0 {
		p.SourceMetadata = json.RawMessage("{}")
	}

	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	query := `
		INSERT INTO projects (
			id, name, repo_url, default_branch, local_path,
			encrypted_token, token_preview, repo_owner, repo_name,
			clone_status, project_type, metadata,
			enabled_modules, source_type, source_metadata,
			is_archived, last_fetched_at, last_error, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14, $15,
			$16, $17, $18, $19, $20
		)
	`
	_, err := r.db.ExecContext(ctx, query,
		p.ID,
		p.Name,
		p.RepoURL,
		p.DefaultBranch,
		p.LocalPath,
		p.EncryptedToken,
		p.TokenPreview,
		p.RepoOwner,
		p.RepoName,
		p.CloneStatus,
		p.ProjectType,
		[]byte(p.Metadata),
		[]byte(p.EnabledModules),
		p.SourceType,
		[]byte(p.SourceMetadata),
		p.IsArchived,
		p.LastFetchedAt,
		p.LastError,
		p.CreatedAt,
		p.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(err.Error(), "23505") {
			return ErrProjectAlreadyExists
		}
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (r *SQLProjectRepository) GetProjectByID(ctx context.Context, id string) (*Project, error) {
	query := `
		SELECT 
			id, name, repo_url, default_branch, COALESCE(local_path, ''),
			COALESCE(encrypted_token, ''), COALESCE(token_preview, ''),
			COALESCE(repo_owner, ''), COALESCE(repo_name, ''),
			COALESCE(clone_status, 'pending'),
			COALESCE(project_type, 'custom'),
			COALESCE(metadata, '{}'::jsonb),
			COALESCE(enabled_modules, '["tasks", "discussions"]'::jsonb),
			COALESCE(source_type, 'blank'),
			COALESCE(source_metadata, '{}'::jsonb),
			COALESCE(is_archived, FALSE),
			last_fetched_at,
			COALESCE(last_error, ''), created_at, updated_at
		FROM projects
		WHERE id = $1
	`
	var p Project
	var lastFetched sql.NullTime
	var metadataBytes, modulesBytes, sourceMetaBytes []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID,
		&p.Name,
		&p.RepoURL,
		&p.DefaultBranch,
		&p.LocalPath,
		&p.EncryptedToken,
		&p.TokenPreview,
		&p.RepoOwner,
		&p.RepoName,
		&p.CloneStatus,
		&p.ProjectType,
		&metadataBytes,
		&modulesBytes,
		&p.SourceType,
		&sourceMetaBytes,
		&p.IsArchived,
		&lastFetched,
		&p.LastError,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project by id: %w", err)
	}

	if lastFetched.Valid {
		p.LastFetchedAt = &lastFetched.Time
	}
	if len(metadataBytes) > 0 {
		p.Metadata = json.RawMessage(metadataBytes)
	} else {
		p.Metadata = json.RawMessage("{}")
	}
	if len(modulesBytes) > 0 {
		p.EnabledModules = json.RawMessage(modulesBytes)
	} else {
		p.EnabledModules = DefaultEnabledModules()
	}
	if len(sourceMetaBytes) > 0 {
		p.SourceMetadata = json.RawMessage(sourceMetaBytes)
	} else {
		p.SourceMetadata = json.RawMessage("{}")
	}

	return &p, nil
}

func (r *SQLProjectRepository) GetProjectByRepo(ctx context.Context, owner, repo string) (*Project, error) {
	query := `
		SELECT 
			id, name, repo_url, default_branch, COALESCE(local_path, ''),
			COALESCE(encrypted_token, ''), COALESCE(token_preview, ''),
			COALESCE(repo_owner, ''), COALESCE(repo_name, ''),
			COALESCE(clone_status, 'pending'),
			COALESCE(project_type, 'custom'),
			COALESCE(metadata, '{}'::jsonb),
			COALESCE(enabled_modules, '["tasks", "discussions"]'::jsonb),
			COALESCE(source_type, 'blank'),
			COALESCE(source_metadata, '{}'::jsonb),
			COALESCE(is_archived, FALSE),
			last_fetched_at,
			COALESCE(last_error, ''), created_at, updated_at
		FROM projects
		WHERE LOWER(repo_owner) = LOWER($1) AND LOWER(repo_name) = LOWER($2)
		LIMIT 1
	`
	var p Project
	var lastFetched sql.NullTime
	var metadataBytes, modulesBytes, sourceMetaBytes []byte

	err := r.db.QueryRowContext(ctx, query, owner, repo).Scan(
		&p.ID,
		&p.Name,
		&p.RepoURL,
		&p.DefaultBranch,
		&p.LocalPath,
		&p.EncryptedToken,
		&p.TokenPreview,
		&p.RepoOwner,
		&p.RepoName,
		&p.CloneStatus,
		&p.ProjectType,
		&metadataBytes,
		&modulesBytes,
		&p.SourceType,
		&sourceMetaBytes,
		&p.IsArchived,
		&lastFetched,
		&p.LastError,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project by repo: %w", err)
	}

	if lastFetched.Valid {
		p.LastFetchedAt = &lastFetched.Time
	}
	if len(metadataBytes) > 0 {
		p.Metadata = json.RawMessage(metadataBytes)
	} else {
		p.Metadata = json.RawMessage("{}")
	}
	if len(modulesBytes) > 0 {
		p.EnabledModules = json.RawMessage(modulesBytes)
	} else {
		p.EnabledModules = DefaultEnabledModules()
	}
	if len(sourceMetaBytes) > 0 {
		p.SourceMetadata = json.RawMessage(sourceMetaBytes)
	} else {
		p.SourceMetadata = json.RawMessage("{}")
	}

	return &p, nil
}

func (r *SQLProjectRepository) ListProjects(ctx context.Context) ([]*Project, error) {
	query := `
		SELECT 
			id, name, repo_url, default_branch, COALESCE(local_path, ''),
			COALESCE(encrypted_token, ''), COALESCE(token_preview, ''),
			COALESCE(repo_owner, ''), COALESCE(repo_name, ''),
			COALESCE(clone_status, 'pending'),
			COALESCE(project_type, 'custom'),
			COALESCE(metadata, '{}'::jsonb),
			COALESCE(enabled_modules, '["tasks", "discussions"]'::jsonb),
			COALESCE(source_type, 'blank'),
			COALESCE(source_metadata, '{}'::jsonb),
			COALESCE(is_archived, FALSE),
			last_fetched_at,
			COALESCE(last_error, ''), created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var list []*Project
	for rows.Next() {
		var p Project
		var lastFetched sql.NullTime
		var metadataBytes, modulesBytes, sourceMetaBytes []byte

		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.RepoURL,
			&p.DefaultBranch,
			&p.LocalPath,
			&p.EncryptedToken,
			&p.TokenPreview,
			&p.RepoOwner,
			&p.RepoName,
			&p.CloneStatus,
			&p.ProjectType,
			&metadataBytes,
			&modulesBytes,
			&p.SourceType,
			&sourceMetaBytes,
			&p.IsArchived,
			&lastFetched,
			&p.LastError,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project row: %w", err)
		}

		if lastFetched.Valid {
			p.LastFetchedAt = &lastFetched.Time
		}
		if len(metadataBytes) > 0 {
			p.Metadata = json.RawMessage(metadataBytes)
		} else {
			p.Metadata = json.RawMessage("{}")
		}
		if len(modulesBytes) > 0 {
			p.EnabledModules = json.RawMessage(modulesBytes)
		} else {
			p.EnabledModules = DefaultEnabledModules()
		}
		if len(sourceMetaBytes) > 0 {
			p.SourceMetadata = json.RawMessage(sourceMetaBytes)
		} else {
			p.SourceMetadata = json.RawMessage("{}")
		}

		list = append(list, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project rows: %w", err)
	}

	return list, nil
}

func (r *SQLProjectRepository) ListProjectsByType(ctx context.Context, projectType string) ([]*Project, error) {
	pt := NormalizeProjectType(projectType)
	query := `
		SELECT 
			id, name, repo_url, default_branch, COALESCE(local_path, ''),
			COALESCE(encrypted_token, ''), COALESCE(token_preview, ''),
			COALESCE(repo_owner, ''), COALESCE(repo_name, ''),
			COALESCE(clone_status, 'pending'),
			COALESCE(project_type, 'custom'),
			COALESCE(metadata, '{}'::jsonb),
			COALESCE(enabled_modules, '["tasks", "discussions"]'::jsonb),
			COALESCE(source_type, 'blank'),
			COALESCE(source_metadata, '{}'::jsonb),
			COALESCE(is_archived, FALSE),
			last_fetched_at,
			COALESCE(last_error, ''), created_at, updated_at
		FROM projects
		WHERE LOWER(project_type) = LOWER($1)
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, pt)
	if err != nil {
		return nil, fmt.Errorf("list projects by type: %w", err)
	}
	defer rows.Close()

	var list []*Project
	for rows.Next() {
		var p Project
		var lastFetched sql.NullTime
		var metadataBytes, modulesBytes, sourceMetaBytes []byte

		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.RepoURL,
			&p.DefaultBranch,
			&p.LocalPath,
			&p.EncryptedToken,
			&p.TokenPreview,
			&p.RepoOwner,
			&p.RepoName,
			&p.CloneStatus,
			&p.ProjectType,
			&metadataBytes,
			&modulesBytes,
			&p.SourceType,
			&sourceMetaBytes,
			&p.IsArchived,
			&lastFetched,
			&p.LastError,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project row: %w", err)
		}

		if lastFetched.Valid {
			p.LastFetchedAt = &lastFetched.Time
		}
		if len(metadataBytes) > 0 {
			p.Metadata = json.RawMessage(metadataBytes)
		} else {
			p.Metadata = json.RawMessage("{}")
		}
		if len(modulesBytes) > 0 {
			p.EnabledModules = json.RawMessage(modulesBytes)
		} else {
			p.EnabledModules = DefaultEnabledModules()
		}
		if len(sourceMetaBytes) > 0 {
			p.SourceMetadata = json.RawMessage(sourceMetaBytes)
		} else {
			p.SourceMetadata = json.RawMessage("{}")
		}

		list = append(list, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project rows: %w", err)
	}

	return list, nil
}

func (r *SQLProjectRepository) ListProjectsBySourceType(ctx context.Context, sourceType string) ([]*Project, error) {
	st := NormalizeSourceType(sourceType)
	query := `
		SELECT 
			id, name, repo_url, default_branch, COALESCE(local_path, ''),
			COALESCE(encrypted_token, ''), COALESCE(token_preview, ''),
			COALESCE(repo_owner, ''), COALESCE(repo_name, ''),
			COALESCE(clone_status, 'pending'),
			COALESCE(project_type, 'custom'),
			COALESCE(metadata, '{}'::jsonb),
			COALESCE(enabled_modules, '["tasks", "discussions"]'::jsonb),
			COALESCE(source_type, 'blank'),
			COALESCE(source_metadata, '{}'::jsonb),
			COALESCE(is_archived, FALSE),
			last_fetched_at,
			COALESCE(last_error, ''), created_at, updated_at
		FROM projects
		WHERE LOWER(source_type) = LOWER($1)
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, st)
	if err != nil {
		return nil, fmt.Errorf("list projects by source type: %w", err)
	}
	defer rows.Close()

	var list []*Project
	for rows.Next() {
		var p Project
		var lastFetched sql.NullTime
		var metadataBytes, modulesBytes, sourceMetaBytes []byte

		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.RepoURL,
			&p.DefaultBranch,
			&p.LocalPath,
			&p.EncryptedToken,
			&p.TokenPreview,
			&p.RepoOwner,
			&p.RepoName,
			&p.CloneStatus,
			&p.ProjectType,
			&metadataBytes,
			&modulesBytes,
			&p.SourceType,
			&sourceMetaBytes,
			&p.IsArchived,
			&lastFetched,
			&p.LastError,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project row: %w", err)
		}

		if lastFetched.Valid {
			p.LastFetchedAt = &lastFetched.Time
		}
		if len(metadataBytes) > 0 {
			p.Metadata = json.RawMessage(metadataBytes)
		} else {
			p.Metadata = json.RawMessage("{}")
		}
		if len(modulesBytes) > 0 {
			p.EnabledModules = json.RawMessage(modulesBytes)
		} else {
			p.EnabledModules = DefaultEnabledModules()
		}
		if len(sourceMetaBytes) > 0 {
			p.SourceMetadata = json.RawMessage(sourceMetaBytes)
		} else {
			p.SourceMetadata = json.RawMessage("{}")
		}

		list = append(list, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project rows: %w", err)
	}

	return list, nil
}

func (r *SQLProjectRepository) UpdateProject(ctx context.Context, p *Project) error {
	if p == nil {
		return errors.New("project cannot be nil")
	}
	p.ProjectType = NormalizeProjectType(p.ProjectType)
	p.SourceType = NormalizeSourceType(p.SourceType)
	if len(p.Metadata) == 0 {
		p.Metadata = json.RawMessage("{}")
	}
	if len(p.EnabledModules) == 0 {
		p.EnabledModules = DefaultEnabledModules()
	}
	if len(p.SourceMetadata) == 0 {
		p.SourceMetadata = json.RawMessage("{}")
	}
	p.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE projects SET
			name = $2,
			repo_url = $3,
			default_branch = $4,
			local_path = $5,
			encrypted_token = $6,
			token_preview = $7,
			repo_owner = $8,
			repo_name = $9,
			clone_status = $10,
			project_type = $11,
			metadata = $12,
			enabled_modules = $13,
			source_type = $14,
			source_metadata = $15,
			is_archived = $16,
			last_fetched_at = $17,
			last_error = $18,
			updated_at = $19
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query,
		p.ID,
		p.Name,
		p.RepoURL,
		p.DefaultBranch,
		p.LocalPath,
		p.EncryptedToken,
		p.TokenPreview,
		p.RepoOwner,
		p.RepoName,
		p.CloneStatus,
		p.ProjectType,
		[]byte(p.Metadata),
		[]byte(p.EnabledModules),
		p.SourceType,
		[]byte(p.SourceMetadata),
		p.IsArchived,
		p.LastFetchedAt,
		p.LastError,
		p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project rows affected: %w", err)
	}
	if rows == 0 {
		return ErrProjectNotFound
	}
	return nil
}

func (r *SQLProjectRepository) DeleteProject(ctx context.Context, id string) error {
	query := "DELETE FROM projects WHERE id = $1"
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete project rows affected: %w", err)
	}
	if rows == 0 {
		return ErrProjectNotFound
	}
	return nil
}

// MemoryProjectRepository implements ProjectRepository in-memory for testing and non-persistent mode.
type MemoryProjectRepository struct {
	mu       sync.RWMutex
	projects map[string]*Project
}

// NewMemoryProjectRepository creates a new thread-safe MemoryProjectRepository.
func NewMemoryProjectRepository() *MemoryProjectRepository {
	return &MemoryProjectRepository{
		projects: make(map[string]*Project),
	}
}

func cloneProject(p *Project) *Project {
	if p == nil {
		return nil
	}
	cp := *p
	if len(p.Metadata) > 0 {
		cp.Metadata = make(json.RawMessage, len(p.Metadata))
		copy(cp.Metadata, p.Metadata)
	} else {
		cp.Metadata = json.RawMessage("{}")
	}
	if len(p.EnabledModules) > 0 {
		cp.EnabledModules = make(json.RawMessage, len(p.EnabledModules))
		copy(cp.EnabledModules, p.EnabledModules)
	} else {
		cp.EnabledModules = DefaultEnabledModules()
	}
	if cp.SourceType == "" {
		cp.SourceType = "blank"
	}
	if len(p.SourceMetadata) > 0 {
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

func (m *MemoryProjectRepository) CreateProject(ctx context.Context, p *Project) error {
	if p == nil {
		return errors.New("project cannot be nil")
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrEmptyProjectName
	}
	if strings.TrimSpace(p.RepoURL) == "" {
		return ErrEmptyRepoURL
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.projects {
		if strings.EqualFold(existing.RepoURL, p.RepoURL) ||
			(strings.EqualFold(existing.RepoOwner, p.RepoOwner) && strings.EqualFold(existing.RepoName, p.RepoName) && p.RepoOwner != "") {
			return ErrProjectAlreadyExists
		}
	}

	if strings.TrimSpace(p.ID) == "" {
		p.ID = GenerateUUID()
	}

	p.ProjectType = NormalizeProjectType(p.ProjectType)
	p.SourceType = NormalizeSourceType(p.SourceType)
	if len(p.Metadata) == 0 {
		p.Metadata = json.RawMessage("{}")
	}
	if len(p.EnabledModules) == 0 {
		p.EnabledModules = DefaultEnabledModules()
	}
	if len(p.SourceMetadata) == 0 {
		p.SourceMetadata = json.RawMessage("{}")
	}

	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	m.projects[p.ID] = cloneProject(p)
	return nil
}

func (m *MemoryProjectRepository) GetProjectByID(ctx context.Context, id string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}
	return cloneProject(p), nil
}

func (m *MemoryProjectRepository) GetProjectByRepo(ctx context.Context, owner, repo string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.projects {
		if strings.EqualFold(p.RepoOwner, owner) && strings.EqualFold(p.RepoName, repo) {
			return cloneProject(p), nil
		}
	}
	return nil, ErrProjectNotFound
}

func (m *MemoryProjectRepository) ListProjects(ctx context.Context) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*Project
	for _, p := range m.projects {
		list = append(list, cloneProject(p))
	}
	return list, nil
}

func (m *MemoryProjectRepository) ListProjectsByType(ctx context.Context, projectType string) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targetType := NormalizeProjectType(projectType)
	var list []*Project
	for _, p := range m.projects {
		if NormalizeProjectType(p.ProjectType) == targetType {
			list = append(list, cloneProject(p))
		}
	}
	return list, nil
}

func (m *MemoryProjectRepository) ListProjectsBySourceType(ctx context.Context, sourceType string) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targetSource := NormalizeSourceType(sourceType)
	var list []*Project
	for _, p := range m.projects {
		if NormalizeSourceType(p.SourceType) == targetSource {
			list = append(list, cloneProject(p))
		}
	}
	return list, nil
}

func (m *MemoryProjectRepository) UpdateProject(ctx context.Context, p *Project) error {
	if p == nil {
		return errors.New("project cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.projects[p.ID]
	if !exists {
		return ErrProjectNotFound
	}

	p.ProjectType = NormalizeProjectType(p.ProjectType)
	p.SourceType = NormalizeSourceType(p.SourceType)
	if len(p.Metadata) == 0 {
		p.Metadata = json.RawMessage("{}")
	}
	if len(p.EnabledModules) == 0 {
		p.EnabledModules = DefaultEnabledModules()
	}
	if len(p.SourceMetadata) == 0 {
		p.SourceMetadata = json.RawMessage("{}")
	}
	p.UpdatedAt = time.Now().UTC()
	p.CreatedAt = existing.CreatedAt

	m.projects[p.ID] = cloneProject(p)
	return nil
}

func (m *MemoryProjectRepository) DeleteProject(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[id]; !exists {
		return ErrProjectNotFound
	}

	delete(m.projects, id)
	return nil
}

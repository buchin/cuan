package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type DBStore struct {
	db *sql.DB
}

func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) CreateProject(ctx context.Context, p *Project) error {
	pt := p.ProjectType
	if pt == "" {
		pt = "custom"
	}
	if len(p.Metadata) == 0 {
		p.Metadata = json.RawMessage("{}")
	}
	enabledModules := p.EnabledModules
	if len(enabledModules) == 0 {
		enabledModules = json.RawMessage(`["tasks", "discussions"]`)
	}
	sourceType := p.SourceType
	if sourceType == "" {
		sourceType = "blank"
	}
	sourceMetadata := p.SourceMetadata
	if len(sourceMetadata) == 0 {
		sourceMetadata = json.RawMessage("{}")
	}

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
	_, err := s.db.ExecContext(ctx, query,
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
		pt,
		[]byte(p.Metadata),
		[]byte(enabledModules),
		sourceType,
		[]byte(sourceMetadata),
		p.IsArchived,
		p.LastFetchedAt,
		p.LastError,
		p.CreatedAt,
		p.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return ErrProjectAlreadyExists
		}
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (s *DBStore) GetProject(ctx context.Context, id string) (*Project, error) {
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

	err := s.db.QueryRowContext(ctx, query, id).Scan(
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
		return nil, fmt.Errorf("get project: %w", err)
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
		p.EnabledModules = json.RawMessage(`["tasks", "discussions"]`)
	}
	if len(sourceMetaBytes) > 0 {
		p.SourceMetadata = json.RawMessage(sourceMetaBytes)
	} else {
		p.SourceMetadata = json.RawMessage("{}")
	}

	return &p, nil
}

func (s *DBStore) GetProjectByRepo(ctx context.Context, owner, repo string) (*Project, error) {
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

	err := s.db.QueryRowContext(ctx, query, owner, repo).Scan(
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
		p.EnabledModules = json.RawMessage(`["tasks", "discussions"]`)
	}
	if len(sourceMetaBytes) > 0 {
		p.SourceMetadata = json.RawMessage(sourceMetaBytes)
	} else {
		p.SourceMetadata = json.RawMessage("{}")
	}

	return &p, nil
}

func (s *DBStore) ListProjects(ctx context.Context) ([]*Project, error) {
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
	rows, err := s.db.QueryContext(ctx, query)
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
			return nil, fmt.Errorf("scan project: %w", err)
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
			p.EnabledModules = json.RawMessage(`["tasks", "discussions"]`)
		}
		if len(sourceMetaBytes) > 0 {
			p.SourceMetadata = json.RawMessage(sourceMetaBytes)
		} else {
			p.SourceMetadata = json.RawMessage("{}")
		}
		list = append(list, &p)
	}

	return list, nil
}

// ListProjectsFiltered returns projects filtered at the database layer,
// avoiding full table scans followed by in-memory filtering.
func (s *DBStore) ListProjectsFiltered(ctx context.Context, filter ArchivedFilter) ([]*Project, error) {
	var whereClause string
	switch filter {
	case ArchivedFilterOnly:
		whereClause = "WHERE COALESCE(is_archived, FALSE) = TRUE"
	case ArchivedFilterAll:
		whereClause = ""
	default: // ArchivedFilterActive
		whereClause = "WHERE COALESCE(is_archived, FALSE) = FALSE"
	}

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
		` + whereClause + `
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list projects filtered: %w", err)
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
			return nil, fmt.Errorf("scan project filtered: %w", err)
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
			p.EnabledModules = json.RawMessage(`["tasks", "discussions"]`)
		}
		if len(sourceMetaBytes) > 0 {
			p.SourceMetadata = json.RawMessage(sourceMetaBytes)
		} else {
			p.SourceMetadata = json.RawMessage("{}")
		}
		list = append(list, &p)
	}

	return list, rows.Err()
}

func (s *DBStore) UpdateProject(ctx context.Context, p *Project) error {
	p.UpdatedAt = time.Now().UTC()
	pt := p.ProjectType
	if pt == "" {
		pt = "custom"
	}
	if len(p.Metadata) == 0 {
		p.Metadata = json.RawMessage("{}")
	}
	enabledModules := p.EnabledModules
	if len(enabledModules) == 0 {
		enabledModules = json.RawMessage(`["tasks", "discussions"]`)
	}
	sourceType := p.SourceType
	if sourceType == "" {
		sourceType = "blank"
	}
	sourceMetadata := p.SourceMetadata
	if len(sourceMetadata) == 0 {
		sourceMetadata = json.RawMessage("{}")
	}

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
	res, err := s.db.ExecContext(ctx, query,
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
		pt,
		[]byte(p.Metadata),
		[]byte(enabledModules),
		sourceType,
		[]byte(sourceMetadata),
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

func (s *DBStore) DeleteProject(ctx context.Context, id string) error {
	query := "DELETE FROM projects WHERE id = $1"
	res, err := s.db.ExecContext(ctx, query, id)
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

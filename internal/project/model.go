package project

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	RepoURL        string          `json:"repo_url"`
	RepoOwner      string          `json:"repo_owner"`
	RepoName       string          `json:"repo_name"`
	DefaultBranch  string          `json:"default_branch"`
	LocalPath      string          `json:"local_path"`
	EncryptedToken string          `json:"-"` // Never serialized in JSON
	TokenPreview   string          `json:"token_preview"`
	CloneStatus    string          `json:"clone_status"`    // pending, connected, synced, failed
	ProjectType    string          `json:"project_type"`    // custom, blog, youtube, olshop, affiliate
	Metadata       json.RawMessage `json:"metadata"`        // blueprint configuration & workflow metadata
	EnabledModules json.RawMessage `json:"enabled_modules"` // e.g. ["tasks", "discussions"]
	SourceType     string          `json:"source_type"`     // blank, template, zip
	SourceMetadata json.RawMessage `json:"source_metadata"` // metadata for zip unpack, template source, etc.
	IsArchived     bool            `json:"is_archived"`
	LastFetchedAt  *time.Time      `json:"last_fetched_at,omitempty"`
	LastError      string          `json:"last_error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type ProjectResponse struct {
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

func (p *Project) ToResponse() *ProjectResponse {
	projectType := p.ProjectType
	if projectType == "" {
		projectType = "custom"
	}
	metadata := p.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
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

	return &ProjectResponse{
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
		ProjectType:    projectType,
		Metadata:       metadata,
		EnabledModules: enabledModules,
		SourceType:     sourceType,
		SourceMetadata: sourceMetadata,
		IsArchived:     p.IsArchived,
		LastFetchedAt:  p.LastFetchedAt,
		LastError:      p.LastError,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

type ConnectProjectRequest struct {
	Name           string          `json:"name"`
	RepoURL        string          `json:"repo_url"`
	Token          string          `json:"token,omitempty"`
	DefaultBranch  string          `json:"default_branch,omitempty"`
	ProjectType    string          `json:"project_type,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	EnabledModules json.RawMessage `json:"enabled_modules,omitempty"`
	SourceType     string          `json:"source_type,omitempty"`
	SourceMetadata json.RawMessage `json:"source_metadata,omitempty"`
	IsArchived     *bool           `json:"is_archived,omitempty"`
}

// ProjectIntegration represents an external service integration bound to a project.
type ProjectIntegration struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	IntegrationType string    `json:"integration_type"` // wordpress, etc.
	ConfigEncrypted string    `json:"-"`                // Never exposed in JSON
	Status          string    `json:"status"`           // active, inactive, error
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ProjectIntegrationResponse is the public DTO for ProjectIntegration.
type ProjectIntegrationResponse struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	IntegrationType string    `json:"integration_type"`
	HasConfig       bool      `json:"has_config"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (in *ProjectIntegration) ToResponse() *ProjectIntegrationResponse {
	return &ProjectIntegrationResponse{
		ID:              in.ID,
		ProjectID:       in.ProjectID,
		IntegrationType: in.IntegrationType,
		HasConfig:       in.ConfigEncrypted != "",
		Status:          in.Status,
		CreatedAt:       in.CreatedAt,
		UpdatedAt:       in.UpdatedAt,
	}
}

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
	CloneStatus    string          `json:"clone_status"` // pending, connected, synced, failed
	ProjectType    string          `json:"project_type"` // custom, blog, youtube, olshop, affiliate
	Metadata       json.RawMessage `json:"metadata"`     // blueprint configuration & workflow metadata
	LastFetchedAt  *time.Time      `json:"last_fetched_at,omitempty"`
	LastError      string          `json:"last_error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type ProjectResponse struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	RepoURL       string          `json:"repo_url"`
	RepoOwner     string          `json:"repo_owner"`
	RepoName      string          `json:"repo_name"`
	DefaultBranch string          `json:"default_branch"`
	LocalPath     string          `json:"local_path,omitempty"`
	HasToken      bool            `json:"has_token"`
	TokenPreview  string          `json:"token_preview,omitempty"`
	CloneStatus   string          `json:"clone_status"`
	ProjectType   string          `json:"project_type"`
	Metadata      json.RawMessage `json:"metadata"`
	LastFetchedAt *time.Time      `json:"last_fetched_at,omitempty"`
	LastError     string          `json:"last_error,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
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

	return &ProjectResponse{
		ID:            p.ID,
		Name:          p.Name,
		RepoURL:       p.RepoURL,
		RepoOwner:     p.RepoOwner,
		RepoName:      p.RepoName,
		DefaultBranch: p.DefaultBranch,
		LocalPath:     p.LocalPath,
		HasToken:      p.EncryptedToken != "" || p.TokenPreview != "",
		TokenPreview:  p.TokenPreview,
		CloneStatus:   p.CloneStatus,
		ProjectType:   projectType,
		Metadata:      metadata,
		LastFetchedAt: p.LastFetchedAt,
		LastError:     p.LastError,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}

type ConnectProjectRequest struct {
	Name          string          `json:"name"`
	RepoURL       string          `json:"repo_url"`
	Token         string          `json:"token,omitempty"`
	DefaultBranch string          `json:"default_branch,omitempty"`
	ProjectType   string          `json:"project_type,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

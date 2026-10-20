package project

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/git"
)

type Service struct {
	store      Store
	gitClient  *git.Client
	authSecret string
	baseDir    string
}

func NewService(store Store, gitClient *git.Client, authSecret, baseDir string) *Service {
	if gitClient == nil {
		gitClient = git.NewClient()
	}
	if baseDir == "" {
		baseDir = "data/projects"
	}
	return &Service{
		store:      store,
		gitClient:  gitClient,
		authSecret: authSecret,
		baseDir:    baseDir,
	}
}

// Store returns the underlying project.Store.
func (s *Service) Store() Store {
	return s.store
}

// Connect validates and connects an existing GitHub repository to Cuan.
func (s *Service) Connect(ctx context.Context, req ConnectProjectRequest) (*ProjectResponse, error) {
	trimmedURL := strings.TrimSpace(req.RepoURL)
	if trimmedURL == "" {
		return nil, errors.New("repository URL is required")
	}

	owner, repo, err := git.ParseGitHubURL(trimmedURL)
	if err != nil {
		return nil, err
	}

	// 1. Check if project already exists
	if existing, err := s.store.GetProjectByRepo(ctx, owner, repo); err == nil && existing != nil {
		return nil, ErrProjectAlreadyExists
	}

	token := strings.TrimSpace(req.Token)

	// 2. Pre-flight verification with GitHub (validates permissions & existence)
	repoInfo, err := s.gitClient.ValidateRepository(ctx, fmt.Sprintf("%s/%s", owner, repo), token)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = repo
	}

	defaultBranch := strings.TrimSpace(req.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = repoInfo.DefaultBranch
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	// 3. Encrypt GitHub access token for safe storage
	encryptedToken, err := crypto.Encrypt(token, s.authSecret)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	tokenPreview := crypto.TokenPreview(token)

	// Generate project ID
	randBytes := make([]byte, 6)
	_, _ = rand.Read(randBytes)
	projectID := fmt.Sprintf("proj_%s", hex.EncodeToString(randBytes))

	localPath := filepath.Join(s.baseDir, projectID)

	projectType := strings.ToLower(strings.TrimSpace(req.ProjectType))
	if projectType == "" {
		projectType = "custom"
	}
	metadata := req.Metadata
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}
	enabledModules := req.EnabledModules
	if len(enabledModules) == 0 {
		enabledModules = []byte(`["tasks", "discussions"]`)
	}
	sourceType := strings.ToLower(strings.TrimSpace(req.SourceType))
	if sourceType == "" {
		sourceType = "blank"
	}
	sourceMetadata := req.SourceMetadata
	if len(sourceMetadata) == 0 {
		sourceMetadata = []byte("{}")
	}

	now := time.Now().UTC()
	p := &Project{
		ID:             projectID,
		Name:           name,
		RepoURL:        fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		RepoOwner:      owner,
		RepoName:       repo,
		DefaultBranch:  defaultBranch,
		LocalPath:      localPath,
		EncryptedToken: encryptedToken,
		TokenPreview:   tokenPreview,
		CloneStatus:    "connected",
		ProjectType:    projectType,
		Metadata:       metadata,
		EnabledModules: enabledModules,
		SourceType:     sourceType,
		SourceMetadata: sourceMetadata,
		LastFetchedAt:  &now,
		LastError:      "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 4. Perform initial clone or fetch
	if err := s.gitClient.CloneOrFetch(ctx, owner, repo, token, localPath); err != nil {
		// If clone fails, record error but save project so owner can see status
		p.CloneStatus = "failed"
		p.LastError = err.Error()
	} else {
		p.CloneStatus = "synced"
	}

	if err := s.store.CreateProject(ctx, p); err != nil {
		return nil, err
	}

	return p.ToResponse(), nil
}

// Get returns the project by ID.
func (s *Service) Get(ctx context.Context, id string) (*ProjectResponse, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	return p.ToResponse(), nil
}

// List returns all connected projects.
func (s *Service) List(ctx context.Context) ([]*ProjectResponse, error) {
	list, err := s.store.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var res []*ProjectResponse
	for _, p := range list {
		res = append(res, p.ToResponse())
	}
	return res, nil
}

// Sync triggers a fetch / sync against the remote GitHub repository.
func (s *Service) Sync(ctx context.Context, id string) (*ProjectResponse, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}

	var token string
	if p.EncryptedToken != "" {
		decrypted, err := crypto.Decrypt(p.EncryptedToken, s.authSecret)
		if err != nil {
			return nil, fmt.Errorf("decrypt credentials: %w", err)
		}
		token = decrypted
	}

	now := time.Now().UTC()
	if err := s.gitClient.CloneOrFetch(ctx, p.RepoOwner, p.RepoName, token, p.LocalPath); err != nil {
		p.CloneStatus = "failed"
		p.LastError = err.Error()
		p.UpdatedAt = now
		_ = s.store.UpdateProject(ctx, p)
		return p.ToResponse(), err
	}

	p.CloneStatus = "synced"
	p.LastFetchedAt = &now
	p.LastError = ""
	p.UpdatedAt = now

	if err := s.store.UpdateProject(ctx, p); err != nil {
		return nil, err
	}

	return p.ToResponse(), nil
}

// GetBranches returns the remote branch list from GitHub.
func (s *Service) GetBranches(ctx context.Context, id string) ([]string, string, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, "", err
	}

	var token string
	if p.EncryptedToken != "" {
		decrypted, err := crypto.Decrypt(p.EncryptedToken, s.authSecret)
		if err != nil {
			return nil, "", fmt.Errorf("decrypt credentials: %w", err)
		}
		token = decrypted
	}

	branches, err := s.gitClient.FetchRemoteBranches(ctx, p.RepoOwner, p.RepoName, token)
	if err != nil {
		return []string{p.DefaultBranch}, p.DefaultBranch, nil
	}

	return branches, p.DefaultBranch, nil
}

// Delete removes the project from Cuan database and cleans up local caches.
// IMPORTANT: This operation DOES NOT delete or modify the remote GitHub repository.
func (s *Service) Delete(ctx context.Context, id string) error {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return err
	}

	// 1. Remove from database
	if err := s.store.DeleteProject(ctx, id); err != nil {
		return err
	}

	// 2. Clean up local path cache if exists
	if p.LocalPath != "" && strings.HasPrefix(p.LocalPath, s.baseDir) {
		_ = os.RemoveAll(p.LocalPath)
	}

	// Remote repository is untouched and remains safe on GitHub
	return nil
}

// UpdateModules updates the enabled_modules configuration for a project.
func (s *Service) UpdateModules(ctx context.Context, id string, modules []string) (*ProjectResponse, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}

	if modules == nil {
		modules = []string{"tasks", "discussions"}
	}

	raw, err := json.Marshal(modules)
	if err != nil {
		return nil, fmt.Errorf("marshal modules: %w", err)
	}

	p.EnabledModules = raw
	p.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateProject(ctx, p); err != nil {
		return nil, err
	}

	return p.ToResponse(), nil
}

// Archive marks a project as archived (is_archived = true).
func (s *Service) Archive(ctx context.Context, id string) (*ProjectResponse, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}

	p.IsArchived = true
	p.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateProject(ctx, p); err != nil {
		return nil, err
	}

	return p.ToResponse(), nil
}

// Unarchive restores an archived project to active status (is_archived = false).
func (s *Service) Unarchive(ctx context.Context, id string) (*ProjectResponse, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}

	p.IsArchived = false
	p.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateProject(ctx, p); err != nil {
		return nil, err
	}

	return p.ToResponse(), nil
}

// EnsureDefault makes first-run general work usable without replacing an existing
// project's repository, workspace, archive state or configuration.
func (s *Service) EnsureDefault(ctx context.Context) (*ProjectResponse, error) {
	if p, err := s.Get(ctx, "default"); err == nil {
		return p, nil
	} else if !errors.Is(err, ErrProjectNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	p := &Project{ID: "default", Name: "Default", ProjectType: "custom", DefaultBranch: "main", CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreateProject(ctx, p); err != nil {
		return s.Get(ctx, "default")
	}
	return s.Get(ctx, "default")
}

// CreateManagedWorkspace creates a local project for an approved delegated goal.
// IDs are application-generated; neither a repository URL nor a user path is accepted.
func (s *Service) CreateManagedWorkspace(ctx context.Context, id, name string) (*ProjectResponse, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 1 || len([]rune(name)) > 100 || !strings.HasPrefix(id, "proj_") || len(id) != 37 {
		return nil, errors.New("invalid managed project")
	}
	for _, r := range id[5:] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return nil, errors.New("invalid managed project ID")
		}
	}
	if existing, err := s.Get(ctx, id); err == nil {
		if existing.Name != name || existing.IsArchived {
			return nil, ErrProjectAlreadyExists
		}
		return existing, nil
	} else if !errors.Is(err, ErrProjectNotFound) {
		return nil, err
	}
	projects, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if strings.EqualFold(p.Name, name) {
			if p.IsArchived {
				return nil, ErrProjectAlreadyExists
			}
			return p, nil
		}
	}
	dir, err := filepath.Abs(filepath.Join(s.baseDir, id))
	if err != nil {
		return nil, err
	}
	if info, e := os.Lstat(dir); e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return nil, errors.New("project workspace is unavailable")
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	p := &Project{ID: id, Name: name, ProjectType: "custom", DefaultBranch: "main", LocalPath: dir, CreatedAt: now, UpdatedAt: now}
	if err = s.store.CreateProject(ctx, p); err != nil {
		return s.Get(ctx, id)
	}
	return s.Get(ctx, id)
}

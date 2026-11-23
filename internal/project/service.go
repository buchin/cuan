package project

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/git"
)

type Service struct {
	accessMu     sync.RWMutex
	sharedAccess func(context.Context) (string, error)
	store        Store
	gitClient    *git.Client
	authSecret   string
	baseDir      string
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
	if token == "" {
		token, err = s.repositoryToken(ctx, "")
		if err != nil {
			return nil, err
		}
	}

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
	encryptedToken, err := crypto.Encrypt(strings.TrimSpace(req.Token), s.authSecret)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	tokenPreview := crypto.TokenPreview(strings.TrimSpace(req.Token))

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

	// Persist identity before network I/O so a slow clone cannot lose the project.
	p.CloneStatus = "connected"
	dbCreateCtx, cancelCreate := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelCreate()
	if err := s.store.CreateProject(dbCreateCtx, p); err != nil {
		return nil, err
	}
	cloneBudget := 45 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline) - 5*time.Second; remaining < cloneBudget {
			cloneBudget = remaining
		}
	}
	if cloneBudget <= 0 {
		p.CloneStatus = "failed"
		p.LastError = "Repository download took too long. Your project is saved. Retry synchronization to continue."
	} else {
		cloneCtx, cancelClone := context.WithTimeout(ctx, cloneBudget)
		cloneErr := s.gitClient.CloneOrFetch(cloneCtx, owner, repo, token, localPath)
		cloneTimedOut := cloneCtx.Err() != nil
		cancelClone()
		if cloneErr != nil {
			p.CloneStatus = "failed"
			p.LastError = cloneErr.Error()
			if cloneTimedOut || errors.Is(cloneErr, context.DeadlineExceeded) {
				p.LastError = "Repository download took too long. Your project is saved. Retry synchronization to continue."
			}
		} else {
			p.CloneStatus = "synced"
		}
	}
	p.UpdatedAt = time.Now().UTC()
	dbUpdateCtx, cancelUpdate := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelUpdate()
	if err := s.store.UpdateProject(dbUpdateCtx, p); err != nil {
		return nil, fmt.Errorf("project saved, but synchronization status could not be updated: %w", err)
	}

	return p.ToResponse(), nil
}

// Get returns the project by ID.
func (s *Service) Get(ctx context.Context, id string) (*ProjectResponse, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	s.ensureValidLocalPath(ctx, p)
	return p.ToResponse(), nil
}

// List returns all connected projects. Prefer ListFiltered for the HTTP list
// endpoint, which pushes the archive predicate to the database layer.
func (s *Service) List(ctx context.Context) ([]*ProjectResponse, error) {
	list, err := s.store.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	if len(list) > 1 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for _, p := range list {
			wg.Add(1)
			go func(proj *Project) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				s.ensureValidLocalPath(ctx, proj)
			}(p)
		}
		wg.Wait()
	} else if len(list) == 1 {
		s.ensureValidLocalPath(ctx, list[0])
	}

	res := make([]*ProjectResponse, 0, len(list))
	for _, p := range list {
		res = append(res, p.ToResponse())
	}
	return res, nil
}

// ListFiltered returns projects pre-filtered by archive state at the database
// layer, avoiding in-memory scanning. It also skips per-project disk I/O
// (ensureValidLocalPath) which is unnecessary for the list view.
func (s *Service) ListFiltered(ctx context.Context, filter ArchivedFilter) ([]*ProjectListItem, error) {
	list, err := s.store.ListProjectsFiltered(ctx, filter)
	if err != nil {
		return nil, err
	}

	items := make([]*ProjectListItem, 0, len(list))
	for _, p := range list {
		items = append(items, p.ToListItem())
	}
	return items, nil
}

// Touch refreshes the project's updated_at timestamp to now, signalling fresh task or user activity.
func (s *Service) Touch(ctx context.Context, id string) error {
	return s.store.TouchProject(ctx, id)
}

// Sync triggers a fetch / sync against the remote GitHub repository.
func (s *Service) Sync(ctx context.Context, id string) (*ProjectResponse, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}

	s.ensureValidLocalPath(ctx, p)

	token, err := s.repositoryToken(ctx, p.EncryptedToken)
	if err != nil {
		return nil, err
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

// ownsWorkspace reports whether a stored path belongs to this project rather
// than to a different one. Workspace directories are named after the project ID,
// so a path whose last element is another project's identifier belongs there.
// Other names are kept as-is because managed workspaces use generated
// directory names inside the project's own base directory.
func (s *Service) ownsWorkspace(p *Project) bool {
	// The default project holds general work and keeps whatever path it was given.
	// It has no repository identity to compare against, so validating it would
	// rewrite a configuration the owner chose.
	if p.ID == "default" {
		return true
	}
	name := filepath.Base(filepath.Clean(p.LocalPath))
	if name == p.ID {
		return true
	}
	// A generated workspace name is not an identity, so it stays valid as long as
	// it is not named after a different project.
	if name == "default" || strings.HasPrefix(name, "proj_") {
		return false
	}
	return true
}

func (s *Service) ensureValidLocalPath(ctx context.Context, p *Project) {
	if p == nil || p.ID == "" {
		return
	}
	if p.RepoURL == "" && p.LocalPath == "" {
		return
	}
	// A stored path that belongs to this project and exists on disk is authoritative.
	// If the stored path is missing, check whether a candidate carrying this project's
	// own ID exists before falling back to unprovisioned workspace paths.
	if p.LocalPath != "" && s.ownsWorkspace(p) {
		if fi, err := os.Stat(p.LocalPath); err == nil && fi.IsDir() {
			return
		}
	}

	// Only a directory that carries this project's own identity may be adopted.
	// Deriving a candidate from the basename of a previously stored path adopted
	// another project's directory: "cuan" inherited the archived "default"
	// workspace and every task then failed with "project unavailable" because
	// the workspace belonged to a different project.
	candidates := []string{
		filepath.Join(s.baseDir, p.ID),
	}
	if s.baseDir == "data/projects" || strings.HasPrefix(s.baseDir, "/opt/cuan") {
		candidates = append(candidates, filepath.Join("/opt/cuan/data/projects", p.ID))
	}

	for _, candidate := range candidates {
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			p.LocalPath = candidate
			_ = s.store.UpdateProject(ctx, p)
			return
		}
	}

	// A path that already belongs to this project is preserved even before its
	// directory exists, because provisioning creates it later.
	if p.LocalPath != "" && s.ownsWorkspace(p) {
		return
	}

	// No candidate directory exists yet. Record the project's own workspace path
	// instead of leaving a stale path that points at another project's files.
	own := filepath.Join(s.baseDir, p.ID)
	if filepath.Clean(p.LocalPath) != filepath.Clean(own) {
		p.LocalPath = own
		_ = s.store.UpdateProject(ctx, p)
	}
}

// GetBranches returns the remote branch list from GitHub.
func (s *Service) GetBranches(ctx context.Context, id string) ([]string, string, error) {
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, "", err
	}

	token, err := s.repositoryToken(ctx, p.EncryptedToken)
	if err != nil {
		return nil, "", err
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
	p := &Project{ID: "default", Name: "Default", ProjectType: "custom", DefaultBranch: "", IsArchived: true, CreatedAt: now, UpdatedAt: now}
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
				if strings.EqualFold(name, "default") {
					continue
				}
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
	initCmd := exec.CommandContext(ctx, "git", "-C", dir, "init", "-b", "main")
	_ = initCmd.Run()
	now := time.Now().UTC()
	p := &Project{ID: id, Name: name, ProjectType: "custom", DefaultBranch: "main", LocalPath: dir, CreatedAt: now, UpdatedAt: now}
	if err = s.store.CreateProject(ctx, p); err != nil {
		return s.Get(ctx, id)
	}
	return s.Get(ctx, id)
}

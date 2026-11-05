package project

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
)

var ErrSettingsConflict = errors.New("Project changed elsewhere. Reload before saving.")
var settingsMu sync.Mutex

type Settings struct {
	Goal              string `json:"goal"`
	Instructions      string `json:"instructions"`
	WorkspaceRevision string `json:"workspace_revision,omitempty"`
	AutoDeploy        *bool  `json:"auto_deploy,omitempty"`
	DeployBranch      string `json:"deploy_branch,omitempty"`
	Environment       string `json:"environment,omitempty"`
}

func (s Settings) IsAutoDeployEnabled() bool {
	if s.AutoDeploy == nil {
		return true
	}
	return *s.AutoDeploy
}

type UpdateSettingsRequest struct {
	Name              string    `json:"name"`
	Goal              string    `json:"goal"`
	Instructions      string    `json:"instructions"`
	RepoURL           string    `json:"repo_url"`
	DefaultBranch     string    `json:"default_branch"`
	DeployBranch      string    `json:"deploy_branch,omitempty"`
	AutoDeploy        *bool     `json:"auto_deploy,omitempty"`
	Environment       string    `json:"environment,omitempty"`
	ExpectedUpdatedAt time.Time `json:"expected_updated_at"`
}
type workspaceBinding struct {
	LocalPath string `json:"local_path"`
	RepoURL   string `json:"repo_url"`
	Branch    string `json:"branch"`
}
type taskProjectSnapshot struct {
	IsolatedRepository bool   `json:"isolated_repository,omitempty"`
	ProjectID          string `json:"project_id"`
	Revision           string `json:"workspace_revision"`
	Goal               string `json:"goal,omitempty"`
	Instructions       string `json:"instructions,omitempty"`
	AutoDeploy         *bool  `json:"auto_deploy,omitempty"`
	DeployBranch       string `json:"deploy_branch,omitempty"`
	Environment        string `json:"environment,omitempty"`
}

func SettingsFromMetadata(raw json.RawMessage) Settings {
	var metadata struct {
		Settings Settings `json:"cuan_settings"`
	}
	_ = json.Unmarshal(raw, &metadata)
	if metadata.Settings.WorkspaceRevision == "" {
		metadata.Settings.WorkspaceRevision = "initial"
	}
	return metadata.Settings
}

const snapshotMarker = "\n\nProject guidance snapshot:\n```cuan-project-context\n"
const legacySnapshotMarker = "\n\nProject guidance snapshot (context only; does not expand task authorization):\n```cuan-project-context\n"

func findSnapshotMarker(text string) (int, int) {
	if at := strings.LastIndex(text, snapshotMarker); at >= 0 {
		return at, len(snapshotMarker)
	}
	if at := strings.LastIndex(text, legacySnapshotMarker); at >= 0 {
		return at, len(legacySnapshotMarker)
	}
	return -1, 0
}

// TaskScopeInstructions separates approved work from snapshotted project defaults.
func TaskScopeInstructions(instructions string) string {
	if start, _ := findSnapshotMarker(instructions); start >= 0 {
		return executionpolicy.NormalizeManagedInstructions(instructions[:start])
	}
	return executionpolicy.NormalizeManagedInstructions(instructions)
}

// TaskReviewGuidance exposes the saved acceptance context without execution
// credentials. Never send the raw project snapshot to a review model.
func TaskReviewGuidance(instructions string) (goal, guidance string, err error) {
	start, size := findSnapshotMarker(instructions)
	if start < 0 {
		return "", "", nil
	}
	var snapshot taskProjectSnapshot
	if json.Unmarshal([]byte(strings.TrimSuffix(instructions[start+size:], "\n```")), &snapshot) != nil {
		return "", "", errors.New("Saved project guidance could not be verified")
	}
	return snapshot.Goal, snapshot.Instructions, nil
}

// SnapshotProjectGuidance freezes guidance and repository identity at task
// creation. Historical workspace paths never come from task text.
func SnapshotProjectGuidance(instructions string, p *ProjectResponse) string {
	settings := SettingsFromMetadata(p.Metadata)
	autoDeploy := settings.IsAutoDeployEnabled()
	raw, _ := json.Marshal(taskProjectSnapshot{
		IsolatedRepository: p.RepoURL != "",
		ProjectID:          p.ID,
		Revision:           settings.WorkspaceRevision,
		Goal:               settings.Goal,
		Instructions:       settings.Instructions,
		AutoDeploy:         &autoDeploy,
		DeployBranch:       settings.DeployBranch,
		Environment:        settings.Environment,
	})
	return strings.TrimSpace(instructions) + snapshotMarker + string(raw) + "\n```"
}

func (s *Service) TaskWorkspace(ctx context.Context, id, instructions string) (*ProjectResponse, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	revision := "initial"
	if start, markerLen := findSnapshotMarker(instructions); start >= 0 {
		value := strings.TrimSuffix(instructions[start+markerLen:], "\n```")
		var snapshot taskProjectSnapshot
		if json.Unmarshal([]byte(value), &snapshot) != nil {
			return nil, errors.New("Task project context is invalid")
		}
		if !historicalProjectID(p, snapshot.ProjectID) {
			return p, nil
		} // Explicit project reassignment.
		revision = snapshot.Revision
	}
	settings := SettingsFromMetadata(p.Metadata)
	if revision == settings.WorkspaceRevision {
		return p, nil
	}
	var source map[string]json.RawMessage
	if json.Unmarshal(p.SourceMetadata, &source) != nil {
		return nil, errors.New("Project workspace history is invalid")
	}
	var versions map[string]string
	if json.Unmarshal(source["cuan_workspace_versions"], &versions) != nil || versions[revision] == "" {
		return nil, errors.New("Original task workspace is unavailable")
	}
	encrypted := versions[revision]
	raw, err := crypto.Decrypt(encrypted, s.authSecret)
	if err != nil {
		return nil, errors.New("Original task workspace could not be verified")
	}
	var original struct {
		ProjectID string           `json:"project_id"`
		Revision  string           `json:"revision"`
		Binding   workspaceBinding `json:"binding"`
	}
	if json.Unmarshal([]byte(raw), &original) != nil || !historicalProjectID(p, original.ProjectID) || original.Revision != revision {
		return nil, errors.New("Original task workspace binding is invalid")
	}
	p.LocalPath = original.Binding.LocalPath
	p.RepoURL = original.Binding.RepoURL
	p.DefaultBranch = original.Binding.Branch
	if p.LocalPath != "" {
		if fi, err := os.Stat(p.LocalPath); err != nil || !fi.IsDir() {
			if current, err := s.store.GetProject(ctx, p.ID); err == nil && current != nil {
				s.ensureValidLocalPath(ctx, current)
				if fi, err := os.Stat(current.LocalPath); err == nil && fi.IsDir() {
					p.LocalPath = current.LocalPath
				}
			}
		}
	}
	return p, nil
}

func (s *Service) UpdateSettings(ctx context.Context, id string, req UpdateSettingsRequest) (*ProjectResponse, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	current, err := s.store.GetProject(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.ExpectedUpdatedAt.IsZero() || !req.ExpectedUpdatedAt.Equal(current.UpdatedAt) {
		return nil, ErrSettingsConflict
	}
	if id == "default" && (strings.TrimSpace(req.Name) != "Default" || strings.TrimSpace(req.RepoURL) != "") {
		return nil, errors.New("Default is for general work. Use a separate project for a name or repository.")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 120 {
		return nil, errors.New("Project name must contain 1–120 characters.")
	}
	if len([]rune(req.Goal)) > 2000 || len([]rune(req.Instructions)) > 4000 {
		return nil, errors.New("Goal or instructions exceed the allowed length.")
	}
	if len([]rune(req.Environment)) > 32000 {
		return nil, errors.New("Environment configuration exceeds the allowed length (32,000 characters).")
	}
	branch := strings.TrimSpace(req.DefaultBranch)
	if branch != "" && (len(branch) > 200 || strings.HasPrefix(branch, "-") || exec.CommandContext(ctx, "git", "check-ref-format", "--branch", branch).Run() != nil) {
		return nil, errors.New("Enter a valid branch name.")
	}
	deployBranch := strings.TrimSpace(req.DeployBranch)
	if deployBranch != "" && (len(deployBranch) > 200 || strings.HasPrefix(deployBranch, "-") || exec.CommandContext(ctx, "git", "check-ref-format", "--branch", deployBranch).Run() != nil) {
		return nil, errors.New("Enter a valid deployment branch name.")
	}
	repo := strings.TrimSpace(req.RepoURL)
	owner, repoName := "", ""
	if repo != "" {
		if strings.Contains(repo, "@") || strings.Contains(repo, "?") || strings.Contains(repo, "#") {
			return nil, errors.New("Use a GitHub repository URL without credentials or query parameters.")
		}
		owner, repoName, err = git.ParseGitHubURL(repo)
		if err != nil {
			return nil, errors.New("Enter a GitHub repository URL, or leave it empty.")
		}
		repo = "https://github.com/" + owner + "/" + repoName
	}
	// An omitted branch preserves the current choice for the same repository.
	// A newly selected repository uses GitHub's default rather than assuming main.
	if branch == "" {
		if repo == strings.TrimSuffix(current.RepoURL, ".git") {
			branch = current.DefaultBranch
		} else if repo != "" {
			token, accessErr := s.repositoryToken(ctx, current.EncryptedToken)
			if accessErr != nil {
				return nil, accessErr
			}
			info, lookupErr := s.gitClient.ValidateRepository(ctx, owner+"/"+repoName, token)
			if lookupErr != nil {
				return nil, errors.New("Could not access this repository. No changes were saved.")
			}
			branch = info.DefaultBranch
		}
		if branch == "" {
			branch = "main"
		}
	}
	metadata := map[string]json.RawMessage{}
	if len(current.Metadata) > 0 && json.Unmarshal(current.Metadata, &metadata) != nil || metadata == nil {
		return nil, errors.New("Project metadata is invalid; no changes were saved.")
	}
	settings := SettingsFromMetadata(current.Metadata)
	settings.Goal = strings.TrimSpace(req.Goal)
	settings.Instructions = strings.TrimSpace(req.Instructions)
	settings.DeployBranch = deployBranch
	settings.Environment = strings.TrimSpace(req.Environment)
	if req.AutoDeploy != nil {
		settings.AutoDeploy = req.AutoDeploy
	}
	next := *current
	next.Name = name
	oldBranch := current.DefaultBranch
	if oldBranch == "" {
		oldBranch = "main"
	}
	connectionChanged := repo != strings.TrimSuffix(current.RepoURL, ".git") || branch != oldBranch
	newPath := ""
	if connectionChanged {
		var source map[string]json.RawMessage
		if len(current.SourceMetadata) == 0 {
			source = map[string]json.RawMessage{}
		} else if json.Unmarshal(current.SourceMetadata, &source) != nil || source == nil {
			return nil, errors.New("Project connection metadata is invalid.")
		}
		versions := map[string]string{}
		if raw, ok := source["cuan_workspace_versions"]; ok {
			if json.Unmarshal(raw, &versions) != nil || versions == nil {
				return nil, errors.New("Project workspace history is invalid.")
			}
		}
		if len(versions) >= 100 {
			return nil, errors.New("Workspace history needs administrator review before another connection change.")
		}
		original, _ := json.Marshal(struct {
			ProjectID string           `json:"project_id"`
			Revision  string           `json:"revision"`
			Binding   workspaceBinding `json:"binding"`
		}{id, settings.WorkspaceRevision, workspaceBinding{current.LocalPath, current.RepoURL, current.DefaultBranch}})
		protected, err := crypto.Encrypt(string(original), s.authSecret)
		if err != nil {
			return nil, errors.New("Could not retain the original workspace.")
		}
		versions[settings.WorkspaceRevision] = protected
		random := make([]byte, 12)
		if _, err = rand.Read(random); err != nil {
			return nil, err
		}
		revision := hex.EncodeToString(random)
		if err = os.MkdirAll(s.baseDir, 0700); err != nil {
			return nil, errors.New("Project workspace directory is unavailable.")
		}
		newPath, err = os.MkdirTemp(s.baseDir, "workspace-")
		if err != nil {
			return nil, errors.New("Could not prepare the project workspace.")
		}
		committed := false
		defer func() {
			if !committed {
				_ = os.RemoveAll(newPath)
			}
		}()
		if repo != "" {
			token, accessErr := s.repositoryToken(ctx, current.EncryptedToken)
			if accessErr != nil {
				return nil, accessErr
			}
			if _, err = s.gitClient.ValidateRepository(ctx, owner+"/"+repoName, token); err != nil {
				return nil, errors.New("Could not access this repository. Check its URL and saved GitHub access. No changes were saved.")
			}
			if err = s.gitClient.CloneOrFetch(ctx, owner, repoName, token, newPath); err != nil {
				return nil, errors.New("Could not prepare the repository. No changes were saved.")
			}
			if exec.CommandContext(ctx, "git", "-C", newPath, "checkout", "--detach", "refs/remotes/origin/"+branch).Run() != nil {
				return nil, errors.New("Branch was not found in this repository. No changes were saved.")
			}
		}
		next.LocalPath = newPath
		next.RepoURL = repo
		next.RepoOwner = owner
		next.RepoName = repoName
		next.DefaultBranch = branch
		next.CloneStatus = "connected"
		next.LastError = ""
		settings.WorkspaceRevision = revision
		source["cuan_workspace_versions"], _ = json.Marshal(versions)
		next.SourceMetadata, _ = json.Marshal(source)
		metadata["cuan_settings"], _ = json.Marshal(settings)
		next.Metadata, _ = json.Marshal(metadata)
		next.UpdatedAt = time.Now().UTC()
		if err = s.saveSettings(ctx, &next, current.UpdatedAt); err != nil {
			return nil, err
		}
		committed = true
		return next.ToResponse(), nil
	}
	metadata["cuan_settings"], _ = json.Marshal(settings)
	next.Metadata, _ = json.Marshal(metadata)
	next.UpdatedAt = time.Now().UTC()
	if err = s.saveSettings(ctx, &next, current.UpdatedAt); err != nil {
		return nil, err
	}
	return next.ToResponse(), nil
}

// PreserveProjectGuidance keeps the original task binding when the human or
// coordinator revises task instructions. A project reassignment is handled by
// TaskWorkspace using the snapshot's explicit project ID.
func PreserveProjectGuidance(previous, next string) string {
	start, _ := findSnapshotMarker(previous)
	if start < 0 {
		return strings.TrimSpace(next)
	}
	saved := previous[start:]
	if at, _ := findSnapshotMarker(next); at >= 0 {
		next = next[:at]
	}
	return strings.TrimSpace(next) + saved
}

// ParseEnvironment parses standard KEY=VALUE lines into a map[string]string.
// Handles comments (#), empty lines, quoted values ("val" or 'val'), and trims whitespace.
func ParseEnvironment(raw string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			result[key] = val
		}
	}
	return result
}

package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/buchin/cuan/internal/agent"
)

var (
	ErrDefaultBranchProtected = errors.New("cannot push directly to protected default branch (main/master). Changes must be pushed via task branch and reviewed through Pull Request")
	ErrWorkspaceEmpty         = errors.New("workspace directory is empty or invalid")
	ErrNoCommitsToPush        = errors.New("no commits to push on task branch")
)

// TaskWorkspace represents an isolated git checkout dedicated to a specific task.
type TaskWorkspace struct {
	TaskID        string `json:"task_id"`
	ProjectID     string `json:"project_id"`
	Path          string `json:"path"`
	BranchName    string `json:"branch_name"`
	DefaultBranch string `json:"default_branch"`
	AuthorName    string `json:"author_name"`
	AuthorEmail   string `json:"author_email"`
}

// WorkspaceConfig contains parameters needed to prepare an isolated task workspace.
type WorkspaceConfig struct {
	TaskID           string
	ProjectID        string
	BaseRepoDir      string
	WorkspacesBaseDir string
	BranchName       string
	DefaultBranch    string
	AuthorName       string
	AuthorEmail      string
	RemoteURL        string
	Token            string
}

// PrepareTaskWorkspace ensures an isolated workspace directory exists for the given task.
// If the workspace already exists, it resumes execution on the existing branch.
// If it is new, it creates an isolated local clone from the base repository,
// sets the task branch, configures human author identity, and installs pre-push protection.
func (c *Client) PrepareTaskWorkspace(ctx context.Context, cfg WorkspaceConfig) (*TaskWorkspace, error) {
	if cfg.TaskID == "" {
		return nil, errors.New("task ID is required to prepare workspace")
	}

	branchName := cfg.BranchName
	if branchName == "" {
		branchName = fmt.Sprintf("task/%s", cfg.TaskID)
	}

	defaultBranch := cfg.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	authorName := cfg.AuthorName
	if authorName == "" {
		authorName = "Buchin"
	}

	authorEmail := cfg.AuthorEmail
	if authorEmail == "" {
		authorEmail = "buchin@dropsugar.com"
	}

	workspacesBase := cfg.WorkspacesBaseDir
	if workspacesBase == "" {
		workspacesBase = filepath.Join("data", "workspaces")
	}

	workspacePath := filepath.Join(workspacesBase, cfg.TaskID)

	ws := &TaskWorkspace{
		TaskID:        cfg.TaskID,
		ProjectID:     cfg.ProjectID,
		Path:          workspacePath,
		BranchName:    branchName,
		DefaultBranch: defaultBranch,
		AuthorName:    authorName,
		AuthorEmail:   authorEmail,
	}

	gitDir := filepath.Join(workspacePath, ".git")
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		// Workspace exists -> resume cycle
		// Ensure checked out to task branch
		checkoutCmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "checkout", "-B", branchName)
		if _, err := checkoutCmd.CombinedOutput(); err != nil {
			// Self-heal: reset hard and clean untracked files in case of dirty workspace conflict
			_ = exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "reset", "--hard").Run()
			_ = exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "clean", "-fd").Run()
			retryCmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "checkout", "-B", branchName)
			if _, retryErr := retryCmd.CombinedOutput(); retryErr != nil {
				// Corrupted workspace that cannot be recovered -> remove it to trigger fresh clone below
				_ = os.RemoveAll(workspacePath)
			} else {
				_ = c.configureWorkspaceGitIdentity(workspacePath, authorName, authorEmail)
				_ = c.installBranchProtectionHook(workspacePath, defaultBranch)
				c.linkWorkspaceDependencies(workspacePath, cfg.BaseRepoDir)
				return ws, nil
			}
		} else {
			// Re-ensure git config and pre-push hook
			_ = c.configureWorkspaceGitIdentity(workspacePath, authorName, authorEmail)
			_ = c.installBranchProtectionHook(workspacePath, defaultBranch)
			c.linkWorkspaceDependencies(workspacePath, cfg.BaseRepoDir)
			return ws, nil
		}
	}

	// New workspace -> Create parent directory
	if err := os.MkdirAll(workspacesBase, 0755); err != nil {
		return nil, fmt.Errorf("create workspaces base dir: %w", err)
	}

	// Perform clone
	// If BaseRepoDir exists and has .git, clone locally from BaseRepoDir for speed and object sharing
	baseGitDir := filepath.Join(cfg.BaseRepoDir, ".git")
	if cfg.BaseRepoDir != "" {
		if fi, err := os.Stat(baseGitDir); err == nil && fi.IsDir() {
			cloneCmd := exec.CommandContext(ctx, c.gitBinary, "clone", "--local", cfg.BaseRepoDir, workspacePath)
			if out, err := cloneCmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("local clone from base repo failed: %s", string(out))
			}
		}
	}

	// If workspace wasn't cloned from BaseRepoDir, clone from RemoteURL or init
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if cfg.RemoteURL != "" {
			cloneCmd := exec.CommandContext(ctx, c.gitBinary, "clone", "--quiet", cfg.RemoteURL, workspacePath)
			if out, err := cloneCmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("remote clone failed: %s", string(out))
			}
		} else {
			// Fallback: init empty repo with default branch
			_ = os.MkdirAll(workspacePath, 0755)
			initCmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "init", "-b", defaultBranch)
			if out, err := initCmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("git init failed: %s", string(out))
			}
		}
	}

	// Configure remote origin if RemoteURL provided
	if cfg.RemoteURL != "" {
		_ = exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "remote", "set-url", "origin", cfg.RemoteURL).Run()
	}

	// Create and switch to task branch
	checkoutCmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "checkout", "-B", branchName)
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("create task branch %s: %s", branchName, string(out))
	}

	// Configure human author identity
	if err := c.configureWorkspaceGitIdentity(workspacePath, authorName, authorEmail); err != nil {
		return nil, err
	}

	// Install branch protection pre-push hook
	if err := c.installBranchProtectionHook(workspacePath, defaultBranch); err != nil {
		return nil, err
	}

	c.linkWorkspaceDependencies(workspacePath, cfg.BaseRepoDir)

	return ws, nil
}

// configureWorkspaceGitIdentity sets user.name and user.email in local repository config.
func (c *Client) configureWorkspaceGitIdentity(workspacePath, name, email string) error {
	cmdName := exec.Command(c.gitBinary, "-C", workspacePath, "config", "user.name", name)
	if out, err := cmdName.CombinedOutput(); err != nil {
		return fmt.Errorf("set git user.name: %s", string(out))
	}

	cmdEmail := exec.Command(c.gitBinary, "-C", workspacePath, "config", "user.email", email)
	if out, err := cmdEmail.CombinedOutput(); err != nil {
		return fmt.Errorf("set git user.email: %s", string(out))
	}

	return nil
}

// installBranchProtectionHook places an executable pre-push hook in the workspace repository
// that rejects direct pushes to the default branch (main/master).
func (c *Client) installBranchProtectionHook(workspacePath, defaultBranch string) error {
	hooksDir := filepath.Join(workspacePath, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	prePushScript := fmt.Sprintf(`#!/bin/sh
# Cuan Pre-Push Hook: Branch Protection Guardrail
# Prevents coding agents from pushing directly to the protected default branch.

protected_branch="%s"

while read local_ref local_oid remote_ref remote_oid
do
    if [ "$remote_ref" = "refs/heads/$protected_branch" ] || [ "$remote_ref" = "refs/heads/main" ] || [ "$remote_ref" = "refs/heads/master" ]; then
        echo "============================================================" >&2
        echo "ERROR: Direct push to protected default branch '$remote_ref' is PROHIBITED." >&2
        echo "All changes must be committed to a task branch and merged via Pull Request." >&2
        echo "============================================================" >&2
        exit 1
    fi
done

exit 0
`, defaultBranch)

	hookPath := filepath.Join(hooksDir, "pre-push")
	if err := os.WriteFile(hookPath, []byte(prePushScript), 0755); err != nil {
		return fmt.Errorf("write pre-push hook: %w", err)
	}

	return nil
}

// linkWorkspaceDependencies links node_modules and built web assets into the workspace
// from base repository or shared cache to ensure lean fast execution and reliable tests.
func (c *Client) linkWorkspaceDependencies(workspacePath, baseRepoDir string) {
	webDir := filepath.Join(workspacePath, "web")
	if _, err := os.Stat(webDir); err != nil {
		return
	}

	targetNodeModules := filepath.Join(webDir, "node_modules")
	if _, err := os.Lstat(targetNodeModules); err != nil {
		candidates := []string{
			filepath.Join(baseRepoDir, "web", "node_modules"),
			filepath.Join(baseRepoDir, "node_modules"),
			"/opt/cuan/data/projects/default/web/node_modules",
			"/opt/cuan/data/projects/task-workspaces/8405093f307a3e147d1170d23842c8c3575c98adfbec715451e76e215a9a530b/web/node_modules",
		}
		for _, cand := range candidates {
			if cand != "" {
				if realPath, rErr := filepath.EvalSymlinks(cand); rErr == nil {
					if fi, sErr := os.Stat(realPath); sErr == nil && fi.IsDir() {
						_ = os.Symlink(cand, targetNodeModules)
						break
					}
				}
			}
		}
	}

	targetDist := filepath.Join(webDir, "dist")
	if _, err := os.Lstat(targetDist); err != nil {
		candidates := []string{
			filepath.Join(baseRepoDir, "web", "dist"),
			os.Getenv("CUAN_STATIC_DIR"),
			"/opt/cuan/current/web/dist",
			"/opt/cuan/data/projects/default/web/dist",
		}
		for _, cand := range candidates {
			if cand != "" {
				if fi, sErr := os.Stat(cand); sErr == nil && fi.IsDir() {
					_ = os.Symlink(cand, targetDist)
					break
				}
			}
		}
	}
}

// CommitChanges stages all changes in workspace and commits with configured author identity.
// Returns the commit SHA, or empty string if there are no changes.
func (c *Client) CommitChanges(ctx context.Context, workspacePath, message, authorName, authorEmail string) (string, error) {
	// Stage all changes
	addCmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add: %s", string(out))
	}

	// Check if there are changes to commit
	statusCmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "status", "--porcelain")
	statusOut, err := statusCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}

	if len(strings.TrimSpace(string(statusOut))) == 0 {
		// Nothing to commit, return current HEAD commit
		return c.GetLatestCommitSHA(ctx, workspacePath)
	}

	if authorName == "" {
		authorName = "Buchin"
	}
	if authorEmail == "" {
		authorEmail = "buchin@dropsugar.com"
	}

	authorFlag := fmt.Sprintf("%s <%s>", authorName, authorEmail)
	commitCmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "commit", "-m", message, "--author", authorFlag)

	// Ensure author/committer env variables are preserved
	env := os.Environ()
	env = append(env,
		"GIT_AUTHOR_NAME="+authorName,
		"GIT_AUTHOR_EMAIL="+authorEmail,
		"GIT_COMMITTER_NAME="+authorName,
		"GIT_COMMITTER_EMAIL="+authorEmail,
	)
	commitCmd.Env = env

	if out, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit: %s", string(out))
	}

	return c.GetLatestCommitSHA(ctx, workspacePath)
}

// PushTaskBranch pushes the specified task branch to the remote repository.
// Protects default branch against accidental push attempts.
func (c *Client) PushTaskBranch(ctx context.Context, workspacePath, branchName, defaultBranch, token string) error {
	trimmedBranch := strings.TrimSpace(branchName)
	if trimmedBranch == "" {
		return errors.New("branch name cannot be empty")
	}

	// Protection check: Reject push if targeting main/master/default branch
	if trimmedBranch == defaultBranch || trimmedBranch == "main" || trimmedBranch == "master" {
		return ErrDefaultBranchProtected
	}

	secretsToMask := []string{}
	if token != "" {
		secretsToMask = append(secretsToMask, token, "x-access-token:"+token)
	}

	cmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "push", "-u", "origin", trimmedBranch)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	maskedStderr := agent.MaskSecrets(stderr.String(), secretsToMask)

	if runErr != nil {
		if strings.Contains(maskedStderr, "is PROHIBITED") {
			return ErrDefaultBranchProtected
		}
		return fmt.Errorf("git push branch '%s' failed: %s", trimmedBranch, maskedStderr)
	}

	return nil
}

// GetLatestCommitSHA returns the 40-character hexadecimal commit hash of HEAD.
func (c *Client) GetLatestCommitSHA(ctx context.Context, workspacePath string) (string, error) {
	cmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetDiffSummary produces a stat summary of changes comparing current HEAD to baseBranch.
func (c *Client) GetDiffSummary(ctx context.Context, workspacePath, baseBranch string) (string, error) {
	if baseBranch == "" {
		baseBranch = "main"
	}

	// Try comparing baseBranch...HEAD
	cmd := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "diff", "--stat", baseBranch+"...HEAD")
	out, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return strings.TrimSpace(string(out)), nil
	}

	// Fallback to origin/baseBranch...HEAD
	cmdRemote := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "diff", "--stat", "origin/"+baseBranch+"...HEAD")
	outRemote, errRemote := cmdRemote.Output()
	if errRemote == nil && len(strings.TrimSpace(string(outRemote))) > 0 {
		return strings.TrimSpace(string(outRemote)), nil
	}

	// Fallback to diff against HEAD~1
	cmdPrev := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "diff", "--stat", "HEAD~1...HEAD")
	outPrev, errPrev := cmdPrev.Output()
	if errPrev == nil && len(strings.TrimSpace(string(outPrev))) > 0 {
		return strings.TrimSpace(string(outPrev)), nil
	}

	// Fallback to git status --short
	cmdStatus := exec.CommandContext(ctx, c.gitBinary, "-C", workspacePath, "status", "--short")
	outStatus, _ := cmdStatus.Output()
	if len(strings.TrimSpace(string(outStatus))) > 0 {
		return strings.TrimSpace(string(outStatus)), nil
	}

	return "No file modifications recorded in diff.", nil
}

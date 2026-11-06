package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/taskprocess"
)

var (
	ErrDefaultBranchProtected = errors.New("cannot push directly to protected default branch (main/master). Changes must be pushed via task branch and reviewed through Pull Request")
	ErrWorkspaceEmpty         = errors.New("workspace directory is empty or invalid")
	ErrNoCommitsToPush        = errors.New("no commits to push on task branch")
)

// TaskWorkspace represents an isolated git checkout dedicated to a specific task.
type TaskWorkspace struct {
	BaseRef       string `json:"base_ref,omitempty"`
	BaseCommit    string `json:"base_commit,omitempty"`
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
	TaskID            string
	ProjectID         string
	BaseRepoDir       string
	WorkspacesBaseDir string
	BranchName        string
	DefaultBranch     string
	AuthorName        string
	AuthorEmail       string
	RemoteURL         string
	Token             string
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

	// Proactively prune stale and excess workspaces to prevent disk space exhaustion
	_, _ = c.PruneStaleWorkspaces(workspacesBase, 24*time.Hour, cfg.TaskID, 5)

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
	if _, err := os.Stat(gitDir); err == nil {
		ws.BaseRef, err = c.bindWorkspaceBase(ctx, cfg.BaseRepoDir, workspacePath, defaultBranch)
		if err != nil {
			return nil, err
		}
		if err := c.pinWorkspaceBase(ctx, ws); err != nil {
			return nil, err
		}
		// Resume in place. A merge conflict is saved work, never permission to
		// reset, clean, delete the workspace, or rewind the task branch.
		if err := c.checkoutRetainedTaskBranch(ctx, workspacePath, branchName, ws.BaseCommit); err != nil {
			return nil, err
		}
		if err := c.configureWorkspaceGitIdentity(workspacePath, authorName, authorEmail); err != nil {
			return nil, err
		}
		if err := c.installBranchProtectionHook(workspacePath, defaultBranch); err != nil {
			return nil, err
		}
		if err := c.isolateBuildOutput(ctx, workspacePath, cfg.BaseRepoDir); err != nil {
			return nil, err
		}
		c.linkWorkspaceDependencies(workspacePath, cfg.BaseRepoDir)
		return ws, nil
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

	// Recover an existing local/remote task branch before creating a new one.
	baseRef, err := c.bindWorkspaceBase(ctx, cfg.BaseRepoDir, workspacePath, defaultBranch)
	if err != nil {
		return nil, err
	}
	ws.BaseRef = baseRef
	if err := c.pinWorkspaceBase(ctx, ws); err != nil {
		return nil, err
	}
	if err := c.checkoutRetainedTaskBranch(ctx, workspacePath, branchName, ws.BaseCommit); err != nil {
		return nil, err
	}

	// Configure human author identity
	if err := c.configureWorkspaceGitIdentity(workspacePath, authorName, authorEmail); err != nil {
		return nil, err
	}

	// Install branch protection pre-push hook
	if err := c.installBranchProtectionHook(workspacePath, defaultBranch); err != nil {
		return nil, err
	}

	if err := c.isolateBuildOutput(ctx, workspacePath, cfg.BaseRepoDir); err != nil {
		return nil, err
	}
	c.linkWorkspaceDependencies(workspacePath, cfg.BaseRepoDir)

	return ws, nil
}

// Pin the imported branch before execution. Moving its remote-tracking alias
// later must not change which code/verification contract this attempt compares.
func (c *Client) pinWorkspaceBase(ctx context.Context, ws *TaskWorkspace) error {
	if ws.BaseRef == "" {
		return nil
	}
	raw, err := exec.CommandContext(ctx, c.gitBinary, "-C", ws.Path, "rev-parse", "--verify", "--end-of-options", ws.BaseRef+"^{commit}").Output()
	if err != nil {
		return errors.New("selected project baseline could not be pinned; saved task files were preserved")
	}
	ws.BaseCommit = strings.TrimSpace(string(raw))
	if !IsCommitHash(ws.BaseCommit) {
		return errors.New("selected project baseline commit is unavailable; saved task files were preserved")
	}
	return nil
}

// checkoutRetainedTaskBranch never rewinds an existing branch or discards files.
func (c *Client) checkoutRetainedTaskBranch(ctx context.Context, dir, branch, baseRef string) error {
	current, err := exec.CommandContext(ctx, c.gitBinary, "-C", dir, "symbolic-ref", "--quiet", "--short", "HEAD").Output()
	if err == nil && strings.TrimSpace(string(current)) == branch {
		return nil // Also preserve an in-progress merge/index on this branch.
	}
	args := []string{"-C", dir, "checkout"}
	local := exec.CommandContext(ctx, c.gitBinary, "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err := local.Run(); err == nil {
		args = append(args, branch)
	} else {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			return errors.New("retained workspace branch could not be inspected; saved files were preserved")
		}
		remote := exec.CommandContext(ctx, c.gitBinary, "-C", dir, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
		if err := remote.Run(); err == nil {
			args = append(args, "-b", branch, "--track", "origin/"+branch)
		} else {
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				return errors.New("retained task branch could not be verified; saved files were preserved")
			}
			args = append(args, "-b", branch)
			if baseRef != "" {
				args = append(args, baseRef)
			}
		}
	}
	if err := exec.CommandContext(ctx, c.gitBinary, args...).Run(); err != nil {
		return errors.New("task branch checkout needs reconciliation; saved files, commits and conflicts were preserved")
	}
	return nil
}

// Import the selected cache ref without changing its working tree or the
// retained task branch. A local clone's HEAD need not be the selected branch.
func (c *Client) bindWorkspaceBase(ctx context.Context, source, destination, branch string) (string, error) {
	var err error
	destination, err = filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if source == "" {
		source = destination
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return "", err
	}
	for _, ref := range []string{"refs/remotes/origin/" + branch, "refs/heads/" + branch} {
		if exec.CommandContext(ctx, c.gitBinary, "-C", source, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}").Run() != nil {
			continue
		}
		bound := "refs/remotes/cuan-base/" + branch
		if exec.CommandContext(ctx, c.gitBinary, "-C", destination, "fetch", "--no-tags", "--no-write-fetch-head", "--", source, "+"+ref+":"+bound).Run() != nil {
			return "", errors.New("current project base could not be read; retained task files were preserved")
		}
		return bound, nil
	}
	if source != destination {
		return c.bindWorkspaceBase(ctx, "", destination, branch)
	}
	if exec.CommandContext(ctx, c.gitBinary, "-C", destination, "rev-parse", "--verify", "HEAD^{commit}").Run() == nil {
		return "", errors.New("selected project base branch is unavailable; retained task files were preserved")
	}
	return "", nil // Empty/new repositories retain their initial branch behavior.
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
	if token != "" {
		cmd.Env = repositoryEnvironment(token)
	}
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
	out, err := c.snapshotGit(ctx, workspacePath, "diff", "--no-ext-diff", "--no-textconv", "--stat", baseBranch+"...HEAD")
	// A bound baseline is authoritative, including an empty diff. Never
	// substitute an unrelated previous commit when no task changes exist.
	if strings.HasPrefix(baseBranch, "refs/") || IsCommitHash(baseBranch) {
		return strings.TrimSpace(string(out)), err
	}
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return strings.TrimSpace(string(out)), nil
	}

	// Fallback to origin/baseBranch...HEAD
	outRemote, errRemote := c.snapshotGit(ctx, workspacePath, "diff", "--no-ext-diff", "--no-textconv", "--stat", "origin/"+baseBranch+"...HEAD")
	if errRemote == nil && len(strings.TrimSpace(string(outRemote))) > 0 {
		return strings.TrimSpace(string(outRemote)), nil
	}

	// Fallback to diff against HEAD~1
	outPrev, errPrev := c.snapshotGit(ctx, workspacePath, "diff", "--no-ext-diff", "--no-textconv", "--stat", "HEAD~1...HEAD")
	if errPrev == nil && len(strings.TrimSpace(string(outPrev))) > 0 {
		return strings.TrimSpace(string(outPrev)), nil
	}

	// Fallback to git status --short
	outStatus, _ := c.snapshotGit(ctx, workspacePath, "status", "--short")
	if len(strings.TrimSpace(string(outStatus))) > 0 {
		return strings.TrimSpace(string(outStatus)), nil
	}

	return "No file modifications recorded in diff.", nil
}

// PruneStaleWorkspaces scans workspacesBase and cleans up stale or excess task workspaces.
// It skips the current activeTaskID, hidden directories (such as .task-execution-locks),
// and any workspace whose task execution lock is actively held.
//
// Age and count nominate candidates, but deletion additionally requires a clean
// repository (including ignored files) and no commits beyond remote-tracking
// refs. Unknown or protected work can therefore exceed the count/age targets.
// The sweep is bounded to ten seconds and deletion holds the execution lock.
//
// Returns the number of workspaces pruned and any error encountered while reading the directory.
func (c *Client) PruneStaleWorkspaces(workspacesBase string, maxAge time.Duration, activeTaskID string, maxWorkspaces int) (int, error) {
	if workspacesBase == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(workspacesBase)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read workspaces base dir: %w", err)
	}

	locksDir := filepath.Join(workspacesBase, ".task-execution-locks")
	if err := os.MkdirAll(locksDir, 0700); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type candidate struct {
		taskID  string
		path    string
		modTime time.Time
	}

	var remaining []candidate
	prunedCount := 0

	for _, entry := range entries {
		if ctx.Err() != nil {
			return prunedCount, nil
		}
		name := entry.Name()
		// Skip files and hidden directories (e.g. .task-execution-locks)
		if !entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}

		// Never prune the currently active task
		if activeTaskID != "" && name == activeTaskID {
			continue
		}

		// Check if the task has an active OS execution lock
		lockFile := filepath.Join(locksDir, name+".lock")
		if active, aErr := taskprocess.Active(lockFile); aErr == nil && active {
			continue
		}

		wsPath := filepath.Join(workspacesBase, name)
		info, iErr := entry.Info()
		if iErr != nil {
			statInfo, sErr := os.Stat(wsPath)
			if sErr != nil {
				continue
			}
			info = statInfo
		}

		cand := candidate{
			taskID:  name,
			path:    wsPath,
			modTime: info.ModTime(),
		}

		// Phase 1: Prune if older than maxAge
		if maxAge > 0 && time.Since(cand.modTime) > maxAge {
			if c.pruneRecreatableWorkspace(ctx, cand.path, lockFile) {
				prunedCount++
			}
			continue
		}

		remaining = append(remaining, cand)
	}

	// Phase 2: FIFO rotation if count exceeds maxWorkspaces
	if maxWorkspaces > 0 && len(remaining) > maxWorkspaces {
		// Sort remaining ascending by modTime (oldest first)
		sort.Slice(remaining, func(i, j int) bool {
			return remaining[i].modTime.Before(remaining[j].modTime)
		})

		excess := len(remaining) - maxWorkspaces
		for i := 0; i < excess; i++ {
			lockFile := filepath.Join(locksDir, remaining[i].taskID+".lock")
			if c.pruneRecreatableWorkspace(ctx, remaining[i].path, lockFile) {
				prunedCount++
			}
		}
	}

	return prunedCount, nil
}

// Age/count alone never prove saved work is disposable. Keep unknown repos,
// tracked/untracked/ignored files, conflicts, stashes and unpublished commits.
// Hold the same execution lock throughout inspection and deletion; never unlink
// its inode while another worker may be waiting to acquire it.
func (c *Client) pruneRecreatableWorkspace(ctx context.Context, dir, lockPath string) bool {
	absLock, err := filepath.Abs(lockPath)
	if err != nil {
		return false
	}
	lock, err := taskprocess.Acquire(absLock)
	if err != nil {
		return false
	}
	defer lock.Close()
	status, err := exec.CommandContext(ctx, c.gitBinary, "-C", dir, "status", "--porcelain", "--untracked-files=normal", "--ignored=matching").Output()
	if err != nil || len(bytes.TrimSpace(status)) != 0 {
		return false
	}
	local, err := exec.CommandContext(ctx, c.gitBinary, "-C", dir, "rev-list", "--max-count=1", "--all", "HEAD", "--not", "--remotes").Output()
	if err != nil || len(bytes.TrimSpace(local)) != 0 || ctx.Err() != nil {
		return false
	}
	return os.RemoveAll(dir) == nil
}

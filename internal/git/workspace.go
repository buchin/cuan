package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/buchin/agentrunner"
	"github.com/buchin/cuan/internal/agent"
)

type TaskWorkspace = agentrunner.TaskWorkspace
type WorkspaceConfig = agentrunner.WorkspaceConfig

var (
	ErrDefaultBranchProtected = agentrunner.ErrDefaultBranchProtected
	ErrWorkspaceEmpty         = agentrunner.ErrWorkspaceEmpty
	ErrNoCommitsToPush        = agentrunner.ErrNoCommitsToPush
)

// PrepareTaskWorkspace ensures an isolated workspace directory exists for the given task
// utilizing the standalone github.com/buchin/agentrunner library.
func (c *Client) PrepareTaskWorkspace(ctx context.Context, cfg WorkspaceConfig) (*TaskWorkspace, error) {
	return agentrunner.PrepareTaskWorkspace(ctx, c.gitBinary, cfg)
}

// configureWorkspaceGitIdentity sets user.name and user.email in local repository config.
func (c *Client) configureWorkspaceGitIdentity(workspacePath, name, email string) error {
	return agentrunner.ConfigureWorkspaceGitIdentity(c.gitBinary, workspacePath, name, email)
}

// installBranchProtectionHook places an executable pre-push hook in the workspace repository
// that rejects direct pushes to the default branch (main/master).
func (c *Client) installBranchProtectionHook(workspacePath, defaultBranch string) error {
	return agentrunner.InstallBranchProtectionHook(workspacePath, defaultBranch)
}

// linkWorkspaceDependencies links node_modules and built web assets into the workspace.
func (c *Client) linkWorkspaceDependencies(workspacePath, baseRepoDir string) {
	agentrunner.LinkWorkspaceDependencies(workspacePath, baseRepoDir)
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

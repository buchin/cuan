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
	"time"

	"github.com/buchin/cuan/internal/agent"
)

var (
	ErrWorkingTreeDirty = errors.New("working tree has uncommitted changes")
	ErrBranchesDiverged = errors.New("local branch and origin have diverged")
)

// PreflightSync safely fetches and fast-forwards the target branch (default "dev")
// from origin/<branch> in repoDir if the working tree is clean.
// It is non-destructive: if there are uncommitted changes, diverged branches,
// or network errors, it returns an informative error without panicking or modifying files.
func (c *Client) PreflightSync(ctx context.Context, repoDir, branch, token string) error {
	if repoDir == "" {
		return errors.New("repository directory is required")
	}
	if branch == "" {
		branch = "dev"
	}

	gitDir := filepath.Join(repoDir, ".git")
	fi, err := os.Stat(gitDir)
	if err != nil || (!fi.IsDir() && !fi.Mode().IsRegular()) {
		return fmt.Errorf("not a git repository: %s", repoDir)
	}

	// 1. Safe working tree check: working tree must be clean
	statusCmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "status", "--porcelain=v1", "--untracked-files=all")
	statusOut, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}
	if len(bytes.TrimSpace(statusOut)) > 0 {
		return ErrWorkingTreeDirty
	}

	// 2. Check if origin remote is configured
	remoteCmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "remote", "get-url", "origin")
	if err := remoteCmd.Run(); err != nil {
		return errors.New("origin remote is not configured")
	}

	// 3. Fetch target branch from origin with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	refspec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", branch, branch)
	fetchCmd := exec.CommandContext(fetchCtx, c.gitBinary, "-C", repoDir, "fetch", "--no-tags", "origin", refspec)
	fetchCmd.Env = repositoryEnvironment(token)
	var stderr bytes.Buffer
	fetchCmd.Stderr = &stderr
	if err := fetchCmd.Run(); err != nil {
		masked := agent.MaskSecrets(stderr.String(), []string{token})
		return fmt.Errorf("git fetch origin %s failed: %s (%w)", branch, strings.TrimSpace(masked), err)
	}

	// 4. Verify remote-tracking commit
	remoteRef := fmt.Sprintf("refs/remotes/origin/%s", branch)
	remoteShaBytes, err := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "rev-parse", "--verify", remoteRef+"^{commit}").Output()
	if err != nil {
		return fmt.Errorf("remote branch %s is not available", remoteRef)
	}
	remoteSHA := strings.TrimSpace(string(remoteShaBytes))

	// 5. Verify local branch ref
	localRef := fmt.Sprintf("refs/heads/%s", branch)
	localShaBytes, err := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "rev-parse", "--verify", localRef+"^{commit}").Output()
	if err != nil {
		// Local branch does not exist yet; create it tracking the remote ref
		branchCmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "branch", branch, remoteRef)
		if out, err := branchCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create local branch %s failed: %s (%w)", branch, string(out), err)
		}
		return nil
	}
	localSHA := strings.TrimSpace(string(localShaBytes))
	if localSHA == remoteSHA {
		// Already up to date
		return nil
	}

	// 6. Check ancestry
	isAncestorCmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "merge-base", "--is-ancestor", localRef, remoteRef)
	if err := isAncestorCmd.Run(); err != nil {
		// Local is not ancestor of remote. Check if remote is ancestor of local (local is ahead)
		isAheadCmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "merge-base", "--is-ancestor", remoteRef, localRef)
		if isAheadCmd.Run() == nil {
			// Local is ahead of remote; do not overwrite local commits
			return nil
		}
		return ErrBranchesDiverged
	}

	// 7. Fast-forward safely
	currentBranchBytes, _ := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "symbolic-ref", "--short", "HEAD").Output()
	currentBranch := strings.TrimSpace(string(currentBranchBytes))

	if currentBranch == branch {
		// Current checked-out branch is target branch and working tree is clean: ff-only merge
		mergeCmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "merge", "--ff-only", remoteRef)
		if out, err := mergeCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("fast-forward merge %s failed: %s (%w)", branch, string(out), err)
		}
	} else {
		// HEAD is on another branch or detached: update local ref directly without altering working tree
		updateCmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoDir, "update-ref", localRef, remoteRef)
		if out, err := updateCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("update-ref %s failed: %s (%w)", branch, string(out), err)
		}
	}

	return nil
}

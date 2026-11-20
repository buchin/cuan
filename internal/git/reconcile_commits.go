package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ReconcileCommits merges two parent commits (e.g. an active running release and a newly
// merged target commit) in memory using git merge-tree and commit-tree without touching
// any worktree or index. It returns the newly created commit SHA.
func (c *Client) ReconcileCommits(ctx context.Context, repoPath, parent1, parent2, message string) (string, error) {
	if !IsCommitHash(parent1) || !IsCommitHash(parent2) {
		return "", errors.New("reconcile requires valid commit hashes")
	}
	if parent1 == parent2 {
		return parent1, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoPath, "merge-tree", "--write-tree", parent1, parent2)
	cmd.Env = verificationGitEnvironment()
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("reconcile merge conflict or failure: %s %w", strings.TrimSpace(errBuf.String()), err)
	}
	tree := strings.TrimSpace(string(out))
	if !IsCommitHash(tree) {
		return "", fmt.Errorf("invalid merge-tree output: %s", tree)
	}

	commitCmd := exec.CommandContext(ctx, c.gitBinary, "-C", repoPath, "commit-tree", tree, "-p", parent1, "-p", parent2, "-m", message)
	commitCmd.Env = verificationGitEnvironment()
	var commitErrBuf strings.Builder
	commitCmd.Stderr = &commitErrBuf
	commitOut, err := commitCmd.Output()
	if err != nil {
		return "", fmt.Errorf("reconcile commit-tree failed: %s %w", strings.TrimSpace(commitErrBuf.String()), err)
	}
	newCommit := strings.TrimSpace(string(commitOut))
	if !IsCommitHash(newCommit) {
		return "", fmt.Errorf("invalid commit-tree output: %s", newCommit)
	}
	return newCommit, nil
}

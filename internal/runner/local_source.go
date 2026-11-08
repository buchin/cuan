package runner

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/buchin/cuan/internal/git"
)

// Observe a clean local developer checkpoint even when publication is forbidden.
// This reads source identity only: it never commits, pushes, prepares a release
// or claims deployment. Dirty work cannot be attributed to HEAD.
func (r *Runner) observeLocalImplementationSource(ctx context.Context, ws *git.TaskWorkspace, defaultBranch string, result *CompleteAttemptRequest) error {
	if ws == nil || ws.Path == "" || r.gitClient == nil {
		return nil
	}
	changed, summary, err := r.detectRepoCodeChanges(ctx, ws, defaultBranch)
	if err != nil {
		return err
	}
	clean, err := exec.CommandContext(ctx, "git", "-C", ws.Path, "status", "--porcelain", "--untracked-files=normal").Output()
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(clean)) != 0 || strings.TrimSpace(summary) == "" {
		return nil
	}
	commit, err := r.gitClient.GetLatestCommitSHA(ctx, ws.Path)
	if err != nil {
		return err
	}
	branch, err := exec.CommandContext(ctx, "git", "-C", ws.Path, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return err
	}
	result.CommitSHA, result.BranchName, result.DiffSummary = commit, strings.TrimSpace(string(branch)), summary
	result.HasCodeChanges = &changed
	return nil
}

package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// PushExecutionSource publishes the admitted immutable SHA, never a later HEAD.
// Existing exact remote state is reconciled before writing. Caller must derive
// workspace/branch/repository/source from the original authorized task.
func (c *Client) PushExecutionSource(ctx context.Context, path, repoURL, branch, defaultBranch, source, token string) (string, error) {
	if !IsCommitHash(source) || strings.TrimSpace(token) == "" {
		return "", errors.New("exact source and saved repository access required")
	}
	if branch == "" || branch == defaultBranch || branch == "main" || branch == "master" || strings.HasPrefix(branch, "-") {
		return "", ErrDefaultBranchProtected
	}
	if _, err := c.snapshotGit(ctx, "", "check-ref-format", "--branch", branch); err != nil {
		return "", errors.New("invalid task publication branch")
	}
	owner, repo, err := ParseGitHubURL(repoURL)
	if err != nil {
		return "", err
	}
	origin, err := c.snapshotGit(ctx, path, "remote", "get-url", "origin")
	if err != nil {
		return "", errors.New("task repository identity unavailable")
	}
	actualOwner, actualRepo, err := ParseGitHubURL(strings.TrimSpace(string(origin)))
	if err != nil || !strings.EqualFold(owner, actualOwner) || !strings.EqualFold(repo, actualRepo) {
		return "", errors.New("task repository differs from admitted repository")
	}
	current, err := c.snapshotGit(ctx, path, "symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(string(current)) != branch {
		return "", errors.New("task publication branch changed")
	}
	if err := c.VerifySnapshotCommit(ctx, path, source, source); err != nil {
		return "", err
	}
	ref := "refs/heads/" + branch
	remote, err := c.observeExecutionSource(ctx, path, owner, repo, ref, token)
	if err != nil {
		return "", err
	}
	if remote == source {
		return source, nil
	}
	immutable := remote != ""
	if immutable {
		// Reconciliation can rebase the task checkpoint after an earlier source
		// was pushed. Preserve that branch/history; publish the reviewed SHA at
		// its own immutable ref rather than force-pushing somebody's work.
		ref = "refs/heads/cuan/verified-source/" + source
		remote, err = c.observeExecutionSource(ctx, path, owner, repo, ref, token)
		if err != nil {
			return "", err
		}
		if remote == source {
			return source, nil
		}
		if remote != "" {
			return "", errors.New("immutable source reference differs; existing remote work preserved")
		}
	}
	args := []string{"-c", "core.fsmonitor=false", "-C", path, "push", "--porcelain"}
	if immutable {
		// Creation-only compare-and-swap: a concurrent writer must not be
		// overwritten, even if their value would permit a normal fast-forward.
		args = append(args, "--force-with-lease="+ref+":")
	}
	args = append(args, "--", c.repoURL(owner, repo), source+":"+ref)
	command := exec.CommandContext(ctx, c.gitBinary, args...)
	command.Env = repositoryEnvironment(token)
	if err := command.Run(); err != nil {
		return "", errors.New("exact source push not confirmed; reconcile remote state before retrying")
	}
	return source, nil
}

func (c *Client) observeExecutionSource(ctx context.Context, path, owner, repo, ref, token string) (string, error) {
	observe := exec.CommandContext(ctx, c.gitBinary, "-c", "core.fsmonitor=false", "-C", path, "ls-remote", "--refs", "--exit-code", "--", c.repoURL(owner, repo), ref)
	observe.Env = repositoryEnvironment(token)
	var output snapshotOutput
	observe.Stdout = &output
	if err := observe.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 {
			return "", errors.New("remote source state unavailable; push not started")
		}
		return "", nil
	} else {
		fields := strings.Fields(string(output.data))
		if output.overflow || len(fields) != 2 || !IsCommitHash(fields[0]) || fields[1] != ref {
			return "", errors.New("remote branch observation invalid; push not started")
		}
		return fields[0], nil
	}
}

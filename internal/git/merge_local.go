package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// MergeLocalBranch is only for a project without a remote repository. It never
// fetches, discards work, retries a conflict, or merges a moving branch name.
func (c *Client) MergeLocalBranch(parent context.Context, root, base, branch, source, message string) (*MergePullRequestResult, error) {
	if !IsCommitHash(source) || base == "" || branch == "" || branch == base {
		return nil, errors.New("distinct target/task branches and exact source commit are required")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	lease, err := AcquireVerificationLease(ctx, root)
	if err != nil {
		return nil, err
	}
	defer lease.Close()
	run := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, c.gitBinary, append([]string{"-C", root}, args...)...)
		cmd.Env = repositoryEnvironment("")
		var output snapshotOutput
		cmd.Stdout = &output
		err := cmd.Run()
		if output.overflow {
			return "", errors.New("repository inspection exceeded its output budget")
		}
		return strings.TrimSpace(string(output.data)), err
	}
	for _, name := range []string{base, branch} {
		if _, err := run("check-ref-format", "--branch", name); err != nil {
			return nil, errors.New("registered branch name is invalid; saved work was preserved")
		}
	}
	status, err := run("status", "--porcelain=v1", "--untracked-files=all")
	if err != nil || status != "" {
		return nil, errors.New("repository has pending work or cannot be inspected; saved files were preserved")
	}
	head, err := run("rev-parse", "--verify", "refs/heads/"+branch+"^{commit}")
	if err != nil || head != source {
		return nil, errors.New("task branch differs from its recorded source; recheck the result before applying")
	}
	if _, err := run("show-ref", "--verify", "refs/heads/"+base); err != nil {
		return nil, errors.New("registered target branch is unavailable; no merge was started")
	}
	if _, err := run("checkout", "--no-guess", base); err != nil {
		return nil, errors.New("target checkout failed; no merge was started and saved work was preserved")
	}
	current, err := run("symbolic-ref", "--short", "HEAD")
	if err != nil || current != base {
		return nil, errors.New("target branch changed; no merge was started")
	}
	if _, err := run("merge", "--no-ff", "-m", message, "--", source); err != nil {
		return nil, errors.New("local merge did not complete; retained conflicts and files need reconciliation")
	}
	merged, err := run("rev-parse", "--verify", "HEAD^{commit}")
	if err != nil || !IsCommitHash(merged) {
		return nil, errors.New("local merge identity could not be confirmed; reconcile current repository state")
	}
	return &MergePullRequestResult{Merged: true, SHA: merged, Message: "Exact source applied locally; no remote publication or deployment performed"}, nil
}

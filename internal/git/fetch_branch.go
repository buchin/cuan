package git

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

// ErrRepositoryDownloadUnavailable is emitted only before any task execution.
var ErrRepositoryDownloadUnavailable = errors.New("Project files could not be downloaded. Your request is saved; Cuan has not started the work.")

// FetchBoundBranch prepares one remote base ref without checkout, reset, merge,
// or altering the working files. A saved task's branch choice is never replaced.
func (c *Client) FetchBoundBranch(ctx context.Context, path, repoURL, branch, token string) error {
	owner, repo, err := ParseGitHubURL(repoURL)
	if err != nil {
		return err
	}
	if branch == "" || strings.HasPrefix(branch, "-") || exec.CommandContext(ctx, c.gitBinary, "check-ref-format", "--branch", branch).Run() != nil {
		return errors.New("This project's saved repository version is invalid. Check its repository settings.")
	}
	command := exec.CommandContext(ctx, c.gitBinary, "-C", path, "fetch", "--no-tags", "--", c.repoURL(owner, repo), "+refs/heads/"+branch+":refs/remotes/origin/"+branch)
	command.Env = repositoryEnvironment(token)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err = command.Run(); err != nil {
		detail := strings.ToLower(stderr.String())
		if strings.Contains(detail, "couldn't find remote ref") {
			return errors.New("This project's selected version no longer exists. Choose an available version in project settings.")
		}
		if strings.Contains(detail, "authentication") || strings.Contains(detail, "could not read username") || strings.Contains(detail, "repository not found") || strings.Contains(detail, "access denied") {
			return errors.New("Connect GitHub in Settings to access this project's files. Your request is saved.")
		}
		return ErrRepositoryDownloadUnavailable
	}
	return nil
}

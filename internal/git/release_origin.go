package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// BindReleaseOrigin records credential-free source identity in a prepared clone.
// It supplies build metadata only; saved-owner remote observation owns sync proof.
func (c *Client) BindReleaseOrigin(ctx context.Context, root, repository string) error {
	owner, repo, err := ParseGitHubURL(repository)
	if err != nil {
		return err
	}
	canonical := fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
	read := func() (string, error) {
		raw, err := c.snapshotGit(ctx, root, "config", "--local", "--get", "remote.origin.url")
		return strings.TrimSpace(string(raw)), err
	}
	existing, err := read()
	if err == nil {
		if !strings.EqualFold(existing, canonical) {
			return errors.New("prepared release origin changed")
		}
		return nil
	}
	var missing *exec.ExitError
	if !errors.As(err, &missing) || missing.ExitCode() != 1 {
		return errors.New("prepared release origin unavailable")
	}
	if _, err := c.snapshotGit(ctx, root, "remote", "add", "origin", canonical); err != nil {
		// A concurrent identical preparation may have written its public identity.
		if observed, readErr := read(); readErr == nil && strings.EqualFold(observed, canonical) {
			return nil
		}
		return errors.New("prepared release origin could not be recorded")
	}
	return nil
}

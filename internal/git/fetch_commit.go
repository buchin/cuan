package git

import (
	"context"
	"encoding/base64"
	"errors"
	"os/exec"
	"strings"
)

// FetchVerificationCommit obtains one exact revision through the project's
// explicit connection. It updates objects only, without checkout, working-file,
// index, remote-ref or FETCH_HEAD changes. No ambient provider login is used.
func (c *Client) FetchVerificationCommit(ctx context.Context, path, repoURL, commit, token string) error {
	if !snapshotObjectID.MatchString(commit) {
		return errors.New("verification requires an exact repository commit")
	}
	owner, repo, err := ParseGitHubURL(repoURL)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, c.gitBinary, "-c", "credential.helper=", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", path, "fetch", "--no-tags", "--no-auto-maintenance", "--no-write-fetch-head", "--recurse-submodules=no", "--", c.repoURL(owner, repo), commit)
	cmd.Env = verificationGitEnvironment()
	if token != "" {
		cmd.Env = append(cmd.Env, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraheader", "GIT_CONFIG_VALUE_0=Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token)))
	}
	var diagnostic snapshotOutput
	cmd.Stderr = &diagnostic
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		message := strings.ToLower(string(diagnostic.data))
		if strings.Contains(message, "authentication") || strings.Contains(message, "could not read username") || strings.Contains(message, "repository not found") || strings.Contains(message, "access denied") {
			return errors.New("Connect GitHub in Settings to access this project's files. Your request is saved.")
		}
		return ErrRepositoryDownloadUnavailable
	}
	return nil
}

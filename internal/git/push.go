package git

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func repositoryEnvironment(token string) []string {
	environment := []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_CONFIG_") {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "GIT_TERMINAL_PROMPT=0")
	if token != "" {
		environment = append(environment, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraheader", "GIT_CONFIG_VALUE_0=Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token)))
	}
	return environment
}

// PushBoundBranch pushes exactly one managed branch, with no force or arbitrary refspec.
func (c *Client) PushBoundBranch(ctx context.Context, path, repoURL, branch, token string) (string, error) {
	if !regexp.MustCompile(`^cuan/task-[a-f0-9]{64}$`).MatchString(branch) {
		return "", errors.New("Only the bound task branch can be pushed")
	}
	owner, repo, err := ParseGitHubURL(repoURL)
	if err != nil {
		return "", err
	}
	current, err := exec.CommandContext(ctx, c.gitBinary, "-C", path, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(current)) != branch {
		return "", errors.New("Task branch changed; push was not started")
	}
	head, err := exec.CommandContext(ctx, c.gitBinary, "-C", path, "rev-parse", "refs/heads/"+branch).Output()
	if err != nil {
		return "", errors.New("Task commit is unavailable")
	}
	cmd := exec.CommandContext(ctx, c.gitBinary, "-C", path, "push", "--porcelain", "--", BuildAuthURL(owner, repo, ""), strings.TrimSpace(string(head))+":refs/heads/"+branch)
	cmd.Env = repositoryEnvironment(token)
	if err = cmd.Run(); err != nil {
		return "", errors.New("GitHub push was not confirmed. Check access or branch conflicts; retrying this push does not recreate commits")
	}
	return strings.TrimSpace(string(head)), nil
}

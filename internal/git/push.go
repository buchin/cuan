package git

import (
	"context"
	"encoding/base64"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func repositoryEnvironment(token string) []string {
	settings := [][2]string{{"credential.helper", ""}, {"core.hooksPath", "/dev/null"}}
	if token != "" {
		header := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token))
		settings = append([][2]string{{"http.https://github.com/.extraheader", header}}, settings...)
	}
	environment := append(verificationGitEnvironment(), "GIT_ALLOW_PROTOCOL=https:http:file", "GIT_CONFIG_COUNT="+strconv.Itoa(len(settings)))
	for i, setting := range settings {
		index := strconv.Itoa(i)
		environment = append(environment, "GIT_CONFIG_KEY_"+index+"="+setting[0], "GIT_CONFIG_VALUE_"+index+"="+setting[1])
	}
	return environment
}

// PushBoundBranch pushes exactly one managed branch, with no force or arbitrary refspec.
func (c *Client) PushBoundBranch(ctx context.Context, path, repoURL, branch, token string) (string, error) {
	if !regexp.MustCompile(`^cuan/task-[a-f0-9]{64}$`).MatchString(branch) {
		return "", errors.New("Only the bound task branch can be pushed")
	}
	return c.pushExactBranch(ctx, path, repoURL, branch, token)
}

// PushExecutionBranch accepts only a server-bound execution branch. The caller
// must derive path/branch from the authenticated task, never request arguments.
// Retained native clones must still identify the saved repository: a changed
// project connection cannot redirect historical work into another repository.
func (c *Client) PushExecutionBranch(ctx context.Context, path, repoURL, branch, defaultBranch, token string) (string, error) {
	if branch == "" || branch == defaultBranch || branch == "main" || branch == "master" {
		return "", ErrDefaultBranchProtected
	}
	if err := exec.CommandContext(ctx, c.gitBinary, "check-ref-format", "--branch", branch).Run(); err != nil {
		return "", errors.New("The bound task branch is invalid")
	}
	origin, err := exec.CommandContext(ctx, c.gitBinary, "-C", path, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", errors.New("The execution repository identity is unavailable")
	}
	wantOwner, wantRepo, wantErr := ParseGitHubURL(repoURL)
	owner, repo, err := ParseGitHubURL(strings.TrimSpace(string(origin)))
	if err != nil || wantErr != nil || !strings.EqualFold(owner, wantOwner) || !strings.EqualFold(repo, wantRepo) {
		return "", errors.New("The execution repository differs from the saved task repository. Local work is preserved")
	}
	return c.pushExactBranch(ctx, path, repoURL, branch, token)
}

func (c *Client) pushExactBranch(ctx context.Context, path, repoURL, branch, token string) (string, error) {
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
	cmd := exec.CommandContext(ctx, c.gitBinary, "-C", path, "push", "--porcelain", "--", c.repoURL(owner, repo), strings.TrimSpace(string(head))+":refs/heads/"+branch)
	cmd.Env = repositoryEnvironment(token)
	if err = cmd.Run(); err != nil {
		return "", errors.New("GitHub push was not confirmed. Check access or branch conflicts; retrying this push does not recreate commits")
	}
	return strings.TrimSpace(string(head)), nil
}

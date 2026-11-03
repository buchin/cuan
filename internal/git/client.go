package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

var (
	ErrInvalidRepoURL = errors.New("invalid GitHub repository URL. Use format 'https://github.com/owner/repo' or 'owner/repo'")
	ErrRepoNotFound   = errors.New("repository not found on GitHub. Check owner/repo name or permissions for private repository")
	ErrAccessDenied   = errors.New("GitHub access denied (401/403). Check Personal Access Token (PAT) validity and repository permissions")
)

type RepoInfo struct {
	Owner         string   `json:"owner"`
	Repo          string   `json:"repo"`
	FullName      string   `json:"full_name"`
	CloneURL      string   `json:"clone_url"`
	DefaultBranch string   `json:"default_branch"`
	IsPrivate     bool     `json:"is_private"`
	Branches      []string `json:"branches"`
}

type Client struct {
	httpClient *http.Client
	gitBinary  string
	apiBaseURL string
	gitBaseURL string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		gitBinary:  "git",
		apiBaseURL: "https://api.github.com",
	}
}

func (c *Client) SetAPIBaseURL(url string) {
	c.apiBaseURL = strings.TrimRight(url, "/")
}

func (c *Client) SetGitBaseURL(url string) {
	c.gitBaseURL = strings.TrimRight(url, "/")
}

func (c *Client) repoURL(owner, repo string) string {
	if c != nil && c.gitBaseURL != "" {
		return fmt.Sprintf("%s/%s/%s.git", c.gitBaseURL, owner, repo)
	}
	return BuildAuthURL(owner, repo, "")
}

// ParseGitHubURL parses various URL formats (HTTPS, github.com shorthand, owner/repo)
// and extracts the owner and repo name.
func ParseGitHubURL(rawURL string) (owner string, repo string, err error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", "", ErrInvalidRepoURL
	}

	trimmed = strings.TrimSuffix(trimmed, ".git")

	// Pattern 1: owner/repo
	shortRegex := regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)
	if shortRegex.MatchString(trimmed) {
		parts := strings.Split(trimmed, "/")
		return parts[0], parts[1], nil
	}

	// Pattern 2: full URL (https://github.com/owner/repo or http://github.com/owner/repo)
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "https://" + trimmed
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", "", ErrInvalidRepoURL
	}

	if !strings.EqualFold(u.Host, "github.com") && !strings.EqualFold(u.Host, "www.github.com") {
		return "", "", ErrInvalidRepoURL
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" || pathParts[1] == "" {
		return "", "", ErrInvalidRepoURL
	}

	return pathParts[0], pathParts[1], nil
}

// BuildAuthURL constructs an authenticated clone URL if token is present,
// or standard https URL if token is empty.
func BuildAuthURL(owner, repo, token string) string {
	if strings.TrimSpace(token) != "" {
		return fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", token, owner, repo)
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
}

// ValidateRepository inspects GitHub repository metadata, checking access permissions,
// existence, default branch, and remote branches.
func (c *Client) ValidateRepository(ctx context.Context, repoInput, token string) (*RepoInfo, error) {
	owner, repo, err := ParseGitHubURL(repoInput)
	if err != nil {
		return nil, err
	}

	// 1. Check via GitHub REST API if possible
	apiURL := fmt.Sprintf("%s/repos/%s/%s", c.apiBaseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create github request: %w", err)
	}

	req.Header.Set("User-Agent", "Cuan-Engine/1.0")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	resp, err := c.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrAccessDenied
		}
		if resp.StatusCode == http.StatusForbidden {
			return nil, ErrAccessDenied
		}
		if resp.StatusCode == http.StatusNotFound {
			// If GitHub REST API returns 404, it might be a private repository accessible via git
			// credential helper or local git configuration. Fallback to git ls-remote before failing.
			branches, defaultBranch, lsErr := c.lsRemote(ctx, owner, repo, token)
			if lsErr == nil && len(branches) > 0 {
				return &RepoInfo{
					Owner:         owner,
					Repo:          repo,
					FullName:      fmt.Sprintf("%s/%s", owner, repo),
					CloneURL:      fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
					DefaultBranch: defaultBranch,
					IsPrivate:     true,
					Branches:      branches,
				}, nil
			}
			if token != "" && errors.Is(lsErr, ErrAccessDenied) {
				return nil, ErrAccessDenied
			}
			return nil, ErrRepoNotFound
		}

		if resp.StatusCode == http.StatusOK {
			var ghResp struct {
				FullName      string `json:"full_name"`
				DefaultBranch string `json:"default_branch"`
				Private       bool   `json:"private"`
				CloneURL      string `json:"clone_url"`
			}
			if decodeErr := json.NewDecoder(resp.Body).Decode(&ghResp); decodeErr == nil {
				defaultBranch := ghResp.DefaultBranch
				if defaultBranch == "" {
					defaultBranch = "main"
				}

				branches, _ := c.FetchRemoteBranches(ctx, owner, repo, token)
				if len(branches) == 0 {
					branches = []string{defaultBranch}
				}

				return &RepoInfo{
					Owner:         owner,
					Repo:          repo,
					FullName:      fmt.Sprintf("%s/%s", owner, repo),
					CloneURL:      fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
					DefaultBranch: defaultBranch,
					IsPrivate:     ghResp.Private,
					Branches:      branches,
				}, nil
			}
		}
	}

	// 2. Fallback to git ls-remote if GitHub API is unreachable or rate-limited
	branches, defaultBranch, err := c.lsRemote(ctx, owner, repo, token)
	if err != nil {
		return nil, err
	}

	return &RepoInfo{
		Owner:         owner,
		Repo:          repo,
		FullName:      fmt.Sprintf("%s/%s", owner, repo),
		CloneURL:      fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
		DefaultBranch: defaultBranch,
		IsPrivate:     token != "",
		Branches:      branches,
	}, nil
}

// FetchRemoteBranches returns the list of branch names for the given repository.
func (c *Client) FetchRemoteBranches(ctx context.Context, owner, repo, token string) ([]string, error) {
	branches, _, err := c.lsRemote(ctx, owner, repo, token)
	return branches, err
}

func (c *Client) lsRemote(ctx context.Context, owner, repo, token string) ([]string, string, error) {
	authURL := BuildAuthURL(owner, repo, token)

	lsCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(lsCtx, c.gitBinary, "ls-remote", "--heads", "--symref", authURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	secretsToMask := []string{}
	if token != "" {
		secretsToMask = append(secretsToMask, token, "x-access-token:"+token)
	}

	cleanStderr := agent.MaskSecrets(stderr.String(), secretsToMask)

	if runErr != nil {
		if errors.Is(lsCtx.Err(), context.DeadlineExceeded) || errors.Is(runErr, context.DeadlineExceeded) {
			return nil, "", fmt.Errorf("git ls-remote timed out: %w", context.DeadlineExceeded)
		}
		lowerErr := strings.ToLower(cleanStderr)
		if strings.Contains(lowerErr, "not found") || strings.Contains(lowerErr, "repository not found") {
			return nil, "", ErrRepoNotFound
		}
		if strings.Contains(lowerErr, "invalid username or password") ||
			strings.Contains(lowerErr, "authentication failed") ||
			strings.Contains(lowerErr, "permission to") ||
			strings.Contains(lowerErr, "access denied") ||
			strings.Contains(lowerErr, "could not read username") {
			return nil, "", ErrAccessDenied
		}
		return nil, "", fmt.Errorf("git ls-remote failed: %s", cleanStderr)
	}

	lines := strings.Split(stdout.String(), "\n")
	var branches []string
	defaultBranch := "main"

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ref: refs/heads/") && strings.Contains(line, "HEAD") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				branch := strings.TrimPrefix(parts[1], "refs/heads/")
				if branch != "" {
					defaultBranch = branch
				}
			}
		}

		if strings.Contains(line, "refs/heads/") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				branch := strings.TrimPrefix(parts[1], "refs/heads/")
				if branch != "" && !sliceContains(branches, branch) {
					branches = append(branches, branch)
				}
			}
		}
	}

	if len(branches) == 0 {
		branches = []string{defaultBranch}
	}

	return branches, defaultBranch, nil
}

// CloneOrFetch clones repository if targetDir does not exist, or fetches updates if already cloned.
// Kredensial dipastikan aman dan tidak pernah muncul di output log maupun error.
func (c *Client) CloneOrFetch(ctx context.Context, owner, repo, token, targetDir string) error {
	secretsToMask := []string{}
	if token != "" {
		secretsToMask = append(secretsToMask, token, "x-access-token:"+token)
	}

	repoURL := c.repoURL(owner, repo)
	authenticate := func(cmd *exec.Cmd) { cmd.Env = repositoryEnvironment(token) }

	gitDir := filepath.Join(targetDir, ".git")
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		// Remove credentials left by older clones and fetch only the intended remote.
		if exec.CommandContext(ctx, c.gitBinary, "-C", targetDir, "remote", "set-url", "origin", repoURL).Run() != nil {
			// Older provisioned caches can be an empty git init without origin. Adding
			// the intended remote does not checkout or discard existing local files.
			if exec.CommandContext(ctx, c.gitBinary, "-C", targetDir, "remote", "add", "origin", repoURL).Run() != nil {
				return errors.New("Could not prepare the repository connection")
			}
		}
		cmd := exec.CommandContext(ctx, c.gitBinary, "-C", targetDir, "fetch", "origin", "--prune")
		authenticate(cmd)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			masked := agent.MaskSecrets(stderr.String(), secretsToMask)
			return fmt.Errorf("git fetch failed: %s", masked)
		}
		return nil
	}

	// New clone
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, c.gitBinary, "clone", "--quiet", repoURL, targetDir)
	authenticate(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		masked := agent.MaskSecrets(stderr.String(), secretsToMask)
		lowerErr := strings.ToLower(masked)
		if strings.Contains(lowerErr, "not found") {
			return ErrRepoNotFound
		}
		if strings.Contains(lowerErr, "authentication failed") || strings.Contains(lowerErr, "access denied") {
			return ErrAccessDenied
		}
		return fmt.Errorf("git clone failed: %s", masked)
	}

	return nil
}

func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

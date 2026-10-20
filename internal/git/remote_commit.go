package git

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
)

// RemoteBranchHead reads the live GitHub ref rather than trusting a cached fetch.
func (c *Client) RemoteBranchHead(ctx context.Context, repoURL, branch, token string) (string, error) {
	owner, repo, err := ParseGitHubURL(repoURL)
	if err != nil {
		return "", err
	}
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err = c.accountRequest(ctx, "/repos/"+owner+"/"+repo+"/git/ref/heads/"+url.PathEscape(branch), token, &ref); err != nil {
		return "", err
	}
	if !regexp.MustCompile(`^[a-f0-9]{40,64}$`).MatchString(ref.Object.SHA) {
		return "", fmt.Errorf("GitHub returned an invalid commit")
	}
	return ref.Object.SHA, nil
}

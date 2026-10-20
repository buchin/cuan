package git

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Account identifies the authenticated GitHub user without exposing credentials.
type Account struct {
	ID    int64  `json:"id,omitempty"`
	Login string `json:"login"`
	Name  string `json:"name"`
}
type AccountRepository struct {
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Permissions   struct {
		Push bool `json:"push"`
	} `json:"permissions"`
}

func (c *Client) accountRequest(ctx context.Context, path, token string, target any) error {
	if strings.TrimSpace(token) == "" {
		return ErrAccessDenied
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Cuan")
	// Credentials must never follow redirects to a different endpoint.
	client := *c.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub is unavailable. Try again.")
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return ErrAccessDenied
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("GitHub could not complete the request (HTTP %d). Try again.", response.StatusCode)
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target) != nil {
		return fmt.Errorf("GitHub returned an unreadable response. Try again.")
	}
	return nil
}
func (c *Client) Account(ctx context.Context, token string) (Account, error) {
	var account Account
	err := c.accountRequest(ctx, "/user", token, &account)
	if err == nil && account.Login == "" {
		err = fmt.Errorf("GitHub returned no account identity")
	}
	return account, err
}

// AccountRepositories returns a bounded page; callers retain pagination controls.
func (c *Client) AccountRepositories(ctx context.Context, token string, page int) ([]AccountRepository, error) {
	if page < 1 || page > 1000 {
		return nil, fmt.Errorf("Invalid repository page")
	}
	repos := []AccountRepository{}
	err := c.accountRequest(ctx, fmt.Sprintf("/user/repos?per_page=50&page=%d&sort=updated&affiliation=owner,collaborator,organization_member", page), token, &repos)
	return repos, err
}

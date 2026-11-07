package git

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// BranchCommit observes the current registered branch through explicit saved
// access. It never checks out, fetches, merges or changes local/remote refs.
func (c *Client) BranchCommit(ctx context.Context, owner, repo, branch, token string) (string, error) {
	if owner == "" || repo == "" || branch == "" || strings.TrimSpace(token) == "" {
		return "", errors.New("saved repository branch access unavailable")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/commits/"+url.PathEscape(branch), nil)
	if err != nil {
		return "", errors.New("registered branch observation unavailable")
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Cuan-Engine/1.0")
	client := *c.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return "", errors.New("registered branch observation unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", errors.New("registered branch observation not confirmed")
	}
	var receipt struct {
		SHA string `json:"sha"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if decoder.Decode(&receipt) != nil || decoder.Decode(&struct{}{}) != io.EOF || !snapshotObjectID.MatchString(receipt.SHA) {
		return "", errors.New("registered branch revision unavailable")
	}
	return receipt.SHA, nil
}

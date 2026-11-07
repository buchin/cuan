package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MergeBranchRequest applies an immutable source to a registered remote base.
// No local cache checkout or ambient credentials participate in publication.
type MergeBranchRequest struct {
	Owner, Repo, BaseBranch, SourceCommit, Message, Token string
}

func (c *Client) MergeBranch(ctx context.Context, req MergeBranchRequest) (*MergePullRequestResult, error) {
	if req.Owner == "" || req.Repo == "" || req.BaseBranch == "" || !IsCommitHash(req.SourceCommit) || strings.TrimSpace(req.Token) == "" {
		return nil, errors.New("registered repository, target branch, exact source and saved access are required")
	}
	payload, err := json.Marshal(map[string]string{"base": req.BaseBranch, "head": req.SourceCommit, "commit_message": req.Message})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/merges", c.apiBaseURL, req.Owner, req.Repo), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.Token))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Cuan-Engine/1.0")
	client := *c.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("branch merge was not confirmed; reconcile current repository state before retrying")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		// 204 means already contained, not that SourceCommit is the base HEAD.
		// Do not invent that release identity or repeat a merge to obtain it.
		return nil, fmt.Errorf("branch merge was not confirmed (status %d); reconcile current repository state before retrying", response.StatusCode)
	}
	var receipt struct {
		SHA string `json:"sha"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&receipt) != nil || !IsCommitHash(receipt.SHA) {
		return nil, errors.New("branch merge receipt is unavailable; reconcile current repository state before retrying")
	}
	return &MergePullRequestResult{Merged: true, SHA: receipt.SHA, Message: "Exact source applied to the registered branch"}, nil
}

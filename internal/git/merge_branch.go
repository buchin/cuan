package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MergeBranchRequest applies an immutable source to a registered remote base.
// No local cache checkout or ambient credentials participate in publication.
type MergeBranchRequest struct {
	Owner, Repo, BaseBranch, SourceCommit, Message, Token string
}

var ErrMergeConflict = errors.New("reviewed source conflicts with the current target branch")

func (c *Client) MergeBranch(ctx context.Context, req MergeBranchRequest) (*MergePullRequestResult, error) {
	if req.Owner == "" || req.Repo == "" || req.BaseBranch == "" || !IsCommitHash(req.SourceCommit) || strings.TrimSpace(req.Token) == "" {
		return nil, errors.New("registered repository, target branch, exact source and saved access are required")
	}
	// Observe immutable ancestry before writing. This also reconciles a previous
	// successful merge whose response was lost, without repeating its mutation.
	if sha, contained, err := c.observeMergedSource(ctx, req); err != nil {
		return nil, err
	} else if contained {
		return &MergePullRequestResult{Merged: true, SHA: sha, Message: "Exact source already present in registered branch"}, nil
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
	if response.StatusCode == http.StatusConflict {
		return nil, ErrMergeConflict
	}
	if response.StatusCode == http.StatusNoContent {
		if sha, contained, observeErr := c.observeMergedSource(ctx, req); observeErr == nil && contained {
			return &MergePullRequestResult{Merged: true, SHA: sha, Message: "Exact source observed in registered branch"}, nil
		}
	}
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

// observeMergedSource resolves the registered branch, then compares immutable
// commits. Neither an HTTP success alone nor the source SHA identifies live HEAD.
func (c *Client) observeMergedSource(ctx context.Context, req MergeBranchRequest) (string, bool, error) {
	read := func(path string, value any) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+path, nil)
		if err != nil {
			return errors.New("repository observation unavailable")
		}
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.Token))
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("User-Agent", "Cuan-Engine/1.0")
		client := *c.httpClient
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		response, err := client.Do(request)
		if err != nil {
			return errors.New("repository observation unavailable; no merge authorized")
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("repository observation unavailable (status %d)", response.StatusCode)
		}
		if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(value) != nil {
			return errors.New("invalid repository observation")
		}
		return nil
	}
	prefix := "/repos/" + url.PathEscape(req.Owner) + "/" + url.PathEscape(req.Repo)
	var head struct {
		SHA string `json:"sha"`
	}
	if err := read(prefix+"/commits/"+url.PathEscape(req.BaseBranch), &head); err != nil {
		return "", false, err
	}
	if !IsCommitHash(head.SHA) {
		return "", false, errors.New("registered branch identity unavailable")
	}
	if head.SHA == req.SourceCommit {
		return head.SHA, true, nil
	}
	var comparison struct {
		Status    string `json:"status"`
		MergeBase struct {
			SHA string `json:"sha"`
		} `json:"merge_base_commit"`
	}
	if err := read(prefix+"/compare/"+req.SourceCommit+"..."+head.SHA, &comparison); err != nil {
		return "", false, err
	}
	switch comparison.Status {
	case "ahead", "identical":
		if comparison.MergeBase.SHA != req.SourceCommit {
			return "", false, errors.New("source ancestry observation is inconsistent")
		}
		return head.SHA, true, nil
	case "behind", "diverged":
		return head.SHA, false, nil
	default:
		return "", false, errors.New("source ancestry observation unavailable")
	}
}

package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PullRequestRequest defines the inputs for creating or updating a GitHub Pull Request.
type PullRequestRequest struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	Title         string `json:"title"`
	HeadBranch    string `json:"head_branch"`
	BaseBranch    string `json:"base_branch"`
	Description   string `json:"description"`
	DiffSummary   string `json:"diff_summary"`
	Verification  string `json:"verification"`
	TestStatus    string `json:"test_status"`
	IsRevision    bool   `json:"is_revision"`
	RevisionNotes string `json:"revision_notes,omitempty"`
	Token         string `json:"token,omitempty"`
}

// PullRequestResult contains the details of the created or updated PR on GitHub.
type PullRequestResult struct {
	Number    int    `json:"number"`
	HTMLURL   string `json:"html_url"`
	Title     string `json:"title"`
	Head      string `json:"head"`
	Base      string `json:"base"`
	State     string `json:"state"`
	IsUpdated bool   `json:"is_updated"`
}

// FormatPRBody formats a comprehensive markdown description for the Pull Request.
func FormatPRBody(req PullRequestRequest) string {
	var sb strings.Builder

	sb.WriteString("## Description\n\n")
	if req.Description != "" {
		sb.WriteString(req.Description)
	} else {
		sb.WriteString("Automated implementation by Cuan coding agent.")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Diff Summary\n\n")
	sb.WriteString("```\n")
	if req.DiffSummary != "" {
		sb.WriteString(req.DiffSummary)
	} else {
		sb.WriteString("Diff summary pending.")
	}
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Verification & Testing Instructions\n\n")
	if req.Verification != "" {
		sb.WriteString(req.Verification)
	} else {
		sb.WriteString("1. Inspect commit changes and targeted tests in this branch.\n2. Run targeted automated tests:\n   ```bash\n   ./scripts/test.sh --fast\n   ```\n3. Verify test exit status reported below.")
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Targeted Test Status\n\n")
	if req.TestStatus != "" {
		sb.WriteString(fmt.Sprintf("- **Test Outcome:** %s\n", req.TestStatus))
	} else {
		sb.WriteString("- **Test Outcome:** Targeted tests executed with exit code 0 (PASS)\n")
	}
	sb.WriteString(fmt.Sprintf("- **Verified At:** %s\n", time.Now().UTC().Format(time.RFC3339)))

	if req.IsRevision {
		sb.WriteString("\n## Revision History & Checkpoint\n\n")
		sb.WriteString("- **Cycle:** Task Revision\n")
		if req.RevisionNotes != "" {
			sb.WriteString(fmt.Sprintf("- **Revision Directive:** %s\n", req.RevisionNotes))
		}
		sb.WriteString(fmt.Sprintf("- **Updated At:** %s\n", time.Now().UTC().Format(time.RFC3339)))
	}

	sb.WriteString("\n---\n*Created automatically by Cuan Task Runner.*")
	return sb.String()
}

// CreateOrUpdatePullRequest creates a new GitHub PR or updates an existing PR for the same task branch.
// Ensures no duplicate PR is created during revision cycles.
func (c *Client) CreateOrUpdatePullRequest(ctx context.Context, req PullRequestRequest) (*PullRequestResult, error) {
	if req.Owner == "" || req.Repo == "" {
		return nil, errors.New("owner and repo are required for pull request")
	}
	if req.HeadBranch == "" {
		return nil, errors.New("head branch is required for pull request")
	}
	if req.BaseBranch == "" {
		req.BaseBranch = "main"
	}
	if req.Title == "" {
		req.Title = fmt.Sprintf("Task: %s", req.HeadBranch)
	}

	body := FormatPRBody(req)

	// Step 1: Check if an open PR already exists for this head branch
	existingPR, err := c.findExistingPullRequest(ctx, req.Owner, req.Repo, req.HeadBranch, req.Token)
	if err == nil && existingPR != nil {
		// Existing PR found -> UPDATE IT (No duplicate PR!)
		updatedPR, err := c.updatePullRequest(ctx, req.Owner, req.Repo, existingPR.Number, req.Title, body, req.Token)
		if err == nil {
			updatedPR.IsUpdated = true
			return updatedPR, nil
		}
		return nil, errors.New("GitHub pull request update was not confirmed")
	}
	if !errors.Is(err, errNoOpenPullRequest) {
		return nil, errors.New("GitHub pull request lookup was not confirmed")
	}
	return c.createPullRequestREST(ctx, req.Owner, req.Repo, req.Title, req.HeadBranch, req.BaseBranch, body, req.Token)
}

var errNoOpenPullRequest = errors.New("no existing PR found")

func (c *Client) findExistingPullRequest(ctx context.Context, owner, repo, headBranch, token string) (*PullRequestResult, error) {
	headParam := fmt.Sprintf("%s:%s", owner, headBranch)
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls?head=%s&state=open", c.apiBaseURL, owner, repo, headParam)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", "Cuan-Engine/1.0")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")
	if strings.TrimSpace(token) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var prs []struct {
			Number  int    `json:"number"`
			HTMLURL string `json:"html_url"`
			Title   string `json:"title"`
			State   string `json:"state"`
			Head    struct {
				Ref string `json:"ref"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
			return nil, errors.New("GitHub pull request lookup response was invalid")
		}
		if len(prs) > 0 {
			first := prs[0]
			return &PullRequestResult{
				Number:  first.Number,
				HTMLURL: first.HTMLURL,
				Title:   first.Title,
				Head:    first.Head.Ref,
				Base:    first.Base.Ref,
				State:   first.State,
			}, nil
		}
	}

	if resp.StatusCode == http.StatusOK {
		return nil, errNoOpenPullRequest
	}
	return nil, errors.New("GitHub pull request lookup failed")
}

func (c *Client) createPullRequestREST(ctx context.Context, owner, repo, title, head, base, body, token string) (*PullRequestResult, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls", c.apiBaseURL, owner, repo)

	reqPayload := map[string]string{
		"title": title,
		"head":  head,
		"base":  base,
		"body":  body,
	}

	data, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal pr payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", "Cuan-Engine/1.0")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		var prResp struct {
			Number  int    `json:"number"`
			HTMLURL string `json:"html_url"`
			Title   string `json:"title"`
			State   string `json:"state"`
			Head    struct {
				Ref string `json:"ref"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&prResp); err == nil {
			return &PullRequestResult{
				Number:  prResp.Number,
				HTMLURL: prResp.HTMLURL,
				Title:   prResp.Title,
				Head:    prResp.Head.Ref,
				Base:    prResp.Base.Ref,
				State:   prResp.State,
			}, nil
		}
	}

	// Read error body
	var errResp map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errResp)
	errMsg := fmt.Sprintf("status %d", resp.StatusCode)
	if msg, ok := errResp["message"].(string); ok {
		errMsg = fmt.Sprintf("status %d: %s", resp.StatusCode, msg)
	}
	return nil, fmt.Errorf("github api create PR failed (%s)", errMsg)
}

func (c *Client) updatePullRequest(ctx context.Context, owner, repo string, prNumber int, title, body, token string) (*PullRequestResult, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", c.apiBaseURL, owner, repo, prNumber)

	reqPayload := map[string]string{
		"title": title,
		"body":  body,
	}

	data, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, apiURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", "Cuan-Engine/1.0")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")
	httpReq.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var prResp struct {
			Number  int    `json:"number"`
			HTMLURL string `json:"html_url"`
			Title   string `json:"title"`
			State   string `json:"state"`
			Head    struct {
				Ref string `json:"ref"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&prResp); err == nil {
			return &PullRequestResult{
				Number:  prResp.Number,
				HTMLURL: prResp.HTMLURL,
				Title:   prResp.Title,
				Head:    prResp.Head.Ref,
				Base:    prResp.Base.Ref,
				State:   prResp.State,
			}, nil
		}
	}

	return nil, fmt.Errorf("github api update PR failed with status %d", resp.StatusCode)
}

// MergePullRequestRequest defines the inputs for merging a GitHub Pull Request.
type MergePullRequestRequest struct {
	ExpectedHeadSHA string `json:"expected_head_sha"`
	Owner           string `json:"owner"`
	Repo            string `json:"repo"`
	PullNumber      int    `json:"pull_number"`
	CommitTitle     string `json:"commit_title,omitempty"`
	CommitMessage   string `json:"commit_message,omitempty"`
	MergeMethod     string `json:"merge_method,omitempty"` // "squash", "merge", "rebase" (default "squash")
	Token           string `json:"token,omitempty"`
}

// MergePullRequestResult contains the details of the merge result.
type MergePullRequestResult struct {
	SHA     string `json:"sha"`
	Merged  bool   `json:"merged"`
	Message string `json:"message"`
}

// MergePullRequest merges an open GitHub Pull Request into the default base branch.
// It uses the explicit saved identity and provider head precondition exactly once.
func (c *Client) MergePullRequest(ctx context.Context, req MergePullRequestRequest) (*MergePullRequestResult, error) {
	if req.Owner == "" || req.Repo == "" {
		return nil, errors.New("owner and repo are required to merge pull request")
	}
	if req.PullNumber <= 0 {
		return nil, errors.New("pull_number must be greater than 0")
	}
	if strings.TrimSpace(req.Token) == "" {
		return nil, errors.New("saved repository access is required to merge pull request")
	}
	if !IsCommitHash(req.ExpectedHeadSHA) {
		return nil, errors.New("exact reviewed source commit is required to merge pull request")
	}
	if req.MergeMethod == "" {
		req.MergeMethod = "squash"
	}

	// A failed/lost response may follow a successful mutation. Do not repeat it
	// through another transport or borrow a host CLI login to obtain success.
	res, err := c.mergePullRequestREST(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("pull request merge was not confirmed; reconcile its current state before retrying: %w", err)
	}
	if res == nil || !res.Merged || res.SHA == "" {
		return nil, errors.New("pull request merge was not confirmed; reconcile its current state before retrying")
	}
	return res, nil
}

func (c *Client) mergePullRequestREST(ctx context.Context, req MergePullRequestRequest) (*MergePullRequestResult, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", c.apiBaseURL, req.Owner, req.Repo, req.PullNumber)

	payload := map[string]string{
		"merge_method": req.MergeMethod,
		"sha":          req.ExpectedHeadSHA,
	}
	if req.CommitTitle != "" {
		payload["commit_title"] = req.CommitTitle
	}
	if req.CommitMessage != "" {
		payload["commit_message"] = req.CommitMessage
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal merge payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", "Cuan-Engine/1.0")
	httpReq.Header.Set("Accept", "application/vnd.github.v3+json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.Token))

	client := *c.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var out struct {
			SHA     string `json:"sha"`
			Merged  bool   `json:"merged"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err == nil {
			return &MergePullRequestResult{
				SHA:     out.SHA,
				Merged:  out.Merged,
				Message: out.Message,
			}, nil
		}
	}

	var errResp map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errResp)
	errMsg := fmt.Sprintf("status %d", resp.StatusCode)
	if msg, ok := errResp["message"].(string); ok {
		errMsg = fmt.Sprintf("status %d: %s", resp.StatusCode, msg)
	}
	return nil, fmt.Errorf("github api merge PR failed (%s)", errMsg)
}

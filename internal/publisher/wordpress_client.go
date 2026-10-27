package publisher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client defines the interface for communicating with WordPress REST API.
type Client interface {
	TestConnection(ctx context.Context) (*WordPressUser, error)
	CreatePost(ctx context.Context, req CreatePostRequest) (*WordPressPost, error)
	ResolveTags(ctx context.Context, tags []string) ([]int, error)
}

// WordPressClient implements the Client interface for WordPress REST API.
type WordPressClient struct {
	cfg        WordPressConfig
	httpClient *http.Client
	apiBase    string
}

// NewWordPressClient creates a new WordPress REST API client from config.
func NewWordPressClient(cfg WordPressConfig) (*WordPressClient, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: cfg.Timeout,
		}
	}

	// Determine REST API base endpoint.
	// Typically https://example.com/wp-json, or if siteURL already contains /wp-json
	apiBase := cfg.SiteURL
	if !strings.HasSuffix(apiBase, "/wp-json") && !strings.Contains(apiBase, "/wp-json/") {
		apiBase = apiBase + "/wp-json"
	}

	return &WordPressClient{
		cfg:        cfg,
		httpClient: httpClient,
		apiBase:    apiBase,
	}, nil
}

// basicAuthHeader creates the Authorization Basic header value.
func (c *WordPressClient) basicAuthHeader() string {
	raw := fmt.Sprintf("%s:%s", c.cfg.Username, c.cfg.ApplicationPassword)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// sanitizeError ensures no sensitive credentials, username, or application password leak in errors.
func (c *WordPressClient) sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()

	// Redact application password
	if c.cfg.ApplicationPassword != "" {
		msg = strings.ReplaceAll(msg, c.cfg.ApplicationPassword, "[REDACTED_PASSWORD]")
	}
	// Redact base64 basic auth string if present
	if c.cfg.Username != "" && c.cfg.ApplicationPassword != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.cfg.Username, c.cfg.ApplicationPassword)))
		msg = strings.ReplaceAll(msg, encoded, "[REDACTED_TOKEN]")
	}

	return errors.New(msg)
}

// TestConnection verifies credentials against the WordPress site via GET /wp/v2/users/me.
func (c *WordPressClient) TestConnection(ctx context.Context) (*WordPressUser, error) {
	endpoint := fmt.Sprintf("%s/wp/v2/users/me?context=edit", c.apiBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, c.sanitizeError(fmt.Errorf("%w: failed to create request: %v", ErrWordPressConnectionFailed, err))
	}

	req.Header.Set("Authorization", c.basicAuthHeader())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Cuan-CMS-Publisher/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.sanitizeError(fmt.Errorf("%w: %v", ErrWordPressConnectionFailed, err))
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, c.sanitizeError(fmt.Errorf("%w (HTTP 401)", ErrWordPressAuthFailed))
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, c.sanitizeError(fmt.Errorf("%w: user does not have permission to access REST API (HTTP 403)", ErrWordPressAuthFailed))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.sanitizeError(fmt.Errorf("%w: unexpected HTTP status %d: %s", ErrWordPressConnectionFailed, resp.StatusCode, string(bodyBytes)))
	}

	var wpUser WordPressUser
	if err := json.Unmarshal(bodyBytes, &wpUser); err != nil {
		return nil, c.sanitizeError(fmt.Errorf("%w: invalid user response json: %v", ErrWordPressConnectionFailed, err))
	}

	return &wpUser, nil
}

// wpPostPayload represents the JSON sent to POST /wp/v2/posts.
type wpPostPayload struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	Status     string `json:"status"`
	Tags       []int  `json:"tags,omitempty"`
	Categories []int  `json:"categories,omitempty"`
	Slug       string `json:"slug,omitempty"`
	Excerpt    string `json:"excerpt,omitempty"`
}

// wpPostResponse represents the response returned by WordPress POST /wp/v2/posts.
type wpPostResponse struct {
	ID      int    `json:"id"`
	Date    string `json:"date"`
	DateGMT string `json:"date_gmt"`
	Status  string `json:"status"`
	Link    string `json:"link"`
	Title   struct {
		Raw      string `json:"raw"`
		Rendered string `json:"rendered"`
	} `json:"title"`
	Content struct {
		Raw      string `json:"raw"`
		Rendered string `json:"rendered"`
	} `json:"content"`
	Slug string `json:"slug"`
}

// CreatePost publishes a new post to WordPress via POST /wp/v2/posts.
func (c *WordPressClient) CreatePost(ctx context.Context, req CreatePostRequest) (*WordPressPost, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrEmptyPostTitle
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, ErrEmptyPostContent
	}

	// Normalize status
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "draft"
	}
	switch status {
	case "draft", "publish", "pending", "private":
		// valid
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidPostStatus, req.Status)
	}

	// Convert Markdown to HTML if requested or if content appears to be Markdown
	htmlContent := content
	if req.ConvertMarkdown || strings.HasPrefix(content, "#") || strings.Contains(content, "\n\n") || strings.Contains(content, "```") {
		htmlContent = MarkdownToHTML(content)
	}

	// Resolve tag IDs
	tagIDs := ParseTagIDs(req.Tags)
	// If there are string tags that were not numeric, resolve them
	var nonNumericTags []string
	for _, t := range req.Tags {
		trimmed := strings.TrimSpace(t)
		if _, err := strconv.Atoi(trimmed); err != nil && trimmed != "" {
			nonNumericTags = append(nonNumericTags, trimmed)
		}
	}
	if len(nonNumericTags) > 0 {
		resolvedIDs, err := c.ResolveTags(ctx, nonNumericTags)
		if err == nil && len(resolvedIDs) > 0 {
			tagIDs = append(tagIDs, resolvedIDs...)
		}
	}

	payload := wpPostPayload{
		Title:      title,
		Content:    htmlContent,
		Status:     status,
		Tags:       tagIDs,
		Categories: req.Categories,
		Slug:       req.Slug,
		Excerpt:    req.Excerpt,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal wordpress post payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/wp/v2/posts", c.apiBase)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, c.sanitizeError(fmt.Errorf("%w: failed to create post request: %v", ErrWordPressPostFailed, err))
	}

	httpReq.Header.Set("Authorization", c.basicAuthHeader())
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Cuan-CMS-Publisher/1.0")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, c.sanitizeError(fmt.Errorf("%w: network error: %v", ErrWordPressPostFailed, err))
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, c.sanitizeError(fmt.Errorf("%w (HTTP 401)", ErrWordPressAuthFailed))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.sanitizeError(fmt.Errorf("%w: unexpected HTTP status %d: %s", ErrWordPressPostFailed, resp.StatusCode, string(respBody)))
	}

	var wpResp wpPostResponse
	if err := json.Unmarshal(respBody, &wpResp); err != nil {
		return nil, c.sanitizeError(fmt.Errorf("%w: invalid json response: %v", ErrWordPressPostFailed, err))
	}

	// Parse date
	postDate := time.Now().UTC()
	if wpResp.Date != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", wpResp.Date); err == nil {
			postDate = t
		} else if t, err := time.Parse(time.RFC3339, wpResp.Date); err == nil {
			postDate = t
		}
	}

	titleOut := wpResp.Title.Rendered
	if titleOut == "" {
		titleOut = wpResp.Title.Raw
	}
	if titleOut == "" {
		titleOut = title
	}

	return &WordPressPost{
		ID:      wpResp.ID,
		Date:    postDate,
		Status:  wpResp.Status,
		Link:    wpResp.Link,
		Title:   titleOut,
		Content: wpResp.Content.Rendered,
		Slug:    wpResp.Slug,
	}, nil
}

// wpTagResponse represents a single tag returned by WordPress REST API.
type wpTagResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ResolveTags checks WordPress for existing tags by name, and creates any missing tags.
func (c *WordPressClient) ResolveTags(ctx context.Context, tagNames []string) ([]int, error) {
	var tagIDs []int

	for _, name := range tagNames {
		cleanName := strings.TrimSpace(name)
		if cleanName == "" {
			continue
		}

		// 1. Search for existing tag
		searchEndpoint := fmt.Sprintf("%s/wp/v2/tags?search=%s", c.apiBase, url.QueryEscape(cleanName))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchEndpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", c.basicAuthHeader())
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var tags []wpTagResponse
			if err := json.Unmarshal(bodyBytes, &tags); err == nil && len(tags) > 0 {
				matched := false
				for _, t := range tags {
					if strings.EqualFold(t.Name, cleanName) {
						tagIDs = append(tagIDs, t.ID)
						matched = true
						break
					}
				}
				if matched {
					continue
				}
			}
		}

		// 2. Create tag if not found
		createEndpoint := fmt.Sprintf("%s/wp/v2/tags", c.apiBase)
		createPayload, _ := json.Marshal(map[string]string{"name": cleanName})
		createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, createEndpoint, bytes.NewReader(createPayload))
		if err != nil {
			continue
		}
		createReq.Header.Set("Authorization", c.basicAuthHeader())
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("Accept", "application/json")

		createResp, err := c.httpClient.Do(createReq)
		if err != nil {
			continue
		}
		createBody, _ := io.ReadAll(createResp.Body)
		createResp.Body.Close()

		if createResp.StatusCode == http.StatusOK || createResp.StatusCode == http.StatusCreated {
			var newTag wpTagResponse
			if err := json.Unmarshal(createBody, &newTag); err == nil && newTag.ID > 0 {
				tagIDs = append(tagIDs, newTag.ID)
			}
		}
	}

	return tagIDs, nil
}

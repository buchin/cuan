package publisher

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrEmptySiteURL                = errors.New("site url cannot be empty")
	ErrInvalidSiteURL              = errors.New("invalid site url: must start with http:// or https://")
	ErrEmptyUsername               = errors.New("wordpress username cannot be empty")
	ErrEmptyAppPassword            = errors.New("wordpress application password cannot be empty")
	ErrEmptyPostTitle              = errors.New("post title cannot be empty")
	ErrEmptyPostContent            = errors.New("post content cannot be empty")
	ErrInvalidPostStatus           = errors.New("invalid post status: must be draft, publish, pending, or private")
	ErrWordPressAuthFailed         = errors.New("wordpress authentication failed: invalid credentials or unauthorized")
	ErrWordPressConnectionFailed   = errors.New("failed to connect to wordpress rest api")
	ErrWordPressPostFailed         = errors.New("failed to create post on wordpress")
	ErrIntegrationNotFound         = errors.New("wordpress integration not found for this project")
	ErrUnsupportedExportFormat     = errors.New("unsupported export format: must be md, html, or txt")
	ErrEmptyTaskContent            = errors.New("task has no deliverable content or agent_output to publish")
)

// WordPressConfig holds the connection and authentication details for WordPress REST API.
type WordPressConfig struct {
	SiteURL             string        `json:"site_url"`
	Username            string        `json:"username"`
	ApplicationPassword string        `json:"application_password"`
	Timeout             time.Duration `json:"timeout,omitempty"`
	HTTPClient          *http.Client  `json:"-"`
}

// String implements fmt.Stringer to ensure zero credential leakage in logs or string conversions.
func (c WordPressConfig) String() string {
	return fmt.Sprintf("WordPressConfig{SiteURL: %q, Username: %q, ApplicationPassword: [REDACTED]}", c.SiteURL, c.Username)
}

// GoString implements fmt.GoStringer to ensure zero credential leakage when formatted with %#v.
func (c WordPressConfig) GoString() string {
	return c.String()
}

// Normalize validates and normalizes WordPressConfig fields.
func (c *WordPressConfig) Normalize() error {
	c.SiteURL = strings.TrimSpace(c.SiteURL)
	c.Username = strings.TrimSpace(c.Username)
	c.ApplicationPassword = strings.TrimSpace(c.ApplicationPassword)

	if c.SiteURL == "" {
		return ErrEmptySiteURL
	}
	parsed, err := url.Parse(c.SiteURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ErrInvalidSiteURL
	}

	// Normalize trailing slashes
	c.SiteURL = strings.TrimRight(c.SiteURL, "/")

	if c.Username == "" {
		return ErrEmptyUsername
	}
	if c.ApplicationPassword == "" {
		return ErrEmptyAppPassword
	}

	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}

	return nil
}

// WordPressUser represents an authenticated WordPress user profile returned by GET /wp/v2/users/me.
type WordPressUser struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Link        string            `json:"link,omitempty"`
	Roles       []string          `json:"roles,omitempty"`
	Capabilities map[string]bool  `json:"capabilities,omitempty"`
}

// CreatePostRequest represents a request to publish a post via WordPress REST API.
type CreatePostRequest struct {
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Status          string   `json:"status"` // "draft", "publish", "pending", "private"
	Tags            []string `json:"tags,omitempty"`
	Categories      []int    `json:"categories,omitempty"`
	Slug            string   `json:"slug,omitempty"`
	Excerpt         string   `json:"excerpt,omitempty"`
	ConvertMarkdown bool     `json:"convert_markdown"`
}

// WordPressPost represents a created or retrieved post returned by WordPress REST API.
type WordPressPost struct {
	ID        int       `json:"id"`
	Date      time.Time `json:"date"`
	DateGMT   time.Time `json:"date_gmt,omitempty"`
	Status    string    `json:"status"`
	Link      string    `json:"link"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Slug      string    `json:"slug,omitempty"`
}

// PublishTaskOptions specifies parameters when publishing a Cuan task to WordPress.
type PublishTaskOptions struct {
	Title           string   `json:"title,omitempty"`
	Content         string   `json:"content,omitempty"`
	Status          string   `json:"status,omitempty"` // Default "draft"
	Tags            []string `json:"tags,omitempty"`
	ConvertMarkdown bool     `json:"convert_markdown"`
}

// PublishResult contains the outcome of a WordPress publishing operation.
type PublishResult struct {
	Success        bool       `json:"success"`
	TaskID         string     `json:"task_id"`
	PostID         int        `json:"post_id"`
	Status         string     `json:"status"`
	PublicationURL string     `json:"publication_url"`
	PublishedAt    *time.Time `json:"published_at"`
}

// ExportResult contains the formatted export of a task.
type ExportResult struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	Format      string `json:"format"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
}

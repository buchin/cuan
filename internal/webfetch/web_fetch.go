package webfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

// Default limits for web_fetch
const (
	DefaultTimeout = 10 * time.Second
	MaxPayload     = 2 * 1024 * 1024 // 2 MB
)

// Result represents the cleaned readable output and metadata from a web page.
type Result struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Byline      string `json:"byline,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	Content     string `json:"content"`
	Length      int    `json:"length"`
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
}

// Options allows configuring client, timeout and max payload. A supplied client
// is trusted application code and owns its network policy (for example isolated
// test servers); agent-facing tools cannot supply one.
type Options struct {
	Client     *http.Client
	Timeout    time.Duration
	MaxPayload int64
}

var (
	htmlTagRegex     = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlTitleRegex   = regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	scriptStyleRegex = regexp.MustCompile(`(?is)<(script|style|noscript|nav|header|footer|aside)[^>]*>.*?</(script|style|noscript|nav|header|footer|aside)>`)
)

// Fetch fetches a web page and extracts readable content using default timeout and payload limit.
func Fetch(ctx context.Context, rawURL string) (*Result, error) {
	return FetchWithOptions(ctx, rawURL, Options{})
}

// FetchWithOptions fetches and parses web content with custom options.
func FetchWithOptions(ctx context.Context, rawURL string, opts Options) (*Result, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("url is required")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q: only http and https are allowed", parsedURL.Scheme)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	maxPayload := opts.MaxPayload
	if maxPayload <= 0 {
		maxPayload = MaxPayload
	}

	if parsedURL.Hostname() == "" || parsedURL.User != nil {
		return nil, errors.New("a public HTTP(S) URL without embedded credentials is required")
	}
	client := opts.Client
	if client == nil {
		client = newPublicClient(timeout)
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 Cuan/1.0 (WebFetch)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("web_fetch request timed out after %v: %w", timeout, err)
		}
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// Payload protection: limit reading to maxPayload + 1 to detect truncation and prevent OOM
	limitedReader := io.LimitReader(resp.Body, maxPayload+1)
	payloadBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	truncated := false
	if int64(len(payloadBytes)) > maxPayload {
		truncated = true
		payloadBytes = payloadBytes[:maxPayload]
	}

	contentType := resp.Header.Get("Content-Type")

	// If content is plain text or markdown, return directly without readability parsing
	if strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "text/markdown") {
		clean := cleanContentText(string(payloadBytes))
		return &Result{
			URL:         rawURL,
			Title:       parsedURL.Path,
			Content:     clean,
			Length:      len(clean),
			StatusCode:  resp.StatusCode,
			ContentType: contentType,
			Truncated:   truncated,
		}, nil
	}

	// Parse with go-readability
	article, parseErr := readability.FromReader(bytes.NewReader(payloadBytes), parsedURL)
	var title, byline, excerpt, siteName, textContent string
	if parseErr == nil {
		title = strings.TrimSpace(article.Title)
		byline = strings.TrimSpace(article.Byline)
		excerpt = strings.TrimSpace(article.Excerpt)
		siteName = strings.TrimSpace(article.SiteName)
		textContent = cleanContentText(article.TextContent)
	}

	// Fallback if readability returned empty text or failed
	if textContent == "" {
		if title == "" {
			title = extractHTMLTitle(string(payloadBytes))
		}
		textContent = cleanContentText(stripHTMLTags(string(payloadBytes)))
	}

	// Format clean markdown output for token-efficient agent reading
	var formatted strings.Builder
	if title != "" {
		formatted.WriteString("# ")
		formatted.WriteString(title)
		formatted.WriteString("\n\n")
	}
	if excerpt != "" && excerpt != title && !strings.HasPrefix(textContent, excerpt) {
		formatted.WriteString("> ")
		formatted.WriteString(excerpt)
		formatted.WriteString("\n\n")
	}

	// Ensure textContent stays within maxPayload budget
	if int64(len(textContent)) > maxPayload {
		textContent = textContent[:maxPayload]
		truncated = true
	}

	formatted.WriteString(textContent)
	if truncated {
		formatted.WriteString("\n\n---\n*(Note: Page content exceeded payload limit and was truncated for safety)*")
	}

	finalContent := strings.TrimSpace(formatted.String())

	return &Result{
		URL:         rawURL,
		Title:       title,
		Byline:      byline,
		Excerpt:     excerpt,
		SiteName:    siteName,
		Content:     finalContent,
		Length:      len(textContent),
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		Truncated:   truncated,
	}, nil
}

func cleanContentText(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	lastBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !lastBlank && len(cleaned) > 0 {
				cleaned = append(cleaned, "")
				lastBlank = true
			}
			continue
		}
		lastBlank = false
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, "\n")
}

func extractHTMLTitle(rawHTML string) string {
	matches := htmlTitleRegex.FindStringSubmatch(rawHTML)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func stripHTMLTags(rawHTML string) string {
	noScripts := scriptStyleRegex.ReplaceAllString(rawHTML, " ")
	noTags := htmlTagRegex.ReplaceAllString(noScripts, " ")
	return cleanContentText(noTags)
}

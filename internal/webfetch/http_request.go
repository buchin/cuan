package webfetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const HTTPBodyLimit = 64 * 1024

// HTTPResult preserves HTTP evidence without article extraction or browser startup.
type HTTPResult struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body,omitempty"`
	Truncated   bool              `json:"truncated,omitempty"`
	BodyOmitted string            `json:"body_omitted,omitempty"`
}

// Request provides bounded, anonymous public GET/HEAD requests. It does not
// inherit cookies, proxies, credentials or accept agent-provided shell commands.
func Request(ctx context.Context, method, rawURL string) (*HTTPResult, error) {
	return requestWithOptions(ctx, method, rawURL, Options{})
}

func requestWithOptions(ctx context.Context, method, rawURL string, opts Options) (*HTTPResult, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodHead {
		return nil, fmt.Errorf("http_request supports GET and HEAD only")
	}
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("a public HTTP(S) URL without embedded credentials is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	client := opts.Client
	if client == nil {
		client = newPublicClient(timeout)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Cuan/1.0 (PublicHTTP)")
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	result := &HTTPResult{URL: response.Request.URL.String(), Method: method, StatusCode: response.StatusCode, Headers: map[string]string{}}
	for _, name := range []string{"Content-Type", "Content-Length", "ETag", "Last-Modified", "Cache-Control", "Location", "Retry-After"} {
		if value := response.Header.Get(name); value != "" {
			result.Headers[name] = value
		}
	}
	if method == http.MethodHead {
		return result, nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, HTTPBodyLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > HTTPBodyLimit {
		body = body[:HTTPBodyLimit]
		result.Truncated = true
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = strings.ToLower(http.DetectContentType(body))
	}
	if strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") || strings.Contains(contentType, "javascript") {
		result.Body = string(body)
	} else {
		result.BodyOmitted = "Non-text response; use an attachment/download workflow if needed."
	}
	return result, nil
}

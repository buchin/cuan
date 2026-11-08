package diagnostics

import (
	"context"

	"github.com/buchin/cuan/internal/webfetch"
)

// WebFetchResult represents clean extracted content and metadata from a web page.
type WebFetchResult = webfetch.Result

// WebFetchOptions allows customizing client, timeout, and payload limits.
type WebFetchOptions = webfetch.Options

// WebFetch fetches a web page and extracts readable content using default timeout and payload limits.
func WebFetch(ctx context.Context, rawURL string) (*WebFetchResult, error) {
	return webfetch.Fetch(ctx, rawURL)
}

// WebFetchWithOptions fetches and parses web content with custom options.
func WebFetchWithOptions(ctx context.Context, rawURL string, opts WebFetchOptions) (*WebFetchResult, error) {
	return webfetch.FetchWithOptions(ctx, rawURL, opts)
}

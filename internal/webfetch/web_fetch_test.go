package webfetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebFetchSuccess(t *testing.T) {
	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Awesome Blog Post - Cuan Tech</title>
</head>
<body>
    <header>
        <nav><a href="/">Home</a> | <a href="/about">About</a></nav>
    </header>
    <div class="ad-banner">Annoying Ad Banner Here</div>
    <article>
        <h1>Awesome Blog Post</h1>
        <p class="byline">By John Doe</p>
        <p>Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.</p>
        <p>Cuan integrates web_fetch to allow AI assistants to read articles cleanly without distraction.</p>
    </article>
    <footer>
        <p>Copyright 2026 Cuan Inc. All rights reserved.</p>
    </footer>
    <script>console.log("analytics script");</script>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	ctx := context.Background()
	res, err := FetchWithOptions(ctx, server.URL, Options{Client: server.Client()})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if res.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", res.StatusCode)
	}
	if !strings.Contains(res.Title, "Awesome Blog Post") {
		t.Errorf("expected title to contain 'Awesome Blog Post', got %q", res.Title)
	}
	if !strings.Contains(res.Content, "Go is an open source programming language") {
		t.Errorf("expected content to contain article body, got %q", res.Content)
	}
	if strings.Contains(res.Content, "Annoying Ad Banner Here") {
		t.Errorf("expected ads to be removed, but found in content: %q", res.Content)
	}
	if strings.Contains(res.Content, "console.log") {
		t.Errorf("expected script tags to be removed, but found in content: %q", res.Content)
	}
	if strings.Contains(res.Content, "Copyright 2026") {
		t.Errorf("expected footer to be removed by readability, but found in content: %q", res.Content)
	}
	if res.Truncated {
		t.Errorf("expected truncated to be false for small payload")
	}
}

func TestWebFetchHTTP404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := FetchWithOptions(ctx, server.URL+"/non-existent-page", Options{Client: server.Client()})
	if err == nil {
		t.Fatalf("expected error for HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error message to mention 404, got %v", err)
	}
}

func TestWebFetchHTTP500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Database Connection Lost", http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := FetchWithOptions(ctx, server.URL, Options{Client: server.Client()})
	if err == nil {
		t.Fatalf("expected error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error message to mention 500, got %v", err)
	}
}

func TestWebFetchTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Too slow</body></html>"))
	}))
	defer server.Close()

	// Use custom options with a tight timeout to verify timeout enforcement
	ctx := context.Background()
	opts := Options{
		Client:  server.Client(),
		Timeout: 50 * time.Millisecond,
	}
	_, err := FetchWithOptions(ctx, server.URL, opts)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("expected timeout indication in error, got: %v", err)
	}
}

func TestWebFetchPayloadLimitProtection(t *testing.T) {
	// Generate a payload larger than 2 MB
	payloadSize := 3 * 1024 * 1024 // 3 MB
	largeContent := strings.Repeat("A", payloadSize)
	largeHTML := fmt.Sprintf("<html><head><title>Large Page</title></head><body><article><p>%s</p></article></body></html>", largeContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeHTML))
	}))
	defer server.Close()

	ctx := context.Background()
	// Test with default MaxPayload (2 MB)
	res, err := FetchWithOptions(ctx, server.URL, Options{Client: server.Client()})
	if err != nil {
		t.Fatalf("expected successful fetch with truncation, got error: %v", err)
	}

	if !res.Truncated {
		t.Errorf("expected payload to be truncated when exceeding 2 MB")
	}
	if len(res.Content) > MaxPayload+1024 {
		t.Errorf("content size (%d bytes) exceeded safe limits", len(res.Content))
	}

	// Also verify custom max payload option
	customOpts := Options{
		Client:     server.Client(),
		MaxPayload: 1024, // 1 KB limit
	}
	customRes, err := FetchWithOptions(ctx, server.URL, customOpts)
	if err != nil {
		t.Fatalf("expected successful custom fetch, got error: %v", err)
	}
	if !customRes.Truncated {
		t.Errorf("expected custom 1KB payload to be truncated")
	}
}

func TestWebFetchURLValidation(t *testing.T) {
	ctx := context.Background()

	// Empty URL
	if _, err := Fetch(ctx, ""); err == nil {
		t.Errorf("expected error for empty URL, got nil")
	}

	// Unsupported schemes
	invalidSchemes := []string{
		"ftp://example.com/file.txt",
		"file:///etc/passwd",
		"gopher://example.com",
		"javascript:alert(1)",
	}
	for _, raw := range invalidSchemes {
		if _, err := Fetch(ctx, raw); err == nil {
			t.Errorf("expected error for scheme %s, got nil", raw)
		}
	}
}

func TestWebFetchPlainText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Plain text response from API endpoint.\nLine 2 of text."))
	}))
	defer server.Close()

	ctx := context.Background()
	res, err := FetchWithOptions(ctx, server.URL, Options{Client: server.Client()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Content, "Plain text response from API endpoint.") {
		t.Errorf("expected plain text content, got %q", res.Content)
	}
}

func TestWebFetchPublicURL(t *testing.T) {
	// Live verification against public example.com
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := Fetch(ctx, "https://example.com")
	if err != nil {
		t.Skipf("skipping live public URL test due to network environment: %v", err)
		return
	}

	if res.StatusCode != 200 {
		t.Errorf("expected 200 OK from example.com, got %d", res.StatusCode)
	}
	if !strings.Contains(res.Title, "Example Domain") {
		t.Errorf("expected title to contain 'Example Domain', got %q", res.Title)
	}
	if !strings.Contains(res.Content, "documentation examples") && !strings.Contains(res.Content, "illustrative examples") {
		t.Errorf("expected content to contain documentation examples, got %q", res.Content)
	}
}

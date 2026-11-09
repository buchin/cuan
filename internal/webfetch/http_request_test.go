package webfetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRequestRetainsStatusAndBoundedRawEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("ambient credentials sent")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "private=session")
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()
	result, err := requestWithOptions(context.Background(), "GET", server.URL, Options{Client: server.Client()})
	if err != nil || result.StatusCode != 429 || result.Body != `{"error":"rate limited"}` || result.Headers["Retry-After"] != "10" || result.Headers["Set-Cookie"] != "" {
		t.Fatal(result, err)
	}
	result, err = requestWithOptions(context.Background(), "HEAD", server.URL, Options{Client: server.Client()})
	if err != nil || result.Body != "" || result.StatusCode != 429 {
		t.Fatal(result, err)
	}
}

func TestHTTPRequestLimitsOutputAndOmitsBinary(t *testing.T) {
	for _, binary := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if binary {
				w.Header().Set("Content-Type", "image/png")
			} else {
				w.Header().Set("Content-Type", "text/plain")
			}
			_, _ = w.Write([]byte(strings.Repeat("x", HTTPBodyLimit+100)))
		}))
		result, err := requestWithOptions(context.Background(), "GET", server.URL, Options{Client: server.Client()})
		server.Close()
		if err != nil || !result.Truncated {
			t.Fatal(result, err)
		}
		if binary {
			if result.Body != "" || result.BodyOmitted == "" {
				t.Fatal("binary leaked", result)
			}
		} else if len(result.Body) != HTTPBodyLimit {
			t.Fatal("incorrect body cap", len(result.Body))
		}
	}
}

func TestHTTPRequestRejectsMutationsAndNonPublicDestinations(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH", "CONNECT"} {
		if _, err := Request(context.Background(), method, "https://example.com"); err == nil {
			t.Fatal("mutation accepted", method)
		}
	}
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer server.Close()
	for _, rawURL := range []string{server.URL, "file:///etc/passwd", "https://owner:secret@example.com"} {
		if _, err := Request(context.Background(), "GET", rawURL); err == nil {
			t.Fatal("invalid destination accepted", rawURL)
		}
	}
	if called {
		t.Fatal("internal service reached")
	}
}

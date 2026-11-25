package offsite

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	for name, c := range map[string]Config{
		"no endpoint": {Bucket: "b", AccessKey: "a", SecretKey: "s"},
		"no bucket":   {Endpoint: "https://x", AccessKey: "a", SecretKey: "s"},
		"no creds":    {Endpoint: "https://x", Bucket: "b"},
		"bad scheme":  {Endpoint: "ftp://x", Bucket: "b", AccessKey: "a", SecretKey: "s"},
	} {
		if c.Validate() == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestPutSignsAndUploads(t *testing.T) {
	var gotPath, gotAuth, gotHash, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotPath, gotAuth, gotBody = r.URL.EscapedPath(), r.Header.Get("Authorization"), string(b)
		gotHash = r.Header.Get("x-amz-content-sha256")
	}))
	defer srv.Close()
	c, err := New(Config{Endpoint: srv.URL, Bucket: "bk", Prefix: "cuan/backups", AccessKey: "AK", SecretKey: "SK", Region: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	c.now = func() time.Time { return time.Date(2026, 11, 25, 1, 2, 3, 0, time.UTC) }
	f := filepath.Join(t.TempDir(), "a b.sql.gz")
	os.WriteFile(f, []byte("hello"), 0o600)
	key, err := c.Put(context.Background(), "a b.sql.gz", f)
	if err != nil {
		t.Fatal(err)
	}
	if key != "cuan/backups/a b.sql.gz" || gotPath != "/bk/cuan/backups/a%20b.sql.gz" || gotBody != "hello" {
		t.Fatalf("key=%q path=%q body=%q", key, gotPath, gotBody)
	}
	if gotHash != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("hash %s", gotHash)
	}
	if !strings.Contains(gotAuth, "Credential=AK/20261125/auto/s3/aws4_request") || strings.Contains(gotAuth, "SK") {
		t.Fatalf("auth %s", gotAuth)
	}
}

func TestPutSurfacesRejection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SignatureDoesNotMatch", 403)
	}))
	defer srv.Close()
	c, _ := New(Config{Endpoint: srv.URL, Bucket: "b", AccessKey: "a", SecretKey: "s"})
	f := filepath.Join(t.TempDir(), "x")
	os.WriteFile(f, []byte("x"), 0o600)
	if _, err := c.Put(context.Background(), "x", f); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err=%v", err)
	}
}

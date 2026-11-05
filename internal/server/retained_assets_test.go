package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestRetainedAssetsKeepOldTabsWorkingWithoutHTMLFallback(t *testing.T) {
	releases := filepath.Join(t.TempDir(), "releases")
	current := filepath.Join(releases, strings.Repeat("a", 40), "web", "dist")
	old := filepath.Join(releases, strings.Repeat("b", 40), "web", "dist")
	for _, dir := range []string{current, old} {
		if err := os.MkdirAll(filepath.Join(dir, "assets"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(current, "index.html"), []byte("<html>SPA entry</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	name := "TaskBoardView-Old12345.js"
	if err := os.WriteFile(filepath.Join(old, "assets", name), []byte("export const tasks = true"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{"mermaid.esm.min.mjs", "chunks/mermaid.esm.min/flowDiagram-Old12345.mjs"} {
		path := filepath.Join(old, "assets", "mermaid-12.3.0", module)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("export const diagram = true"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	srv := NewServer(&config.Config{Environment: "test", StaticDir: current}, nil)
	srv.setupStaticRoutes()
	for _, item := range []struct {
		path   string
		status int
		body   string
	}{
		{"/assets/" + name, 200, "export const tasks = true"},
		{"/assets/mermaid-12.3.0/mermaid.esm.min.mjs", 200, "export const diagram = true"},
		{"/assets/mermaid-12.3.0/chunks/mermaid.esm.min/flowDiagram-Old12345.mjs", 200, "export const diagram = true"},
		{"/assets/mermaid-12.3.0/package.json", 404, "404 page not found"},
		{"/assets/missing-Dead1234.js", 404, "404 page not found"},
		{"/tasks", 200, "<html>SPA entry</html>"},
	} {
		res := httptest.NewRecorder()
		srv.Router().ServeHTTP(res, httptest.NewRequest(http.MethodGet, item.path, nil))
		if res.Code != item.status || !strings.Contains(res.Body.String(), item.body) {
			t.Fatal(item.path, res.Code, res.Body.String())
		}
		if item.status == 404 && res.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing asset was cached")
		}
		if item.status == 200 && strings.HasSuffix(item.path, ".mjs") && !strings.Contains(res.Header().Get("Content-Type"), "javascript") {
			t.Fatal("diagram module was not served as JavaScript")
		}
	}
	if retainedAssetPath(current, "/assets/../index.html") != "" {
		t.Fatal("traversal accepted")
	}
	if err := os.Symlink(filepath.Join(current, "index.html"), filepath.Join(old, "assets", "Other-Dead1234.js")); err != nil {
		t.Fatal(err)
	}
	if retainedAssetPath(current, "/assets/Other-Dead1234.js") != "" {
		t.Fatal("retained symlink served")
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "mermaid.esm.min.mjs"), []byte("private"), 0600)
	if err := os.Symlink(outside, filepath.Join(old, "assets", "mermaid-0.0.1")); err != nil {
		t.Fatal(err)
	}
	if retainedAssetPath(current, "/assets/mermaid-0.0.1/mermaid.esm.min.mjs") != "" {
		t.Fatal("retained asset directory symlink escaped the release")
	}
	// Ordinary development checkouts must never search neighboring projects.
	dev := filepath.Join(t.TempDir(), "project", "web", "dist")
	if err := os.MkdirAll(dev, 0700); err != nil {
		t.Fatal(err)
	}
	if retainedAssetPath(dev, "/assets/"+name) != "" {
		t.Fatal("development checkout searched neighbors")
	}
}

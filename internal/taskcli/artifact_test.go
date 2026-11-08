package taskcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestCapability(t *testing.T, apiURL string) (string, string) {
	artifactsDir := t.TempDir()
	capDir := t.TempDir()
	capFile := filepath.Join(capDir, "capability.json")
	capData := Capability{
		API:          apiURL,
		Token:        "secret-token",
		TaskID:       "task-123",
		AttemptID:    "att-456",
		ArtifactsDir: artifactsDir,
	}
	raw, err := json.Marshal(capData)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(capFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", capFile)
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	t.Setenv("CUAN_TASK_ARTIFACTS_DIR", artifactsDir)
	return artifactsDir, capFile
}

func TestArtifactDownloadSuccess(t *testing.T) {
	content := "# Complete Task Report\nAll checks verified 100%."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/artifacts/att_valid_1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="delivered_report.md"`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	artifactsDir, _ := setupTestCapability(t, server.URL)

	var out, errOut bytes.Buffer
	code := Run([]string{"artifact", "download", "att_valid_1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0, got %d. stderr: %s", code, errOut.String())
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON output, got: %s", out.String())
	}
	expectedPath := filepath.Join(artifactsDir, "delivered_report.md")
	if result["saved_path"] != expectedPath {
		t.Fatalf("expected saved_path %q, got %q", expectedPath, result["saved_path"])
	}
	if result["attachment_id"] != "att_valid_1" {
		t.Fatalf("expected attachment_id att_valid_1, got %v", result["attachment_id"])
	}

	savedBytes, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(savedBytes) != content {
		t.Fatalf("saved content mismatch: got %q, want %q", string(savedBytes), content)
	}
}

func TestArtifactDownloadCustomOutput(t *testing.T) {
	content := "Binary or image payload"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	artifactsDir, _ := setupTestCapability(t, server.URL)

	var out, errOut bytes.Buffer
	code := Run([]string{"artifact", "download", "att_bin_1", "--output", "custom_name.bin"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0, got %d. stderr: %s", code, errOut.String())
	}

	expectedPath := filepath.Join(artifactsDir, "custom_name.bin")
	savedBytes, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(savedBytes) != content {
		t.Fatalf("saved content mismatch: got %q, want %q", string(savedBytes), content)
	}
}

func TestArtifactDownloadPathEscapeRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("malicious file"))
	}))
	defer server.Close()

	artifactsDir, _ := setupTestCapability(t, server.URL)

	for _, escapePath := range []string{"../escape.txt", "../../evil.txt", "/tmp/absolute_escape.txt"} {
		var out, errOut bytes.Buffer
		code := Run([]string{"artifact", "download", "att_1", "--output", escapePath}, &out, &errOut)
		if code != 2 {
			t.Errorf("expected exit code 2 for path escape %q, got %d", escapePath, code)
		}
		if !strings.Contains(errOut.String(), "Path escape detected") {
			t.Errorf("expected path escape error in stderr for %q, got: %s", escapePath, errOut.String())
		}
	}

	// Verify no file was created outside artifactsDir
	parentDir := filepath.Dir(artifactsDir)
	if _, err := os.Stat(filepath.Join(parentDir, "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("escape.txt was written outside artifactsDir!")
	}
}

func TestArtifactDownloadAntiOverwrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("NEW CONTENT THAT SHOULD BE REJECTED"))
	}))
	defer server.Close()

	artifactsDir, _ := setupTestCapability(t, server.URL)

	existingFile := filepath.Join(artifactsDir, "existing.txt")
	originalContent := "ORIGINAL UNTOUCHED CONTENT"
	if err := os.WriteFile(existingFile, []byte(originalContent), 0600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := Run([]string{"artifact", "download", "att_1", "--output", "existing.txt"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected code 1 when attempting overwrite, got %d", code)
	}
	if !strings.Contains(errOut.String(), "already exists") {
		t.Fatalf("expected 'already exists' in stderr, got: %s", errOut.String())
	}

	// Verify original file was preserved
	content, err := os.ReadFile(existingFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != originalContent {
		t.Fatalf("existing file was modified: got %q, want %q", string(content), originalContent)
	}
}

func TestArtifactDownloadPartialAndErrorResponses(t *testing.T) {
	status := http.StatusNotFound
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	artifactsDir, _ := setupTestCapability(t, server.URL)

	var out, errOut bytes.Buffer
	code := Run([]string{"artifact", "download", "att_404", "--output", "target.txt"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected code 1 on 404, got %d", code)
	}

	// Confirm no file was created
	if _, err := os.Stat(filepath.Join(artifactsDir, "target.txt")); !os.IsNotExist(err) {
		t.Fatal("partial file was left behind on 404!")
	}
}

func TestArtifactReadText(t *testing.T) {
	content := "# Markdown Report Content\nVerified independently."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	_, _ = setupTestCapability(t, server.URL)

	// JSON mode
	var out, errOut bytes.Buffer
	code := Run([]string{"artifact", "read", "att_md_1"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 for read, got %d. stderr: %s", code, errOut.String())
	}
	var res map[string]any
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("expected JSON, got: %s", out.String())
	}
	if res["content"] != content {
		t.Fatalf("expected content %q, got %q", content, res["content"])
	}

	// Raw mode
	out.Reset()
	errOut.Reset()
	code = Run([]string{"artifact", "read", "att_md_1", "--raw"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 for read --raw, got %d", code)
	}
	if out.String() != content {
		t.Fatalf("expected raw content %q, got %q", content, out.String())
	}
}

func TestArtifactDownloadDirectCommand(t *testing.T) {
	content := "Direct download test payload"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	artifactsDir, _ := setupTestCapability(t, server.URL)

	var out, errOut bytes.Buffer
	code := Run([]string{"download", "att_direct", "--output", "direct.txt"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0, got %d. stderr: %s", code, errOut.String())
	}
	savedBytes, err := os.ReadFile(filepath.Join(artifactsDir, "direct.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(savedBytes) != content {
		t.Fatalf("saved content mismatch")
	}
}

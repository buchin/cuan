package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdater_CompareVersions(t *testing.T) {
	cases := []struct {
		v1, v2   string
		expected int
	}{
		{"0.2.0", "0.1.0", 1},
		{"0.1.0", "0.2.0", -1},
		{"0.1.0", "0.1.0", 0},
		{"v1.0.0", "0.9.9", 1},
		{"0.1.1", "0.1.0", 1},
		{"0.1.0", "0.1.1", -1},
		{"1.2.3", "1.2.3", 0},
	}

	for _, c := range cases {
		got := compareVersions(c.v1, c.v2)
		if got != c.expected {
			t.Errorf("compareVersions(%q, %q) = %d, expected %d", c.v1, c.v2, got, c.expected)
		}
	}
}

func TestUpdater_CheckForUpdateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/testowner/testrepo/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubReleaseDTO{
			TagName:     "v0.2.0",
			Name:        "Release 0.2.0",
			Body:        "New scheduler and updater features",
			HTMLURL:     "https://github.com/testowner/testrepo/releases/tag/v0.2.0",
			PublishedAt: time.Now().UTC(),
		})
	}))
	defer server.Close()

	// Create test service using custom http client pointing to test server
	svc := NewService(Config{
		CurrentVersion: "0.1.0",
		CurrentCommit:  "abc1234",
		RepoOwner:      "testowner",
		RepoName:       "testrepo",
		HTTPClient:     server.Client(),
	})

	// Hijack URL by mocking httpClient transport
	svc.httpClient.Transport = &rewriteTransport{
		target: server.URL,
	}

	ctx := context.Background()
	info, err := svc.CheckForUpdate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.HasUpdate {
		t.Fatalf("expected HasUpdate = true, got false")
	}
	if info.LatestVersion != "0.2.0" {
		t.Fatalf("expected latest version 0.2.0, got %s", info.LatestVersion)
	}
	if info.ReleaseNotes != "New scheduler and updater features" {
		t.Fatalf("unexpected release notes: %s", info.ReleaseNotes)
	}
}

func TestUpdater_SettingsPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-updater-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storagePath := filepath.Join(tmpDir, "system_update.json")

	svc1 := NewService(Config{
		CurrentVersion: "0.1.0",
		StoragePath:    storagePath,
	})

	enabled := false
	customTime := "04:30"
	_, err = svc1.UpdateSettings(SettingsRequest{
		AutoUpdateEnabled: &enabled,
		AutoUpdateTime:    &customTime,
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	// Verify persistence in second instance
	svc2 := NewService(Config{
		CurrentVersion: "0.1.0",
		StoragePath:    storagePath,
	})
	info := svc2.GetInfo()
	if info.AutoUpdateEnabled != false {
		t.Fatalf("expected AutoUpdateEnabled = false, got true")
	}
	if info.AutoUpdateTime != "04:30" {
		t.Fatalf("expected AutoUpdateTime = 04:30, got %s", info.AutoUpdateTime)
	}
}

type rewriteTransport struct {
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	newReq.URL.Scheme = "http"
	newReq.URL.Host = req.URL.Host
	if t.target != "" {
		// Replace host with test server
		targetURL := t.target[len("http://"):]
		newReq.URL.Host = targetURL
	}
	return http.DefaultTransport.RoundTrip(newReq)
}

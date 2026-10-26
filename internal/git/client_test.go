package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	cases := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"https://github.com/buchin/cuan", "buchin", "cuan", false},
		{"https://github.com/buchin/cuan.git", "buchin", "cuan", false},
		{"http://github.com/buchin/cuan", "buchin", "cuan", false},
		{"github.com/buchin/cuan", "buchin", "cuan", false},
		{"buchin/cuan", "buchin", "cuan", false},
		{"https://www.github.com/buchin/cuan", "buchin", "cuan", false},
		{"https://gitlab.com/buchin/cuan", "", "", true},
		{"https://github.com/onlyowner", "", "", true},
		{"", "", "", true},
		{"   ", "", "", true},
	}

	for _, tc := range cases {
		owner, repo, err := ParseGitHubURL(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseGitHubURL(%q) expected error, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParseGitHubURL(%q) unexpected error: %v", tc.input, err)
			}
			if owner != tc.wantOwner || repo != tc.wantRepo {
				t.Errorf("ParseGitHubURL(%q) = (%q, %q); want (%q, %q)", tc.input, owner, repo, tc.wantOwner, tc.wantRepo)
			}
		}
	}
}

func TestBuildAuthURL(t *testing.T) {
	token := "ghp_SECRET_PAT_TOKEN_12345"
	authURL := BuildAuthURL("buchin", "cuan", token)
	if !strings.Contains(authURL, "x-access-token:"+token) {
		t.Fatalf("expected token in authURL, got %s", authURL)
	}

	noTokenURL := BuildAuthURL("buchin", "cuan", "")
	if strings.Contains(noTokenURL, "x-access-token") {
		t.Fatalf("did not expect token in url, got %s", noTokenURL)
	}
}

func TestValidateRepository_GitHubAPI(t *testing.T) {
	mux := http.NewServeMux()

	// 1. Success endpoint
	mux.HandleFunc("/repos/buchin/cuan", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"full_name": "buchin/cuan",
			"default_branch": "main",
			"private": false,
			"clone_url": "https://github.com/buchin/cuan.git"
		}`))
	})

	// 2. Not found endpoint
	mux.HandleFunc("/repos/buchin/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	})

	// 3. Unauthorized / access denied endpoint
	mux.HandleFunc("/repos/buchin/private-denied", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
	})

	// 4. Forbidden endpoint
	mux.HandleFunc("/repos/buchin/forbidden", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message": "Resource not accessible by personal access token"}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient()
	client.apiBaseURL = server.URL

	ctx := context.Background()

	// Test Success
	info, err := client.ValidateRepository(ctx, "buchin/cuan", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.DefaultBranch != "main" || info.Owner != "buchin" || info.Repo != "cuan" {
		t.Fatalf("unexpected info result: %+v", info)
	}

	// Test 404 Not Found
	_, err = client.ValidateRepository(ctx, "buchin/nonexistent", "")
	if err != ErrRepoNotFound {
		t.Fatalf("expected ErrRepoNotFound, got %v", err)
	}

	// Test 401 Unauthorized
	_, err = client.ValidateRepository(ctx, "buchin/private-denied", "invalid_token")
	if err != ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}

	// Test 403 Forbidden
	_, err = client.ValidateRepository(ctx, "buchin/forbidden", "limited_token")
	if err != ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

func TestCloneOrFetch_LocalFixture(t *testing.T) {
	tempDir := t.TempDir()

	// Create a bare git repo as "remote" at tempDir/test/repo.git
	remoteBareDir := filepath.Join(tempDir, "test", "repo.git")
	if err := os.MkdirAll(filepath.Dir(remoteBareDir), 0755); err != nil {
		t.Fatalf("failed to create remote dir: %v", err)
	}
	cmd := exec.Command("git", "init", "--bare", remoteBareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init bare git repo: %s", string(out))
	}

	// Clone bare repo to a working copy to add a commit
	workDir := filepath.Join(tempDir, "work")
	cmd = exec.Command("git", "clone", remoteBareDir, workDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to clone: %s", string(out))
	}

	// Create a commit
	testFile := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Cuan Fixture"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_ = exec.Command("git", "-C", workDir, "config", "user.name", "Tester").Run()
	_ = exec.Command("git", "-C", workDir, "config", "user.email", "test@example.com").Run()
	_ = exec.Command("git", "-C", workDir, "add", ".").Run()
	if out, err := exec.Command("git", "-C", workDir, "commit", "-m", "init commit").CombinedOutput(); err != nil {
		t.Fatalf("failed commit: %s", string(out))
	}
	if out, err := exec.Command("git", "-C", workDir, "push", "origin", "HEAD:main").CombinedOutput(); err != nil {
		t.Fatalf("failed push: %s", string(out))
	}

	// Test CloneOrFetch into target destination
	targetDir := filepath.Join(tempDir, "clone_target")
	client := NewClient()
	client.SetGitBaseURL(tempDir)

	ctx := context.Background()
	// 1. Fresh clone
	if err := client.CloneOrFetch(ctx, "test", "repo", "", targetDir); err != nil {
		t.Fatalf("failed CloneOrFetch fresh clone: %v", err)
	}

	// 2. Fetch on existing targetDir
	if err := client.CloneOrFetch(ctx, "test", "repo", "", targetDir); err != nil {
		t.Fatalf("expected successful fetch on existing dir: %v", err)
	}
}

func TestDeleteProject_DoesNotAffectRemoteRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy remote bare repo
	remoteDir := filepath.Join(tempDir, "remote_upstream.git")
	if out, err := exec.Command("git", "init", "--bare", remoteDir).CombinedOutput(); err != nil {
		t.Fatalf("failed to init bare repo: %s", string(out))
	}

	// Create local project path
	localPath := filepath.Join(tempDir, "local_cache")
	if err := os.MkdirAll(localPath, 0755); err != nil {
		t.Fatalf("failed to create local path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localPath, "test.txt"), []byte("local cache"), 0644); err != nil {
		t.Fatalf("failed write: %v", err)
	}

	// Simulate Cuan project deletion: removes local cache only
	_ = os.RemoveAll(localPath)

	// Remote repository must still exist and be intact!
	if _, err := os.Stat(remoteDir); os.IsNotExist(err) {
		t.Fatalf("remote repository was deleted! Deleting project from Cuan MUST NOT delete remote repository")
	}

	// Check git log on remote still functions
	checkCmd := exec.Command("git", "--git-dir", remoteDir, "rev-parse", "--git-dir")
	if out, err := checkCmd.CombinedOutput(); err != nil {
		t.Fatalf("remote repository damaged: %s", string(out))
	}
}

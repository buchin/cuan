package terminal

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEnsureGitAskpassScript(t *testing.T) {
	path, err := EnsureGitAskpassScript("/usr/local/bin/cuan-server")
	if err != nil {
		t.Fatalf("EnsureGitAskpassScript failed: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read askpass script: %v", err)
	}
	if !strings.Contains(string(content), `'/usr/local/bin/cuan-server' git-askpass "$@"`) {
		t.Fatalf("unexpected script content: %s", string(content))
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("askpass script is not executable: mode=%v err=%v", info.Mode(), err)
	}
}

func TestRunGitAskpassCLI(t *testing.T) {
	if code := RunGitAskpassCLI([]string{"Username for 'https://github.com':"}); code != 0 {
		t.Fatalf("expected exit 0 for username prompt, got %d", code)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer scoped-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"ghs_test_secret_token"}`))
	}))
	defer srv.Close()

	t.Setenv("CUAN_GIT_ASKPASS_URL", srv.URL)
	t.Setenv("CUAN_GIT_ASKPASS_TOKEN", "scoped-token")

	if code := RunGitAskpassCLI([]string{"Password for 'https://x-access-token@github.com':"}); code != 0 {
		t.Fatalf("expected exit 0 for password prompt, got %d", code)
	}

	t.Setenv("CUAN_GIT_ASKPASS_TOKEN", "wrong-token")
	if code := RunGitAskpassCLI([]string{"Password for 'https://x-access-token@github.com':"}); code == 0 {
		t.Fatalf("expected non-zero exit when server returns 401")
	}

	// Verify --config mode works even when env vars are scrubbed by AI CLIs
	t.Setenv("CUAN_GIT_ASKPASS_URL", "")
	t.Setenv("CUAN_GIT_ASKPASS_TOKEN", "")
	scriptPath, err := EnsureGitAskpassScript("/usr/local/bin/cuan-server", "proj_scrub_test", srv.URL, "scoped-token")
	if err != nil {
		t.Fatalf("EnsureGitAskpassScript with config failed: %v", err)
	}
	cfgPath := strings.TrimSuffix(scriptPath, ".sh") + ".json"
	if code := RunGitAskpassCLI([]string{"--config", cfgPath, "Password for 'https://x-access-token@github.com':"}); code != 0 {
		t.Fatalf("expected exit 0 with --config when env vars are scrubbed, got %d", code)
	}
}

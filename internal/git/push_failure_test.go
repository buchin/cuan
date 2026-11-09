package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundPushRetainsSanitizedCommandFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "git")
	script := `#!/bin/sh
case "$*" in
  *symbolic-ref*) echo task/source ;;
  *rev-parse*) echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa ;;
  *push*) echo 'remote: SECRET_TOKEN private/repo' >&2; echo '! [rejected] task/source (non-fast-forward)' >&2; exit 1 ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	client := NewClient()
	client.gitBinary = path
	commit, err := client.pushExactBranch(context.Background(), t.TempDir(), "https://github.com/owner/repo", "task/source", "saved-token")
	if commit != "" || err == nil || !strings.Contains(err.Error(), "remote branch has diverged") || strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Fatal("command failure lost its safe actionable diagnosis", commit, err)
	}
}

func TestPushFailurePreservesRecoveryCategoryWithoutRemoteSecrets(t *testing.T) {
	for _, test := range []struct{ output, want string }{
		{"! [rejected] HEAD -> task (non-fast-forward)", "remote branch has diverged"},
		{"GH013: Repository rule violations", "repository policy rejected"},
		{"fatal: Authentication failed", "repository access was rejected"},
		{"fatal: Could not resolve host", "remote connection failed"},
		{"unexpected remote response", "unknown remote outcome"},
	} {
		err := classifyPushFailure(context.Background(), []byte(test.output+" https://SECRET_TOKEN@github.com/private/repo"))
		if errors.Is(err, ErrRemoteBranchDiverged) != strings.Contains(test.output, "non-fast-forward") {
			t.Fatal("unrelated push failure admitted divergence reconciliation", err)
		}
		if !strings.Contains(err.Error(), test.want) || strings.Contains(err.Error(), "SECRET_TOKEN") || strings.Contains(err.Error(), "private/repo") {
			t.Fatal("missing recovery category or exposed remote output", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := classifyPushFailure(ctx, []byte("Authentication failed")); !strings.Contains(err.Error(), "request interrupted") {
		t.Fatal("interrupted request asserted a terminal remote rejection", err)
	}
}

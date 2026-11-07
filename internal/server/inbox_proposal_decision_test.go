package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestProposalDecisionUsesIsolatedToolFreeProvider(t *testing.T) {
	home, bin, workspace := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte("REPOSITORY_ONLY_RULE"), 0600); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
[ "$PWD" = "$HOME/.cuan-chat-workspace" ] || exit 21
for arg in "$@";do [ "$arg" != "--add-dir" ] || exit 22;done
case "$2" in *"REPOSITORY_ONLY_RULE"*|*"previous tool attempt exceeded"*) exit 23;; esac
case "$2" in *"Do not use tools or inspect repository/host files"*) ;; *) exit 24;; esac
case "$2" in *"A question, objection, deferral or requested change is NOT approval"*) ;; *) exit 25;; esac
printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"The proposal uses two separate tasks.\",\"topic_title\":\"\",\"action\":null}"}}'
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	ctx := context.WithValue(context.Background(), inboxProposalDecisionKey{}, true)
	output, failure := s.executeInboxCLI(ctx, "@cuan", "agy", "Does this use two tasks?", home, workspace, nil)
	if failure != "" || !strings.Contains(output, "two separate tasks") || strings.Contains(output, "cuan-delegate") {
		t.Fatal("proposal question lost isolated no-action answer", output, failure)
	}
}

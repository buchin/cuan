package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
	cuanPrompt "github.com/buchin/cuan/internal/prompt"
	"github.com/buchin/cuan/internal/workflow"
)

func TestNativeCoordinatorPromptIncludesOnePolicyAndOriginalContext(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "-p" ]; then printf '%s' "$2" > "$HOME/coordinator-prompt-fixture"; fi
printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Fixture answer\",\"topic_title\":\"\",\"action\":null}"}}'
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	const guidance = "BOUND_REPOSITORY_GUIDANCE: preserve original outcome and inspect actual evidence."
	if err := os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte(guidance), 0600); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	const original = "ORIGINAL_OWNER_SCOPE: create the requested file with exact five lines; do not open the application repository."
	_, failure := s.executeInboxCLI(context.Background(), "@configured-name", "agy", original, home, workspace, nil)
	if failure != "" {
		t.Fatal(failure)
	}
	data, err := os.ReadFile(filepath.Join(home, "coordinator-prompt-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(data)
	for _, required := range []string{inboxCoordinatorSchema, guidance, original, "@configured-name", cuanPrompt.AutonomousOwnership, cuanPrompt.ConversationStyle} {
		if !strings.Contains(prompt, required) {
			t.Fatal("native prompt lost schema, owner/project guidance or ownership")
		}
	}
	if strings.Count(prompt, workflow.Coordinator) != 1 || strings.Count(prompt, cuanPrompt.ConversationStyle) != 1 || strings.Contains(prompt, "=== AGENTIC SCRUM RULES (§1–§13) ===") {
		t.Fatal("native prompt duplicated workflow or injected a second sprint procedure")
	}
	t.Logf("native coordinator fixture: prompt=%d policy=%d schema=%d original_context=%d bytes", len(prompt), len(inboxCoordinatorInstruction), len(inboxCoordinatorSchema), len(original))
}

package prompt

import (
	"strings"
	"testing"
)

func TestOwnershipMandateIsInjectedIntoCoordinatorAndWorkerPrompts(t *testing.T) {
	for _, p := range []string{GetChatPromptAdditions(t.TempDir()), BuildRunnerPromptWithDoD("Approved work", "Finish original scope", "", nil, t.TempDir())} {
		if !strings.Contains(p, AutonomousOwnership) || !strings.Contains(p, "not whether Cuan should keep trying") {
			t.Fatal("core ownership policy absent")
		}
	}
}

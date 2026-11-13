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
		if !strings.Contains(p, "ROOT CAUSE RESOLUTION (ANTI-TAMBAL SULAM)") || !strings.Contains(p, "Selesaikan masalah user dari akarnya, bukan tambal sulam") {
			t.Fatal("anti-tambal sulam root cause resolution mandate absent")
		}
	}
}

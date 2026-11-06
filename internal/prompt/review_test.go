package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewRetainsRepositoryConstraintsWithoutRepeatingExecutionProcedures(t *testing.T) {
	repo := t.TempDir()
	guidance := "Keep approved scope. Inspect actual390px and desktop screenshots. Preserve original acceptance evidence."
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte(guidance), 0600); err != nil {
		t.Fatal(err)
	}
	review := GetReviewPromptAdditions(repo)
	if !strings.Contains(review, guidance) || !strings.Contains(review, AutonomousOwnership) {
		t.Fatal("review lost required guidance or ownership")
	}
	if strings.Contains(review, "AGENTIC SCRUM RULES (§1–§13)") || strings.Contains(review, GetScrumRules()) {
		t.Fatal("execution/planning procedures duplicated in review")
	}
	if len(review) >= len(GetChatPromptAdditions(repo)) {
		t.Fatal("review context was not reduced")
	}
}

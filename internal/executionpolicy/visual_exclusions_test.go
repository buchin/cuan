package executionpolicy

import (
	"strings"
	"testing"
)

func TestVisualExclusionListsPreserveLaterPositiveWork(t *testing.T) {
	for _, scope := range []string{"tidak ada UI, browser/screenshot, email", "No UI, browser, screenshots or email", "Tanpa UI/UX, browser screenshot, atau email", "Tidak boleh screenshot", "Jangan buka browser", "Do not use browser or screenshots"} {
		if !ExcludesVisualCapture(scope) {
			t.Errorf("capture ban missed: %q", scope)
		}
		stripped := strings.ToLower(WithoutVisualExclusions(scope))
		if strings.Contains(stripped, "screenshot") || strings.Contains(stripped, "browser") || strings.Contains(stripped, "ui") {
			t.Errorf("exclusions requested capabilities: %q", stripped)
		}
		positive := scope + ". Inspect supplied customer.png and run frontend checks."
		kept := WithoutVisualExclusions(positive)
		if !strings.Contains(kept, "customer.png") || !strings.Contains(kept, "frontend checks") {
			t.Fatal("positive work lost", kept)
		}
	}
	for _, scope := range []string{"No UI changes. Capture the existing screenshot.", "No UI, capture screenshot for the owner.", "Run frontend tests without modifying UI.", "Inspect existing browser screenshot."} {
		if ExcludesVisualCapture(scope) {
			t.Errorf("positive capture prohibited: %q", scope)
		}
	}
}

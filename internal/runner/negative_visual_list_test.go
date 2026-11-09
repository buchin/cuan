package runner

import (
	"github.com/buchin/cuan/internal/task"
	"testing"
)

func TestNegativeVisualListDoesNotSeedFrontend(t *testing.T) {
	for _, scope := range []string{"Batasan: tidak ada UI, browser/screenshot, email, scheduler baru.", "No UI, browser, screenshots or email.", "Tanpa UI/UX, browser screenshot, atau email."} {
		claim := &ClaimTaskResponse{Task: &task.Task{Title: "Backend snapshot retention", Instructions: scope}, Verification: &task.VerificationIntent{IsolatedWorkspace: true}}
		if verificationFrontendDependenciesRequired(claim, "") {
			t.Errorf("negative scope requested frontend dependencies: %q", scope)
		}
		claim.Task.Instructions += "\nRun the frontend regression checks."
		if !verificationFrontendDependenciesRequired(claim, "") {
			t.Fatal("positive frontend requirement hidden")
		}
	}
}

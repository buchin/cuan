package runner

import (
	"testing"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

func TestDeveloperFrontendPreparationOnlyOwnsVisualCodeWorkspace(t *testing.T) {
	for _, mode := range []string{"visual developer", "backend developer", "shared workspace", "readonly", "operations", "source verifier", "live inspector", "native release", "missing task", "missing claim"} {
		t.Run(mode, func(t *testing.T) {
			claim := &ClaimTaskResponse{RequiresVision: true, Task: &task.Task{ExecutionCategory: task.ExecutionCategoryCode}}
			isolated := true
			switch mode {
			case "backend developer":
				claim.RequiresVision = false
			case "shared workspace":
				isolated = false
			case "readonly":
				claim.PublicationLimits = executionpolicy.PublicationPolicy{ReadOnlyRepository: true}
			case "operations":
				claim.Task.ExecutionCategory = task.ExecutionCategoryOps
			case "source verifier":
				claim.Verification = &task.VerificationIntent{}
			case "live inspector":
				claim.LiveInspection = &task.LiveInspectionIntent{}
			case "native release":
				claim.NativeRelease = &task.NativeReleaseIntent{}
			case "missing task":
				claim.Task = nil
			case "missing claim":
				claim = nil
			}
			if got := developerFrontendDependenciesRequired(claim, isolated); got != (mode == "visual developer") {
				t.Fatalf("preparation admitted=%t for %s", got, mode)
			}
		})
	}
}

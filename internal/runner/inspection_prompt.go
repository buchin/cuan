package runner

import "github.com/buchin/cuan/internal/project"

// Execution retains the original authorization and policy separately. Supplemental
// verification needs acceptance guidance, never the raw credential snapshot.
func inspectionPromptInstructions(original string) string {
	scope := project.TaskScopeInstructions(original)
	goal, guidance, err := project.TaskReviewGuidance(original)
	if err != nil {
		return scope
	}
	if goal != "" {
		scope += "\n\nSaved project goal:\n" + goal
	}
	if guidance != "" {
		scope += "\n\nSaved project instructions:\n" + guidance
	}
	return scope
}

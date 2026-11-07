package project

// TaskPromptInstructions projects acceptance guidance for model-facing reads.
// Execution retains authorization, policy and environment separately; every role
// receives the saved goal and instructions without the raw credential snapshot.
func TaskPromptInstructions(original string) string {
	scope := TaskScopeInstructions(original)
	goal, guidance, err := TaskReviewGuidance(original)
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

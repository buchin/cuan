package runner

import "github.com/buchin/cuan/internal/task"

// Prepare frontend tooling only for visual development in an isolated code
// workspace. Inspectors, operational jobs and ordinary backend work retain
// their existing preparation paths and execution permissions.
func developerFrontendDependenciesRequired(claim *ClaimTaskResponse, isolated bool) bool {
	return isolated && claim != nil && claim.Task != nil && claim.RequiresVision &&
		task.NormalizeExecutionCategory(claim.Task.ExecutionCategory) == task.ExecutionCategoryCode &&
		!claim.PublicationLimits.ReadOnlyRepository && claim.Verification == nil &&
		claim.LiveInspection == nil && claim.NativeRelease == nil
}

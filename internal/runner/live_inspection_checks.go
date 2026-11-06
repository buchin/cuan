package runner

import (
	"context"
	"errors"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

// A failed check may itself change files. Observe the repository after checks
// on both paths; retaining a pre-check observation would misstate final scope.
func verifyLiveInspectionChecks(ctx context.Context, root, targets string, policy executionpolicy.PublicationPolicy, baseline string) (verificationResult, *RepositoryVerification) {
	if !policy.ReadOnlyRepository || baseline == "" {
		return verificationResult{Error: "Live inspection requires a bound read-only repository baseline"}, nil
	}
	checks := runRegisteredProjectVerification(ctx, root, targets)
	boundary, evidence := verifyTaskRepository(ctx, root, "", policy, baseline)
	result := checks
	if !checks.Passed || !task.PassingProjectCheckSummary(checks.Summary) {
		result.Passed = false
		result.Error = "Live result inspection project checks did not pass"
	}
	if !boundary.Passed {
		result.Passed = false
		result.Error = boundary.Error
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Passed = false
		result.Error = "Live result inspection deadline expired during independent project verification; checks are incomplete"
	}
	return result, evidence
}

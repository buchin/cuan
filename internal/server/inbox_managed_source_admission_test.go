package server

import (
	"github.com/buchin/cuan/internal/executionpolicy"
	"strings"
	"testing"
)

func TestManagedSourceCorrectionRequiresKnownUnpublishedResult(t *testing.T) {
	if policy := executionpolicy.FromInstructions(managedSourceAdmissionRevision(strings.Repeat("a", 40))); policy.ReadOnlyRepository || policy.DenyPush || policy.DenyDeploy {
		t.Fatal("generated repair revision manufactured a publication ban", policy)
	}
	base := inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "publication", Revision: "Publish the independently checked source", ManagerAction: "continue"}
	absent := false
	publication := reviewPublication{Required: true, ResultCommitContained: &absent, ObservedRelease: strings.Repeat("a", 40)}
	if !missingManagedSourceCorrection(base, &publication) {
		t.Fatal("known unpublished source could not be reconciled")
	}
	for _, mode := range []string{"unknown ancestry", "already included", "already published", "not required", "unknown release", "owner input", "wait", "implementation defect", "no revision"} {
		decision, observed := base, publication
		switch mode {
		case "unknown ancestry":
			observed.ResultCommitContained = nil
		case "already included":
			included := true
			observed.ResultCommitContained = &included
		case "already published":
			observed.Verified = true
		case "not required":
			observed.Required = false
		case "unknown release":
			observed.ObservedRelease = ""
		case "owner input":
			decision.RequestedOutcome = "needs_input"
		case "wait":
			decision.ManagerAction = "wait"
		case "implementation defect":
			decision.RemainingWork = "implementation"
		case "no revision":
			decision.Revision = ""
		}
		if missingManagedSourceCorrection(decision, &observed) {
			t.Fatal("unsafe or unrelated reconciliation", mode)
		}
	}
}

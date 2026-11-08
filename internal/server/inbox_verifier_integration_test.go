package server

import (
	"github.com/buchin/cuan/internal/executionpolicy"
	"strings"
	"testing"
)

func TestVerifiedSourceIntegrationRequiresUnchangedNativeFailure(t *testing.T) {
	no := false
	target := strings.Repeat("a", 40)
	decision := inboxReviewEvaluation{Decision: "revise", RemainingWork: "publication"}
	publication := reviewPublication{Required: true, ObservedRelease: target, ResultCommitContained: &no}
	scope := executionpolicy.PublicationPolicy{}
	if !verifiedSourceIntegrationRequired(decision, &publication, &scope, target) {
		t.Fatal("native known integration gap returned to repeated verification")
	}
	unknown := publication
	unknown.ResultCommitContained = nil
	if !verifiedSourceIntegrationRequired(decision, &unknown, &scope, target) {
		t.Fatal("unknown ancestry prevented reconciliation of a native check failure")
	}
	continued := decision
	continued.RemainingWork = "implementation"
	continued.Revision = "Native publication reconciliation: native source integration finding: reconcile exact saved findings"
	if !verifiedSourceIntegrationRequired(continued, &unknown, &scope, target) {
		t.Fatal("reapplying the native guard lost the saved integration decision")
	}
	for _, mode := range []string{"changed release", "missing failure", "already contained", "already published", "publication not required", "readonly owner", "deploy forbidden", "missing scope", "generic verification", "pass"} {
		d, p, s := decision, publication, scope
		approved := &s
		failed := target
		switch mode {
		case "changed release":
			p.ObservedRelease = strings.Repeat("b", 40)
		case "missing failure":
			failed = ""
		case "already contained":
			yes := true
			p.ResultCommitContained = &yes
		case "already published":
			p.Verified = true
		case "publication not required":
			p.Required = false
		case "readonly owner":
			s.ReadOnlyRepository = true
		case "deploy forbidden":
			s.DenyDeploy = true
		case "missing scope":
			approved = nil
		case "generic verification":
			d.RemainingWork = "verification"
		case "pass":
			d.Decision = "pass"
		}
		if verifiedSourceIntegrationRequired(d, &p, approved, failed) {
			t.Fatal("unproven/forbidden integration classified as implementation", mode)
		}
	}
}

package server

import (
	"strings"
	"testing"
)

// The approval prompt is the only place the coordinator learns it may declare
// stages. A prompt that forbids extra fields, or omits the stage contract, makes
// required_stages unreachable no matter how correct the parser is.
func TestApprovalInstructionDescribesStageDeclaration(t *testing.T) {
	if strings.Contains(inboxApprovalInstruction, "Use only title, description, and either project_id or project_name.") {
		t.Fatal("prompt still forbids declaring stages")
	}
	for _, required := range []string{"required_stages", "implement", "verify", "publish"} {
		if !strings.Contains(inboxApprovalInstruction, required) {
			t.Fatalf("approval prompt does not mention %q", required)
		}
	}
	if !strings.Contains(inboxApprovalInstruction, "empty array") {
		t.Fatal("prompt does not explain the empty declaration for read-only work")
	}
	// The prompt must not encourage publishing just because a project deploys.
	if !strings.Contains(inboxApprovalInstruction, "Never add") {
		t.Fatal("prompt lacks the guidance against defaulting to publish")
	}
}

// The approval payload must support what the prompt asks for.
func TestApprovalParserAcceptsDeclaredStages(t *testing.T) {
	content := "Plan ready.\n\n```cuan-approval\n" +
		`{"title":"Fix landing hero","description":"Adjust the hero copy.","project_id":"p1","required_stages":["implement","verify"]}` +
		"\n```"
	plan := parseInboxApprovalPlan(content)
	if plan == nil {
		t.Fatal("declared stages were rejected")
	}
	if len(plan.RequiredStages) != 2 || plan.RequiredStages[0] != "implement" || plan.RequiredStages[1] != "verify" {
		t.Fatalf("stages not preserved: %v", plan.RequiredStages)
	}
}

// An explicit empty declaration must survive parsing as "declared, nothing
// implied" rather than collapsing into "not declared".
func TestApprovalParserKeepsEmptyDeclaration(t *testing.T) {
	content := "Audit only.\n\n```cuan-approval\n" +
		`{"title":"Audit","description":"Read-only audit.","project_id":"p1","required_stages":[]}` +
		"\n```"
	plan := parseInboxApprovalPlan(content)
	if plan == nil {
		t.Fatal("empty stage declaration was rejected")
	}
	if plan.RequiredStages == nil {
		t.Fatal("empty declaration collapsed into undeclared")
	}
	if len(plan.RequiredStages) != 0 {
		t.Fatalf("unexpected stages: %v", plan.RequiredStages)
	}
}

// A typo must be rejected, not silently dropped, or the owner would believe
// they approved publication that never happens.
func TestApprovalParserRejectsUnknownStage(t *testing.T) {
	content := "Plan.\n\n```cuan-approval\n" +
		`{"title":"Deploy","description":"Publish it.","project_id":"p1","required_stages":["deploy"]}` +
		"\n```"
	if plan := parseInboxApprovalPlan(content); plan != nil {
		t.Fatalf("unknown stage accepted: %v", plan.RequiredStages)
	}
}

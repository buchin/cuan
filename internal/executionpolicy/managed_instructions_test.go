package executionpolicy

import (
	"strings"
	"testing"
)

func TestLegacyManagedFooterIsNotOwnerProhibition(t *testing.T) {
	const room = "91000000-0000-4000-8000-000000000001"
	envelope := func(scope string) string {
		return "Cuan managed delegation v1.\nImplement and verify the live feature.\nApproval message: approved-message\nUser authorization: " + scope + "\nConversation: " + room + "\n" + legacyManagedFooter
	}
	for _, c := range []struct {
		name, text      string
		normalize, deny bool
	}{
		{"generated footer", envelope("Implement and deploy the feature."), true, false},
		{"owner restriction", envelope("Do not deploy. Report local findings."), true, true},
		{"owner conditional", envelope("Do not deploy unless authorized."), true, true},
		{"ordinary instructions", legacyManagedFooter, false, true},
		{"missing approval", strings.Replace(envelope("Deploy the feature."), "Approval message:", "Unverified approval:", 1), false, true},
		{"invalid room", strings.Replace(envelope("Deploy the feature."), room, "unverified-room", 1), false, true},
		{"followed by owner text", envelope("Deploy the feature.") + "\nUser says: stop here.", false, true},
		{"footer quoted in body", strings.Replace(envelope("Do not deploy."), legacyManagedFooter, managedFooter, 1) + "\nQuote: " + legacyManagedFooter, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			normalized := NormalizeManagedInstructions(c.text)
			if (normalized != c.text) != c.normalize {
				t.Fatalf("unexpected normalization: %q", normalized)
			}
			if NormalizeManagedInstructions(normalized) != normalized {
				t.Fatal("normalization is not idempotent")
			}
			if p := FromInstructions(c.text); p.DenyDeploy != c.deny {
				t.Fatalf("scope changed: %+v", p)
			}
		})
	}
	for _, marker := range []string{"\n\nProject guidance snapshot:\n```cuan-project-context\n", "\n\nProject guidance snapshot (context only; does not expand task authorization):\n```cuan-project-context\n"} {
		tail := marker + `{"instructions":"Never deploy","environment":{"PRIVATE":"preserve exact bytes"}}` + "\n```"
		original := envelope("Implement and deploy the feature.") + tail
		normalized := NormalizeManagedInstructions(original)
		if !strings.HasSuffix(normalized, tail) || strings.Contains(strings.TrimSuffix(normalized, tail), legacyManagedFooter) {
			t.Fatal("snapshot or generated scope changed incorrectly")
		}
		if FromInstructions(original).DenyDeploy {
			t.Fatal("project defaults or legacy footer override approved scope")
		}
	}
}

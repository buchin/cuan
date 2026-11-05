package executionpolicy

import (
	"regexp"
	"strings"
)

const legacyManagedFooter = "Report actual progress and verification evidence. Stay within this scope; do not deploy unless authorized."
const managedFooter = "Report actual progress and verification evidence. Stay within this scope."

var managedFooterEnvelope = regexp.MustCompile(`\nConversation: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\n` + regexp.QuoteMeta(legacyManagedFooter) + `[\t\r\n ]*$`)

// NormalizeManagedInstructions removes only the obsolete conditional guard in
// Cuan's exact generated footer. It never grants execution permission, removes
// an owner's restriction, changes stored instructions or rewrites their hash.
func NormalizeManagedInstructions(instructions string) string {
	if !strings.HasPrefix(instructions, "Cuan managed delegation v1.\n") {
		return instructions
	}
	body, tail := instructions, ""
	for _, marker := range []string{"\n\nProject guidance snapshot:\n```cuan-project-context\n", "\n\nProject guidance snapshot (context only; does not expand task authorization):\n```cuan-project-context\n"} {
		if i := strings.LastIndex(body, marker); i >= 0 {
			body, tail = body[:i], body[i:]+tail
		}
	}
	if !strings.Contains(body, "\nApproval message: ") || !strings.Contains(body, "\nUser authorization: ") {
		return instructions
	}
	span := managedFooterEnvelope.FindStringIndex(body)
	if span == nil {
		return instructions
	}
	return body[:span[0]] + strings.Replace(body[span[0]:span[1]], legacyManagedFooter, managedFooter, 1) + tail
}

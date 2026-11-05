package executionpolicy

import (
	"regexp"
	"strings"
)

const legacyManagedFooter = "Report actual progress and verification evidence. Stay within this scope; do not deploy unless authorized."
const managedFooter = "Report actual progress and verification evidence. Stay within this scope."

var managedFooterEnvelope = regexp.MustCompile(`\nConversation: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\n` + regexp.QuoteMeta(legacyManagedFooter) + `[\t\r\n ]*$`)

// NormalizeManagedInstructions removes only the obsolete conditional guard in
// Cuan's exact generated footer when the original scope already requests publication.
// It never grants execution permission, removes
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
	if span == nil || !managedPublicationRequested(body[:span[0]]) {
		return instructions
	}
	return body[:span[0]] + strings.Replace(body[span[0]:span[1]], legacyManagedFooter, managedFooter, 1) + tail
}

var managedInlineCode = regexp.MustCompile("`+[^`\\n]*`+")
var managedScopeClause = regexp.MustCompile(`[.!?;\n]+`)
var managedNegativePublication = regexp.MustCompile(`(?i)\b(?:do not|don't|never|jangan|dilarang|tanpa|without|not required|tidak perlu|no deployment)\b`)
var managedDeployRequest = regexp.MustCompile(`(?i)(?:^|[\s:,(])(?:deploy|ter-?deploy|deploying)(?:\s|$)|\b(?:must|should|wajib|harus|be)\s+(?:deployed)\b`)
var managedReleaseRequest = regexp.MustCompile(`(?i)\b(?:release|rilis|ship|publish)\b`)
var managedProductionTarget = regexp.MustCompile(`(?i)\b(?:production|produksi|live|prod)\b`)

func managedPublicationRequested(scope string) bool {
	fenced := false
	for _, line := range strings.Split(scope, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced || strings.HasPrefix(strings.TrimSpace(line), ">") {
			continue
		}
		line = managedInlineCode.ReplaceAllString(line, "")
		for _, clause := range managedScopeClause.Split(line, -1) {
			clause = strings.TrimSpace(clause)
			if managedNegativePublication.MatchString(clause) {
				continue
			}
			if managedDeployRequest.MatchString(clause) || (managedReleaseRequest.MatchString(clause) && managedProductionTarget.MatchString(clause)) {
				return true
			}
		}
	}
	return false
}

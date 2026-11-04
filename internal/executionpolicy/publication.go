package executionpolicy

import (
	"regexp"
	"strings"
)

// PublicationPolicy can only restrict project defaults, never grant publication
// permission. Project guidance is not a replacement for the approved task scope.
type PublicationPolicy struct {
	DenyPush   bool `json:"deny_push"`
	DenyDeploy bool `json:"deny_deploy"`
}

var publicationProhibition = regexp.MustCompile(`(?i)\b(?:jangan|dilarang|tanpa|do not|don't|must not|never|no)\b[^\n.!?]{0,400}`)
var pushAction = regexp.MustCompile(`(?i)\b(?:push|publishing|publish)\b`)
var deployAction = regexp.MustCompile(`(?i)\b(?:deploy|deployment|deploying)\b`)

func FromInstructions(instructions string) PublicationPolicy {
	// Snapshotted project guidance may require deployment. Only the task's own
	// scope and authorization can narrow that requirement.
	if i := strings.Index(instructions, "\nProject guidance snapshot:"); i >= 0 {
		instructions = instructions[:i]
	}
	policy := PublicationPolicy{}
	for _, clause := range publicationProhibition.FindAllString(instructions, -1) {
		policy.DenyPush = policy.DenyPush || pushAction.MatchString(clause)
		policy.DenyDeploy = policy.DenyDeploy || deployAction.MatchString(clause)
	}
	return policy
}

func (p PublicationPolicy) Prompt() string {
	if !p.DenyPush && !p.DenyDeploy {
		return ""
	}
	s := "\n## Binding task publication limits\nThese task limits override project defaults and generic delivery instructions. Report and deliver the local artifact without inventing a deployment requirement.\n"
	if p.DenyPush {
		s += "Git push, remote branch publication and pull requests are forbidden for this task. Do not run task push.\n"
	}
	if p.DenyDeploy {
		s += "Deployment and automatic apply/merge are forbidden for this task. Do not release or change production.\n"
	}
	return s
}

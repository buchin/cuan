package executionpolicy

import (
	"regexp"
)

// PublicationPolicy can only restrict project defaults, never grant publication
// permission. Project guidance is not a replacement for the approved task scope.
type PublicationPolicy struct {
	ReadOnlyRepository bool `json:"read_only_repository"`
	DenyPush           bool `json:"deny_push"`
	DenyDeploy         bool `json:"deny_deploy"`
}

var publicationProhibition = regexp.MustCompile(`(?i)\b(?:jangan|dilarang|tanpa|tidak|do not|don't|must not|never|no)\b[^\n.!?]{0,400}`)
var repositoryTarget = regexp.MustCompile(`(?i)\b(?:repo|repository|repositori|git|kode|code)\b`)
var pushAction = regexp.MustCompile(`(?i)\b(?:push|publishing|publish)\b`)
var readOnlyScope = regexp.MustCompile(`(?im)(?:\b(?:audit|investigation|inspection|investigasi|pemeriksaan)\s+(?:read[ -]only|baca[ -]saja)\b|\bread[ -]only\s+(?:audit|investigation|inspection)\b|\b(?:repository|repositori|repo)\s+(?:is |must remain |harus tetap )?read[ -]only\b|^\s*(?:scope|batasan)\s*:\s*read[ -]only\b)`)
var deployAction = regexp.MustCompile(`(?i)\b(?:deploy|deployment|deploying)\b`)

func FromInstructions(instructions string) PublicationPolicy {
	instructions = publicationScope(instructions)
	policy := PublicationPolicy{ReadOnlyRepository: readOnlyScope.MatchString(instructions) && repositoryTarget.MatchString(instructions)}
	if policy.ReadOnlyRepository {
		policy.DenyPush, policy.DenyDeploy = true, true
	}
	for _, clause := range publicationProhibition.FindAllString(instructions, -1) {
		policy.DenyPush = policy.DenyPush || pushAction.MatchString(clause)
		policy.DenyDeploy = policy.DenyDeploy || deployAction.MatchString(clause)
	}
	if policy.DenyDeploy && standaloneArtifactShape(instructions) {
		policy.DenyPush = true
	}
	return policy
}

func (p PublicationPolicy) Prompt() string {
	if !p.DenyPush && !p.DenyDeploy && !p.ReadOnlyRepository {
		return ""
	}
	s := "\n## Binding task publication limits\nThese task limits override project defaults and generic delivery instructions. Report and deliver the local artifact without inventing a deployment requirement.\n"
	if p.ReadOnlyRepository {
		s += "This repository is read-only for this attempt. Do not modify code, configuration, test scripts, documentation, the git index or commits. Deliver the audit in the task report. Create verification files outside the repository under CUAN_TASK_ARTIFACTS_DIR and attach them with --artifact session:relative/path. Broken unrelated project tests are observations, not permission to repair them. Automatic correction feedback cannot expand this scope.\n"
	}
	if p.DenyPush {
		s += "Git push, remote branch publication and pull requests are forbidden for this task. Do not run task push.\n"
	}
	if p.DenyDeploy {
		s += "Deployment and automatic apply/merge are forbidden for this task. Do not release or change production.\n"
	}
	return s
}

// Restrict combines independently trusted scope boundaries; neither can relax the other.
func (p PublicationPolicy) Restrict(other PublicationPolicy) PublicationPolicy {
	p.ReadOnlyRepository = p.ReadOnlyRepository || other.ReadOnlyRepository
	p.DenyPush = p.DenyPush || other.DenyPush || p.ReadOnlyRepository
	p.DenyDeploy = p.DenyDeploy || other.DenyDeploy || p.ReadOnlyRepository
	return p
}

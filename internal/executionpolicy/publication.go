package executionpolicy

import (
	"fmt"
	"regexp"
)

// PublicationPolicy can only restrict project defaults, never grant publication
// permission. Project guidance is not a replacement for the approved task scope.
type PublicationPolicy struct {
	ReadOnlyRepository bool `json:"read_only_repository"`
	DenyPush           bool `json:"deny_push"`
	DenyDeploy         bool `json:"deny_deploy"`
}

// A semicolon starts a separate scope clause, just like a sentence boundary.
// Do not let "tidak melakukan copy; ... native deploy" prohibit publication.
var publicationProhibition = regexp.MustCompile(`(?i)\b(?:jangan|dilarang|tanpa|tidak|do not|don't|must not|never|no)\b[^;\n.!?]{0,400}`)

// An explicit subsequent imperative starts a new clause. In particular,
// "tidak tertimpa ... lalu lanjutkan deploy" does not prohibit deployment.
// Any prohibition before the transition is still evaluated independently.
var publicationClauseTransition = regexp.MustCompile(`(?i)\b(?:(?:lalu|kemudian)\s+(?:lakukan|jalankan|lanjutkan)|then\s+(?:perform|execute|run|continue))\b`)

// This exact native header describes a completed predecessor. It is not the
// new attempt's scope. Keep explicit owner/attempt bans elsewhere untouched.
var completedNativeInspection = regexp.MustCompile(`(?i)^(Native publication reconciliation: the retained source (?:[a-f0-9]{40}|[a-f0-9]{64}) has completed a source-bound )read-only (inspection; the current registered release does not contain that commit\.)`)

// Retrospective references to a prior completed read-only audit (e.g.
// "Berdasarkan hasil audit read-only...", "riwayat audit `read-only`", or "audit read-only sebelumnya")
// describe context/motivation for a follow-up code task, not a read-only lock on it.
var historicalAuditReference = regexp.MustCompile(`(?i)\b(?:berdasarkan|dari|setelah|pasca|mengacu(?:\s+pada)?|sesuai|terkait|riwayat|hasil|temuan|laporan)\s+(?:hasil\s+|temuan\s+|laporan\s+|riwayat\s+)?(?:(?:task|tugas)\s+)?(?:audit|investigation|inspection|investigasi|pemeriksaan)\s+[` + "`" + `"']?(?:read[ -]only|baca[ -]saja)[` + "`" + `"']?\b|\b(?:audit|investigation|inspection|investigasi|pemeriksaan)\s+[` + "`" + `"']?(?:read[ -]only|baca[ -]saja)[` + "`" + `"']?\s+(?:sebelumnya|terdahulu|kemarin|lalu|tadi)\b|\b(?:based\s+on|from|after|following|per|prior|previous|completed|historical)\s+(?:the\s+)?(?:(?:audit|investigation|inspection)\s+[` + "`" + `"']?(?:read[ -]only|baca[ -]saja)[` + "`" + `"']?|[` + "`" + `"']?read[ -]only[` + "`" + `"']?\s+(?:audit|investigation|inspection))\b`)
var repositoryTarget = regexp.MustCompile(`(?i)\b(?:repo|repository|repositori|git|kode|code)\b`)
var pushAction = regexp.MustCompile(`(?i)\b(?:push|publishing|publish)\b`)
var readOnlyScope = regexp.MustCompile(`(?im)(?:\b(?:audit|investigation|inspection|investigasi|pemeriksaan)\s+(?:read[ -]only|baca[ -]saja)\b|\bread[ -]only\s+(?:audit|investigation|inspection)\b|\b(?:repository|repositori|repo)\s+(?:is |must remain |harus tetap )?read[ -]only\b|^\s*(?:scope|batasan)\s*:\s*read[ -]only\b)`)
var deployAction = regexp.MustCompile(`(?i)\b(?:deploy|deployment|deploying)\b`)

// Negating cancellation is not a deployment prohibition. Rename only its
// deployment noun; retain the negation for coordinated push/deploy prohibitions.
// This does not grant publication authority or relax an independent scope.
var deploymentCancellationCaution = regexp.MustCompile(`(?i)\b((?:tanpa\s+membatalkan|jangan\s+(?:membatalkan|batalkan)|do not\s+cancel|don't\s+cancel))\s+(?:deploy|deployment)\b`)

// A prohibition explicitly limited to the inspect command describes product
// behavior. It cannot grant publication: a separate positive task request is
// required, and any other task-wide prohibition still wins.
var inspectBehaviorLimit = regexp.MustCompile("(?i)^\\s*(?:jangan\\s+(?:jalankan|menjalankan)|do not\\s+(?:run|execute))\\s+(?:(?:tool|tools)\\s+(?:atau|or)\\s+)?(?:script\\s+)?deploy\\s+(?:saat|ketika|during)\\s+(?:`?inspect`?|(?:the\\s+)?inspect\\s+command)\\s*$")

// These bounded cautions prohibit an unverified/repeated effect, not all
// publication. Recognize them only alongside a separate positive publication
// request. Original bans, read-only scope and unknown qualifiers still win.
var publicationEvidenceLimit = regexp.MustCompile(`(?i)^\s*(?:do not repeat deployment solely to obtain evidence|jangan (?:lakukan|melakukan) deploy (?:buta|secara buta)(?: atau menandai task selesai sebelum ada bukti valid)?|jangan mengulang (?:broad suite atau )?deploy hanya demi bukti(?: bila bukti native yang sah sudah ada)?)\s*$`)

// Prohibiting manual deployment (e.g. "Jangan deploy manual") forbids ad-hoc
// manual releases, not Cuan's automated apply/merge and deployment pipeline.
var manualDeployCaution = regexp.MustCompile(`(?i)^\s*(?:jangan|dilarang|tanpa|tidak|do not|don't|must not|never|no)\s+(?:(?:lakukan|melakukan|jalankan|menjalankan|run|execute|perform)\s+)?(?:manual\s+(?:deploy|deployment|deploying)|(?:deploy|deployment|deploying)\s+(?:manual|secara\s+manual|manually)|manually\s+(?:deploy|deployment|deploying))(?:\s+(?:ke|to)\s+(?:production|produksi|live|prod|server|vps))?\s*$`)

// Prohibiting direct push from inside a runner session (e.g. "Jangan melakukan
// push langsung ke main ... dari dalam sesi runner") forbids runner-side pushes,
// not Cuan's automated apply/merge pipeline when managed publication is requested.
var runnerPushCaution = regexp.MustCompile(`(?i)\b(?:dari\s+(?:dalam\s+)?sesi\s+runner|di\s+(?:dalam\s+)?sesi\s+runner|from\s+(?:inside\s+)?(?:the\s+)?runner(?:\s+session)?|in\s+(?:the\s+)?runner\s+session)\b`)

// Negating false positives (e.g. "Tidak ada false positive: deployment ...")
// asserts product correctness, not a prohibition on deployment.
var falsePositiveAssertion = regexp.MustCompile(`(?i)\b(?:tidak\s+ada|tanpa|no)\s+false\s+positives?\b\s*:?`)

// Describing a mismatch symptom (e.g. "tidak cocok ... skrip deploy",
// "do not match") is not a prohibition on deployment.
var mismatchCondition = regexp.MustCompile(`(?i)\b(?:tidak\s+(?:cocok|sesuai|identik|sama)|(?:do|does|did)\s+not\s+match)\b`)

var publicationContractLine = regexp.MustCompile(`(?m)^Publication policy contract:\s*allow_push=(true|false)\s+allow_deploy=(true|false)\s+read_only_repository=(true|false)\s*$`)

// FormatContract formats an explicit machine-readable publication policy
// contract emitted by the AI Coordinator or approved plan.
func FormatContract(allowPush, allowDeploy, readOnly bool) string {
	if readOnly {
		allowPush, allowDeploy = false, false
	}
	return fmt.Sprintf("Publication policy contract: allow_push=%t allow_deploy=%t read_only_repository=%t", allowPush, allowDeploy, readOnly)
}

func FromInstructions(instructions string) PublicationPolicy {
	instructions = publicationScope(instructions)
	if match := publicationContractLine.FindStringSubmatch(instructions); len(match) == 4 {
		allowPush := match[1] == "true"
		allowDeploy := match[2] == "true"
		readOnly := match[3] == "true"
		if readOnly {
			return PublicationPolicy{ReadOnlyRepository: true, DenyPush: true, DenyDeploy: true}
		}
		return PublicationPolicy{
			ReadOnlyRepository: false,
			DenyPush:           !allowPush,
			DenyDeploy:         !allowDeploy,
		}
	}
	instructions = completedNativeInspection.ReplaceAllString(instructions, "${1}${2}")
	instructions = historicalAuditReference.ReplaceAllString(instructions, " prior audit ")
	instructions = deploymentCancellationCaution.ReplaceAllString(instructions, "${1} release")
	instructions = falsePositiveAssertion.ReplaceAllString(instructions, " ")
	instructions = mismatchCondition.ReplaceAllString(instructions, " mismatch ")
	policy := PublicationPolicy{ReadOnlyRepository: readOnlyScope.MatchString(instructions) && repositoryTarget.MatchString(instructions)}
	if policy.ReadOnlyRepository {
		policy.DenyPush, policy.DenyDeploy = true, true
	}
	clauses := publicationClauseTransition.ReplaceAllString(instructions, "\n$0")
	for _, clause := range publicationProhibition.FindAllString(clauses, -1) {
		policy.DenyPush = policy.DenyPush || (pushAction.MatchString(clause) && !(runnerPushCaution.MatchString(clause) && managedPublicationRequested(instructions)))
		policy.DenyDeploy = policy.DenyDeploy || (deployAction.MatchString(clause) && !manualDeployCaution.MatchString(clause) && !((inspectBehaviorLimit.MatchString(clause) || publicationEvidenceLimit.MatchString(clause) || runnerPushCaution.MatchString(clause)) && managedPublicationRequested(instructions)))
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

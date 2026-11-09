package runner

import (
	"regexp"
	"strings"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

var (
	visualScopeRegex          = regexp.MustCompile(`(?i)\b(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)\b|\.(?:tsx|jsx|css)\b`)
	visualMutationProhibition = regexp.MustCompile(`(?i)\b(?:no|without|tanpa|tidak ada|jangan|dilarang|do not|don't|never)\s+(?:(?:push|pr|deploy|deployment|perubahan konfigurasi|configuration changes?)(?:\s*,\s*|\s+(?:or|and|atau|dan)\s+))*(?:(?:perubahan|modifikasi|ubah|mengubah|menyentuh|change|changes? to|modify|edit|touch)\s+(?:(?:komponen|components?|elemen)\s+)?(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)(?:\s*(?:/|,|\bor\b|\band\b|\batau\b|\bdan\b)\s*(?:komponen\s+|components?\s+|elemen\s+)?(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual))*\b|(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)\s+(?:component\s+|komponen\s+)?changes?\b)`)
	visualCaptureProhibition  = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang)\s+(?:take|capture|create|generate|use|buat|ambil|membuat|mengambil|gunakan|menggunakan|pakai)\s+|(?:no|without|tanpa)\s+)(?:(?:(?:browser\s+automation|(?:otomasi|otomatisasi)\s+browser)\s+(?:or|and|atau|dan)\s+))?(?:new\s+)?(?:screenshots?|tangkapan layar)\b`)
	browserAutomationRegex    = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang)\s+(?:use|run|launch|open|invoke|gunakan|menggunakan|pakai|memakai|jalankan|menjalankan|buka|membuka)\s+(?:(?:a|an|the|any|automated|headless|automation|otomasi|otomatisasi|alat|tool|tools)\s+)*(?:browser|playwright|puppeteer|chromium)|(?:no|without|tanpa)\s+(?:(?:otomasi|otomatisasi|headless)\s+)?(?:browser|playwright|puppeteer|chromium))\b`)
	commandOutputScope        = regexp.MustCompile(`(?i)--help\b|\b(?:cli|command-line|terminal|stdout|stderr)\b`)
	webOutputScope            = regexp.MustCompile(`(?i)\b(?:halaman|page|browser|web|component|komponen|panel)\b`)
	ambiguousDisplayScope     = regexp.MustCompile(`(?i)\btampilan\b`)
)

// hasFrontendVerificationScope checks whether a text scope requires frontend/visual tooling,
// stripping mutation prohibitions (e.g. "tanpa modifikasi UI", "Do not modify UI") and capture prohibitions
// so that interface constraints do not falsely trigger frontend scope in backend tasks, nor block
// legitimate frontend verification tasks that carry modification restrictions.
func hasFrontendVerificationScope(text string) bool {
	text = executionpolicy.WithoutVisualExclusions(text)
	clean := visualMutationProhibition.ReplaceAllString(text, "")
	clean = visualCaptureProhibition.ReplaceAllString(clean, "")
	clean = browserAutomationRegex.ReplaceAllString(clean, "")
	for _, line := range strings.Split(clean, "\n") {
		if commandOutputScope.MatchString(line) && !webOutputScope.MatchString(line) {
			line = ambiguousDisplayScope.ReplaceAllString(line, "")
		}
		if visualScopeRegex.MatchString(line) {
			return true
		}
	}
	return false
}

// Prepare frontend tooling only for visual development in an isolated code
// workspace. Inspectors, operational jobs and ordinary backend work retain
// their existing preparation paths and execution permissions.
func developerFrontendDependenciesRequired(claim *ClaimTaskResponse, isolated bool) bool {
	return isolated && claim != nil && claim.Task != nil && claim.RequiresVision &&
		task.NormalizeExecutionCategory(claim.Task.ExecutionCategory) == task.ExecutionCategoryCode &&
		!claim.PublicationLimits.ReadOnlyRepository && claim.Verification == nil &&
		claim.LiveInspection == nil && claim.NativeRelease == nil
}

// verificationFrontendDependenciesRequired determines whether an isolated source
// verifier needs frontend dependencies. Pure backend tasks skip node_modules seeding,
// while tasks with vision requirements, registered visual recipes, or frontend scope
// retain complete dependencies per manifest and lockfile even when UI modification is prohibited.
// Optional visual verification scripts in the repository (such as scripts/verify-ui.sh)
// that are not bound to the claim do not trigger frontend dependencies for pure backend tasks.
func verificationFrontendDependenciesRequired(claim *ClaimTaskResponse, _ string) bool {
	if claim == nil || claim.Verification == nil {
		return false
	}
	if claim.RequiresVision {
		return true
	}
	if claim.VisualRecipe != nil {
		return true
	}
	if claim.Task != nil {
		if hasFrontendVerificationScope(claim.Task.Title) || hasFrontendVerificationScope(claim.Task.Instructions) {
			return true
		}
		for _, item := range claim.Task.DoDChecklist {
			if hasFrontendVerificationScope(item.Text) {
				return true
			}
		}
	}
	return false
}

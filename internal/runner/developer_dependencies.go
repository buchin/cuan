package runner

import (
	"regexp"

	"github.com/buchin/cuan/internal/task"
)

var (
	visualScopeRegex       = regexp.MustCompile(`(?i)\b(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)\b|\.(?:tsx|jsx|css)\b`)
	visualProhibitedRegex  = regexp.MustCompile(`(?i)\b(?:no|without|tanpa|tidak ada|jangan|dilarang|do not|don't|never)\s+(?:(?:push|pr|deploy|deployment|perubahan konfigurasi|configuration changes?)(?:\s*,\s*|\s+(?:or|and|atau|dan)\s+))*(?:(?:perubahan|modifikasi|ubah|mengubah|menyentuh|change|changes? to|modify|edit|touch)\s+(?:(?:komponen|components?|elemen)\s+)?(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)\b|(?:ui|ux|frontend|front-end|layout|typography|styling|antarmuka|tampilan|desain|visual)\s+changes?\b)`)
	visualCaptureRegex     = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang)\s+(?:take|capture|create|generate|use|buat|ambil|membuat|mengambil|gunakan|menggunakan|pakai)\s+|(?:no|without|tanpa)\s+)(?:(?:(?:browser\s+automation|(?:otomasi|otomatisasi)\s+browser)\s+(?:or|and|atau|dan)\s+))?(?:new\s+)?(?:screenshots?|tangkapan layar)\b`)
	browserAutomationRegex = regexp.MustCompile(`(?i)\b(?:(?:do not|don't|never|jangan|dilarang)\s+(?:use|run|launch|open|invoke|gunakan|menggunakan|pakai|memakai|jalankan|menjalankan|buka|membuka)\s+(?:(?:a|an|the|any|automated|headless|automation|otomasi|otomatisasi|alat|tool|tools)\s+)*(?:browser|playwright|puppeteer|chromium)|(?:no|without|tanpa)\s+(?:(?:otomasi|otomatisasi|headless)\s+)?(?:browser|playwright|puppeteer|chromium))\b`)
)

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
// retain complete dependencies per manifest and lockfile.
func verificationFrontendDependenciesRequired(claim *ClaimTaskResponse, workingDir string) bool {
	if claim == nil || claim.Verification == nil {
		return false
	}
	if claim.RequiresVision {
		return true
	}
	if claim.VisualRecipe != nil {
		return true
	}
	if claim.PHPUnitTargets != "" {
		return false
	}
	instructions := ""
	if claim.Task != nil {
		instructions = claim.Task.Instructions
	}
	prohibited := visualProhibitedRegex.MatchString(instructions) ||
		visualCaptureRegex.MatchString(instructions) ||
		browserAutomationRegex.MatchString(instructions)

	if !prohibited {
		if claim.Verification.SourceCommit != "" {
			recipe, err := VisualRecipeAt(workingDir, claim.Verification.SourceCommit)
			if err == nil && recipe != nil {
				return true
			}
		}
		if claim.Task != nil {
			if visualScopeRegex.MatchString(claim.Task.Title + "\n" + instructions) {
				return true
			}
			for _, item := range claim.Task.DoDChecklist {
				if visualScopeRegex.MatchString(item.Text) {
					return true
				}
			}
		}
	}
	return false
}

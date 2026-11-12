package task

import "strings"

// Acceptance stage declarations. A task states which stages it must reach, so
// the runner no longer has to guess intent from prose or from a project-wide
// "auto deploy" default. Nil RequiredStages means "not declared" and keeps the
// behaviour that existed before this field, so older tasks are unaffected.
const (
	// StageImplement means the work changes something and must be produced.
	StageImplement = "implement"
	// StageVerify means the result must carry independent acceptance evidence.
	StageVerify = "verify"
	// StagePublish means the change must reach the declared production target
	// with a verified receipt.
	StagePublish = "publish"
)

// validStages is the closed vocabulary an approval may declare.
var validStages = map[string]bool{
	StageImplement: true,
	StageVerify:    true,
	StagePublish:   true,
}

// NormalizeStages trims, lowercases and deduplicates a declared stage list
// while preserving order. Unknown values are dropped; the caller validates
// separately so a typo is reported rather than silently ignored.
func NormalizeStages(stages []string) []string {
	if stages == nil {
		return nil
	}
	seen := make(map[string]bool, len(stages))
	out := make([]string, 0, len(stages))
	for _, raw := range stages {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// InvalidStages returns declared values outside the supported vocabulary.
func InvalidStages(stages []string) []string {
	var invalid []string
	for _, raw := range stages {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" || validStages[value] {
			continue
		}
		invalid = append(invalid, value)
	}
	return invalid
}

// HasStage reports whether the declaration includes a stage. A nil declaration
// never contains a stage; callers that need legacy behaviour must check for nil
// explicitly rather than relying on this helper.
func HasStage(stages []string, stage string) bool {
	for _, value := range stages {
		if strings.EqualFold(strings.TrimSpace(value), stage) {
			return true
		}
	}
	return false
}

// RequiresPublish reports whether the task declared that its result must reach
// a production target. This replaces reading a project-wide auto-deploy default
// as if it were a per-task requirement.
func RequiresPublish(stages []string) bool {
	return HasStage(stages, StagePublish)
}

// StagesDeclared distinguishes "no declaration" (legacy) from "declared, with
// no stages" (explicitly nothing implied). The distinction matters because the
// two cases route to different behaviour.
func StagesDeclared(stages []string) bool {
	return stages != nil
}

// DescribeStages renders a declaration in owner language, never internal jargon.
func DescribeStages(stages []string) string {
	if !StagesDeclared(stages) {
		return ""
	}
	if len(stages) == 0 {
		return "tanpa tahap lanjutan"
	}
	parts := make([]string, 0, len(stages))
	for _, stage := range stages {
		switch stage {
		case StageImplement:
			parts = append(parts, "kerjakan perubahannya")
		case StageVerify:
			parts = append(parts, "pastikan hasilnya benar")
		case StagePublish:
			parts = append(parts, "terbitkan ke produksi")
		}
	}
	return strings.Join(parts, ", ")
}

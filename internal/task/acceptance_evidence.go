package task

import "strings"

// AcceptanceGaps distinguishes a reported result from an unsubstantiated exit 0.
// Evidence is recorded per original criterion; these are reported observations,
// not a substitute for the runner's independent verification checks.
func AcceptanceGaps(t *Task) []string {
	var gaps []string
	if strings.TrimSpace(t.Summary) == "" {
		gaps = append(gaps, "A work summary is required.")
	}
	if strings.TrimSpace(t.CompleteNote) == "" {
		gaps = append(gaps, "Verification evidence is required.")
	}
	for _, item := range t.DoDChecklist {
		if !item.Completed || strings.TrimSpace(item.Evidence) == "" {
			gaps = append(gaps, "Acceptance criterion needs verification evidence: "+item.Text)
		}
	}
	return gaps
}

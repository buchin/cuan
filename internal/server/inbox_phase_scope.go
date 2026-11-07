package server

import (
	"fmt"
	"strconv"
	"strings"
)

func multiPhaseOutcomes(content string) map[int]string {
	phases := map[int]string{}
	for _, m := range multiPhaseRegex.FindAllStringSubmatch(content, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 {
			continue
		}
		if _, exists := phases[n]; !exists {
			phases[n] = strings.Trim(strings.TrimSpace(m[2]), `"':;., `)
		}
	}
	return phases
}

func approvedPhaseStart(plan inboxApprovalPlan) (int, string, bool) {
	phases := multiPhaseOutcomes(plan.Description)
	if len(phases) < 2 {
		return 0, "", false
	}
	phase := 1
	if m := multiPhaseRegex.FindStringSubmatch(plan.Title); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			phase = n
		}
	}
	outcome, ok := phases[phase]
	return phase, outcome, ok && outcome != ""
}

// The API title limit applies only to display text, never approved scope/evidence.
func phaseTaskTitle(phase int, outcome string) string {
	title := []rune(fmt.Sprintf("Phase %d: %s", phase, outcome))
	if len(title) > 200 {
		title = append(title[:199], '…')
	}
	return string(title)
}

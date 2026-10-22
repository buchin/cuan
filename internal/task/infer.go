package task

import "strings"

// Initial metadata is local and deterministic; the bound agent can refine it.
func inferPrompt(prompt string) (string, string) {
	text := strings.TrimSpace(prompt)
	line := strings.SplitN(text, "\n", 2)[0]
	line = strings.TrimSpace(strings.TrimLeft(line, "#* -\t"))
	if line == "" {
		line = text
	}
	runes := []rune(line)
	if len(runes) > 110 {
		line = string(runes[:107]) + "…"
	}
	priority := PriorityNormal
	words := " " + strings.ToLower(text) + " "
	for _, signal := range []string{"production down", "data loss", "security breach", "critical outage"} {
		if strings.Contains(words, signal) {
			return line, PriorityUrgent
		}
	}
	for _, signal := range []string{" urgent ", " asap ", "high priority", "blocking users"} {
		if strings.Contains(words, signal) {
			priority = PriorityHigh
		}
	}
	return line, priority
}

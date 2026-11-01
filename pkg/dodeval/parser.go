// Package dodeval provides an autonomous, zero-dependency Definition of Done (DoD)
// evaluator and markdown criteria parser.
package dodeval

import (
	"regexp"
	"strconv"
	"strings"
)

// Criterion represents a parsed requirement or checklist item.
type Criterion struct {
	ID           string `json:"id,omitempty"`
	Index        int    `json:"index"`
	Text         string `json:"text"`
	OriginalLine string `json:"original_line"`
	Checked      bool   `json:"checked"`
	Section      string `json:"section,omitempty"`
}

var (
	// Regex matching standard markdown checklist items:
	// - [ ] or - [x] or - [X]
	// * [ ] or * [x] or * [X]
	// 1. [ ] or 1. [x] or 1. [X]
	checklistRegex = regexp.MustCompile(`^\s*(?:[-*]|\d+\.)\s+\[([ xX])\]\s+(.+)$`)

	// Regex matching numbered criteria without checkbox:
	// 1. Phase 1: ...
	numberedCriteriaRegex = regexp.MustCompile(`^\s*(\d+)\.\s+(.+)$`)

	// Regex matching acceptance criteria / DoD headers
	sectionHeaderRegex = regexp.MustCompile(`(?i)^#{1,4}\s+(.*(?:acceptance\s+criteria|definition\s+of\s+done|dod\b|kriteria\s+penerimaan|rincian\s+tiap\s+fase).*)$`)
	anyHeaderRegex     = regexp.MustCompile(`^#{1,4}\s+.*$`)
)

// ExtractDoDSections scans markdown and extracts lines belonging to DoD or Acceptance Criteria sections.
// If no specific section header is found, the entire input is returned.
func ExtractDoDSections(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var captured []string
	inTargetSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if sectionHeaderRegex.MatchString(trimmed) {
			inTargetSection = true
			captured = append(captured, line)
			continue
		}
		if inTargetSection {
			// If we hit another major header that doesn't match DoD, stop capturing this section
			if anyHeaderRegex.MatchString(trimmed) && !sectionHeaderRegex.MatchString(trimmed) {
				inTargetSection = false
				continue
			}
			captured = append(captured, line)
		}
	}

	if len(captured) > 0 {
		return strings.Join(captured, "\n")
	}
	return markdown
}

// ParseChecklist parses all checklist and numbered criteria items from a markdown string.
func ParseChecklist(markdown string) []Criterion {
	lines := strings.Split(markdown, "\n")
	var items []Criterion
	currentSection := "General"
	index := 1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Detect section headers
		if strings.HasPrefix(trimmed, "#") {
			cleanHeader := strings.TrimLeft(trimmed, "#")
			currentSection = strings.TrimSpace(cleanHeader)
			continue
		}

		// 1. Match [ ] or [x] checklist items
		if matches := checklistRegex.FindStringSubmatch(trimmed); len(matches) == 3 {
			checked := strings.EqualFold(matches[1], "x")
			text := strings.TrimSpace(matches[2])
			items = append(items, Criterion{
				ID:           "crit_" + strconv.Itoa(index),
				Index:        index,
				Text:         text,
				OriginalLine: line,
				Checked:      checked,
				Section:      currentSection,
			})
			index++
			continue
		}

		// 2. Match numbered criteria lines (e.g. "1. Phase 1: ...")
		// Only capture if inside an acceptance criteria / DoD / Phase section or explicitly numbered
		if matches := numberedCriteriaRegex.FindStringSubmatch(trimmed); len(matches) == 3 {
			text := strings.TrimSpace(matches[2])
			// Avoid capturing standard list items in casual discussion unless in relevant section
			isRelevantSection := strings.Contains(strings.ToLower(currentSection), "acceptance") ||
				strings.Contains(strings.ToLower(currentSection), "done") ||
				strings.Contains(strings.ToLower(currentSection), "dod") ||
				strings.Contains(strings.ToLower(currentSection), "fase") ||
				strings.Contains(strings.ToLower(currentSection), "criteria")

			if isRelevantSection || strings.HasPrefix(strings.ToLower(text), "phase") || strings.HasPrefix(strings.ToLower(text), "fase") {
				items = append(items, Criterion{
					ID:           "crit_" + strconv.Itoa(index),
					Index:        index,
					Text:         text,
					OriginalLine: line,
					Checked:      false, // default unchecked unless proven by evidence
					Section:      currentSection,
				})
				index++
			}
		}
	}

	return items
}

// ParseAcceptanceCriteria extracts criteria prioritizing explicit acceptance criteria sections.
func ParseAcceptanceCriteria(markdown string) []Criterion {
	dodText := ExtractDoDSections(markdown)
	items := ParseChecklist(dodText)
	if len(items) == 0 && dodText != markdown {
		// Fallback to entire text if section extraction was too strict
		items = ParseChecklist(markdown)
	}
	return items
}

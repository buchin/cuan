package task

import (
	"regexp"
	"strings"
)

var (
	ansiEscapeRegex    = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~]|\].*?(?:\x07|\x1b\\)|[PX^_].*?\x1b\\)`)
	spinnerBulletRegex = regexp.MustCompile(`^[\s\x00-\x1f\r\x08\x7f\x{2800}-\x{28ff}\x{2022}\x{25cb}\x{25cf}\x{25aa}\x{25ab}\x{25e6}\*\-\+\>\$\:\;\.\|·⋮…]+`)
	noiseParenRegex    = regexp.MustCompile(`(?i)\(ctrl\+[a-z]\s+to\s+expand\)`)
)

// CleanTerminalStatusLine strips ANSI escape codes, terminal spinners (braille patterns),
// bullet markers, control characters, and keyboard hints from a terminal line.
func CleanTerminalStatusLine(line string) string {
	if line == "" {
		return ""
	}
	// 1. Remove ANSI escape sequences
	cleaned := ansiEscapeRegex.ReplaceAllString(line, "")
	// 2. Remove noise parentheticals like (ctrl+o to expand)
	cleaned = noiseParenRegex.ReplaceAllString(cleaned, "")
	// 3. Remove carriage returns, backspaces, and non-printable control chars
	cleaned = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\b' || (r < 32 && r != '\t') || r == 127 {
			return -1
		}
		return r
	}, cleaned)
	// 4. Strip spinner / braille and bullet / prompt symbols from start
	cleaned = spinnerBulletRegex.ReplaceAllString(cleaned, "")
	// 5. Trim whitespace
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

// IsTerminalNoiseLine checks if a cleaned line is just a divider or terminal footer/header noise.
func IsTerminalNoiseLine(line string) bool {
	if len(line) < 3 {
		return true
	}
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "esc to cancel") ||
		strings.HasPrefix(lower, "ctrl+") ||
		strings.Contains(lower, "gemini 3.") ||
		strings.Contains(lower, "claude 3.") ||
		strings.Contains(lower, "gpt-4") ||
		strings.HasPrefix(lower, "model:") ||
		strings.HasPrefix(lower, "token:") ||
		strings.HasPrefix(lower, "tokens:") {
		return true
	}
	isSep := true
	for _, r := range line {
		if r != '─' && r != '-' && r != '_' && r != '=' && r != '*' && r != '~' && r != ' ' {
			isSep = false
			break
		}
	}
	if isSep {
		return true
	}
	if line == ">" || line == "$" {
		return true
	}
	return false
}

// isToolActionSummary checks if a line represents a tool execution summary.
func isToolActionSummary(line string) bool {
	lower := strings.ToLower(line)
	prefixes := []string{
		"ran ", "running ", "thought for ", "checked ", "scheduled ",
		"scheduling", "reading ", "editing ", "writing ", "calling ",
		"fetching ", "executing ", "compiling ", "testing ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// isNarrativeStatus checks if a line is a narrative thought or descriptive status from the runner.
func isNarrativeStatus(line string) bool {
	if isToolActionSummary(line) {
		return false
	}
	words := strings.Fields(line)
	if len(words) >= 3 {
		return true
	}
	if strings.HasSuffix(line, "...") || strings.HasSuffix(line, ".") {
		return len(words) >= 2
	}
	return false
}

// ExtractRunnerLiveActivity extracts the most relevant recent narrative status or
// tool action summary from a slice of raw terminal/log lines.
func ExtractRunnerLiveActivity(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	var latestToolAction string
	var latestCleaned string

	// Iterate from newest line to oldest
	for i := len(lines) - 1; i >= 0; i-- {
		cleaned := CleanTerminalStatusLine(lines[i])
		if cleaned == "" || IsTerminalNoiseLine(cleaned) {
			continue
		}

		if latestCleaned == "" {
			latestCleaned = cleaned
		}

		// If it's a narrative status explanation, prioritize it immediately
		if isNarrativeStatus(cleaned) {
			if len(cleaned) > 140 {
				cleaned = cleaned[:137] + "..."
			}
			return cleaned
		}

		// Remember the newest tool action summary
		if latestToolAction == "" && isToolActionSummary(cleaned) {
			latestToolAction = cleaned
		}
	}

	if latestToolAction != "" {
		if len(latestToolAction) > 140 {
			latestToolAction = latestToolAction[:137] + "..."
		}
		return latestToolAction
	}

	if latestCleaned != "" {
		if len(latestCleaned) > 140 {
			latestCleaned = latestCleaned[:137] + "..."
		}
		return latestCleaned
	}

	return ""
}

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

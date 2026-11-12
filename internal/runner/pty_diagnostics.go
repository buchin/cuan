package runner

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/task"
)

const maxPTYDiagnosticFragment = 16 * 1024

// Quota phrases are at most 27 bytes. Keep a small independent overlap for
// split phrases/ANSI frames instead of rescanning 16 KiB on every spinner redraw.
const maxPTYQuotaOverlap = 256

var ptyCursorUp = regexp.MustCompile("\x1b\\[[0-9]*A")

// PTY transport frames are arbitrary bytes, not complete lines. This collector
// only affects persisted diagnostic logs; terminal viewers receive original bytes.
type ptyDiagnosticCollector struct {
	pending     string
	lastSpinner string
	quotaWindow string
	redrawText  string
	redrawRaw   string
	progress    bool
}

// animatedStatus reports whether a cleaned line is only a provider progress
// animation such as "Running command..." rather than new work output. An agent
// waiting on one long command redraws this continuously, so counting it as
// progress would hide a genuine stall.
func animatedStatus(clean string) bool {
	key := strings.ToLower(strings.TrimRight(clean, ". …"))
	if key == "" {
		return false
	}
	for _, status := range []string{"running command", "reading", "reading file", "thinking", "working", "managing tasks"} {
		if strings.HasPrefix(status, key) {
			return true
		}
	}
	return false
}

// Progressed reports whether the last Feed produced work output rather than
// only animation frames. Callers use it to tell a working agent from a stuck one.
func (c *ptyDiagnosticCollector) Progressed() bool { return c.progress }

// noteProgress records work output. keep() may return several diagnostic lines
// concatenated, so each one is classified separately.
func (c *ptyDiagnosticCollector) noteProgress(line string) {
	if c.progress {
		return
	}
	for _, part := range strings.Split(line, "\n") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if !animatedStatus(task.CleanTerminalStatusLine(part)) {
			c.progress = true
			return
		}
	}
}

func (c *ptyDiagnosticCollector) Feed(chunk []byte) ([]string, string) {
	c.progress = false
	c.pending += string(chunk)
	c.quotaWindow += string(chunk)
	probe := task.CleanTerminalStatusLine(c.quotaWindow)
	if len(c.quotaWindow) > maxPTYQuotaOverlap {
		c.quotaWindow = c.quotaWindow[len(c.quotaWindow)-maxPTYQuotaOverlap:]
	}
	quota := ""
	for _, signal := range []string{"Individual quota reached", "Resource has been exhausted", "Quota exceeded", "rate_limit_exceeded"} {
		if strings.Contains(probe, signal) {
			quota = probe
			break
		}
	}
	var out []string
	for {
		at := strings.IndexAny(c.pending, "\r\n")
		if at < 0 {
			if len(c.pending) <= maxPTYDiagnosticFragment {
				break
			}
			at = maxPTYDiagnosticFragment
			for at > 0 && !utf8.RuneStart(c.pending[at]) {
				at--
			}
			if at == 0 {
				at = maxPTYDiagnosticFragment
			}
			if line := c.keep(c.pending[:at]); line != "" {
				out = append(out, line)
			}
			c.pending = c.pending[at:]
			continue
		}
		if line := c.keep(c.pending[:at]); line != "" {
			out = append(out, line)
			c.noteProgress(line)
		}
		c.pending = c.pending[at+1:]
	}
	return out, quota
}
func (c *ptyDiagnosticCollector) Flush() []string {
	line := c.keep(c.pending)
	c.pending = ""
	if redraw := c.flushRedraw(); redraw != "" {
		if line != "" {
			line += "\n"
		}
		line += redraw
	}
	if line == "" {
		return nil
	}
	return []string{line}
}

func (c *ptyDiagnosticCollector) flushRedraw() string {
	raw := c.redrawRaw
	c.redrawText, c.redrawRaw = "", ""
	return raw
}

func (c *ptyDiagnosticCollector) keep(raw string) string {
	clean := task.CleanTerminalStatusLine(raw)
	if clean == "" {
		return ""
	}
	spinner := strings.IndexFunc(raw, func(r rune) bool { return r >= 0x2800 && r <= 0x28ff }) >= 0
	// AGY animates arbitrary progress sentences with cursor-up redraws, not
	// just fixed "Reading" labels. Retain the most complete version of a
	// prefix-related redraw group. Ordinary output, repeated facts and changes
	// of sentence flush it unchanged. Viewers still receive all original bytes.
	if spinner && ptyCursorUp.MatchString(raw) {
		if c.redrawText == "" || strings.HasPrefix(clean, c.redrawText) || strings.HasPrefix(c.redrawText, clean) {
			if len(clean) >= len(c.redrawText) {
				c.redrawText, c.redrawRaw = clean, raw
			}
			return ""
		}
		previous := c.flushRedraw()
		c.redrawText, c.redrawRaw = clean, raw
		return previous
	}
	previous := c.flushRedraw()
	line := c.keepStatus(raw, clean, spinner)
	if previous != "" && line != "" {
		return previous + "\n" + line
	}
	return previous + line
}

func (c *ptyDiagnosticCollector) keepStatus(raw, clean string, spinner bool) string {
	key := strings.ToLower(strings.TrimRight(clean, ". …"))
	if spinner && key != "" {
		// Real AGY redraws include one-, two- and three-letter prefixes.
		// Prefer the current state for ambiguous prefixes such as "R";
		// otherwise they reset deduplication on every animation cycle.
		if c.lastSpinner != "" && strings.HasPrefix(c.lastSpinner, key) {
			key = c.lastSpinner
		}
		for _, status := range []string{"running command", "reading", "reading file", "thinking", "working", "managing tasks"} {
			if strings.HasPrefix(status, key) {
				key = status
				// Reading and Reading file share one redraw state.
				if strings.HasPrefix(key, "reading") {
					key = "reading"
				}
				break
			}
		}
	}
	// Collapse only recognized animated status redraws. Identical factual lines,
	// commands and errors remain separate diagnostic records.
	if spinner && (key == "running command" || key == "reading" || key == "thinking" || key == "working" || key == "managing tasks") {
		if c.lastSpinner == key {
			return ""
		}
		c.lastSpinner = key
	} else {
		c.lastSpinner = ""
	}
	return raw
}

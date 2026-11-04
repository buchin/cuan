package runner

import (
	"strings"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/task"
)

const maxPTYDiagnosticFragment = 16 * 1024

// Quota phrases are at most 27 bytes. Keep a small independent overlap for
// split phrases/ANSI frames instead of rescanning 16 KiB on every spinner redraw.
const maxPTYQuotaOverlap = 256

// PTY transport frames are arbitrary bytes, not complete lines. This collector
// only affects persisted diagnostic logs; terminal viewers receive original bytes.
type ptyDiagnosticCollector struct {
	pending     string
	lastSpinner string
	quotaWindow string
}

func (c *ptyDiagnosticCollector) Feed(chunk []byte) ([]string, string) {
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
		}
		c.pending = c.pending[at+1:]
	}
	return out, quota
}
func (c *ptyDiagnosticCollector) Flush() []string {
	line := c.keep(c.pending)
	c.pending = ""
	if line == "" {
		return nil
	}
	return []string{line}
}
func (c *ptyDiagnosticCollector) keep(raw string) string {
	clean := task.CleanTerminalStatusLine(raw)
	if clean == "" {
		return ""
	}
	spinner := strings.IndexFunc(raw, func(r rune) bool { return r >= 0x2800 && r <= 0x28ff }) >= 0
	key := strings.ToLower(strings.TrimRight(clean, ". …"))
	if spinner && len(key) >= 4 {
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

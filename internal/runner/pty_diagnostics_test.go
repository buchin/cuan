package runner

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPTYDiagnosticsReassemblesArbitraryFrames(t *testing.T) {
	input := "\x1b[31mfatal: workspace unavailable\x1b[0m\r\nEvidence: café verified\n"
	for width := 1; width <= len(input); width++ {
		var c ptyDiagnosticCollector
		var lines []string
		for at := 0; at < len(input); at += width {
			end := at + width
			if end > len(input) {
				end = len(input)
			}
			out, _ := c.Feed([]byte(input[at:end]))
			lines = append(lines, out...)
		}
		lines = append(lines, c.Flush()...)
		if len(lines) != 2 || lines[0] != "\x1b[31mfatal: workspace unavailable\x1b[0m" || lines[1] != "Evidence: café verified" {
			t.Fatalf("width %d fragmented diagnostic: %q", width, lines)
		}
	}
}
func TestPTYDiagnosticsBoundsSpinnerNoiseWithoutDroppingErrors(t *testing.T) {
	var c ptyDiagnosticCollector
	var lines []string
	for i := 0; i < 10000; i++ {
		out, _ := c.Feed([]byte("\r⣯  \x1b[90mRunning command...\x1b[0m"))
		lines = append(lines, out...)
	}
	out, _ := c.Feed([]byte("\rERROR: build failed\nERROR: build failed\n"))
	lines = append(lines, out...)
	lines = append(lines, c.Flush()...)
	if len(lines) != 3 || lines[1] != "ERROR: build failed" || lines[2] != "ERROR: build failed" {
		t.Fatalf("spinner evicted factual evidence: %q", lines)
	}
}
func TestPTYDiagnosticsDetectsQuotaSplitAcrossFrames(t *testing.T) {
	for _, signal := range []string{
		"Individual quota reached",
		"Resource has been exhausted",
		"Quota exceeded",
		"rate_limit_exceeded",
		"RESOURCE_EXHAUSTED",
		"No capacity available",
		"capacity available for model",
		"high traffic right now",
		"UNAVAILABLE (code 503)",
	} {
		for split := 1; split < len(signal); split++ {
			var c ptyDiagnosticCollector
			if _, hit := c.Feed([]byte(signal[:split])); hit != "" {
				t.Fatal("detected incomplete quota signal")
			}
			if _, hit := c.Feed([]byte(signal[split:])); !strings.Contains(hit, signal) {
				t.Fatalf("missed quota split %d for %q", split, signal)
			}
		}
	}
}

func TestPTYDiagnosticsQuotaOverlapAfterSpinnerHistory(t *testing.T) {
	var c ptyDiagnosticCollector
	for i := 0; i < 1000; i++ {
		c.Feed([]byte("\r⣯  \x1b[90mRunning command...\x1b[0m"))
	}
	if len(c.quotaWindow) > maxPTYQuotaOverlap {
		t.Fatal("quota scan retained diagnostic-sized history")
	}
	for _, chunk := range []string{"\r\x1b[31", "mResource has ", "been exhausted\x1b[0m"} {
		_, hit := c.Feed([]byte(chunk))
		if strings.Contains(chunk, "exhausted") && !strings.Contains(hit, "Resource has been exhausted") {
			t.Fatal("split ANSI quota signal was lost after spinner history")
		}
	}
}
func TestPTYDiagnosticsBoundsUnterminatedOutputAndFlushesTail(t *testing.T) {
	var c ptyDiagnosticCollector
	input := strings.Repeat("x", 5*maxPTYDiagnosticFragment) + "final error without newline"
	out, _ := c.Feed([]byte(input))
	if len(c.pending) > maxPTYDiagnosticFragment {
		t.Fatal("unbounded partial log")
	}
	out = append(out, c.Flush()...)
	if strings.Join(out, "") != input {
		t.Fatal("unterminated output lost")
	}
	if len(c.Flush()) != 0 {
		t.Fatal("flush duplicated output")
	}
}

func TestPTYDiagnosticsKeepsUTF8ValidAtBoundedFragmentBoundary(t *testing.T) {
	var c ptyDiagnosticCollector
	input := strings.Repeat("界", 3*maxPTYDiagnosticFragment) + "end"
	lines, _ := c.Feed([]byte(input))
	lines = append(lines, c.Flush()...)
	for _, line := range lines {
		if !utf8.ValidString(line) {
			t.Fatal("valid UTF-8 split into invalid database log records")
		}
	}
	if strings.Join(lines, "") != input {
		t.Fatal("UTF-8 bytes changed")
	}
}

func TestPTYDiagnosticsCollapsesPartialStatusRedrawsButKeepsSpecificWork(t *testing.T) {
	var c ptyDiagnosticCollector
	var lines []string
	for _, status := range []string{"Running command", "Reading file", "Managing tasks"} {
		for cycle := 0; cycle < 1000; cycle++ {
			for length := 1; length <= len(status); length++ {
				out, _ := c.Feed([]byte("\r⣯  " + status[:length] + "\b\b\r"))
				lines = append(lines, out...)
			}
		}
	}
	for _, factual := range []string{"⣯ Reading failed: permission denied", "⣯ Running command npm test", "Reading", "Reading", "ERROR: failed", "ERROR: failed"} {
		out, _ := c.Feed([]byte(factual + "\n"))
		lines = append(lines, out...)
	}
	if len(lines) != 9 {
		t.Fatalf("partial redraws evicted evidence or factual lines collapsed: %d", len(lines))
	}
	if !strings.Contains(lines[3], "permission denied") || lines[7] != "ERROR: failed" || lines[8] != "ERROR: failed" {
		t.Fatalf("specific evidence lost: %q", lines)
	}
}

func TestPTYDiagnosticsShortRedrawsAcrossTransportFramesPreserveEvidence(t *testing.T) {
	input := "\r⣾ \x1b[94mRunning command\x1b[m\r"
	for i := 0; i < 20; i++ {
		for _, status := range []string{"R", "Ru", "Run", "Runn", "Running command"} {
			input += "\x1b[3A⣷ \x1b[94m" + status + "\x1b[m\r"
		}
	}
	input += "⣯ Running command npm test\nERROR: test failed\nERROR: test failed\n"
	for _, width := range []int{1, 7, 64, len(input)} {
		var c ptyDiagnosticCollector
		var lines []string
		for at := 0; at < len(input); at += width {
			end := min(at+width, len(input))
			out, _ := c.Feed([]byte(input[at:end]))
			lines = append(lines, out...)
		}
		lines = append(lines, c.Flush()...)
		if len(lines) != 4 || !strings.Contains(lines[1], "npm test") || lines[2] != "ERROR: test failed" || lines[3] != lines[2] {
			t.Fatalf("width %d: cosmetic redraws or lost factual evidence: %q", width, lines)
		}
	}
}

func TestPTYDiagnosticsRetainsCompleteAnimatedProgressAndRealOutput(t *testing.T) {
	full := "Analyzing workspace setup reveals repeated dependency installation"
	input := ""
	for cycle := 0; cycle < 100; cycle++ {
		for length := 1; length <= len(full); length++ {
			input += "\x1b[3A⣷ \x1b[94m" + full[:length] + "\x1b[m\r"
		}
	}
	input += "Ran npm test\nERROR: rejected\nERROR: rejected\n"
	for _, width := range []int{1, 73, len(input)} {
		var c ptyDiagnosticCollector
		var output []string
		for at := 0; at < len(input); at += width {
			lines, _ := c.Feed([]byte(input[at:min(at+width, len(input))]))
			output = append(output, lines...)
		}
		output = append(output, c.Flush()...)
		text := strings.Join(output, "\n")
		if strings.Count(text, full) != 1 || strings.Count(text, "Ran npm test") != 1 || strings.Count(text, "ERROR: rejected") != 2 || strings.Count(text, "\n") != 3 {
			t.Fatalf("width %d lost diagnostics or retained animation: %q", width, text)
		}
	}
}

func TestPTYDiagnosticsFlushesDistinctAnimatedStatementsAndQuota(t *testing.T) {
	var c ptyDiagnosticCollector
	first := "\x1b[3A⣷ Checking source commit\r"
	second := "\x1b[3A⣷ Testing dependency reuse\r"
	lines, _ := c.Feed([]byte(first + second))
	lines = append(lines, c.Flush()...)
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "Checking source commit") || !strings.Contains(text, "Testing dependency reuse") {
		t.Fatal("distinct progress statements lost", text)
	}
	if _, quota := c.Feed([]byte("\x1b[3A⣷ Resource has been exhausted\r")); !strings.Contains(quota, "Resource has been exhausted") {
		t.Fatal("buffering delayed quota detection")
	}
	if !strings.Contains(strings.Join(c.Flush(), "\n"), "Resource has been exhausted") {
		t.Fatal("quota diagnostic lost on final flush")
	}
}

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
	for _, signal := range []string{"Individual quota reached", "Resource has been exhausted", "Quota exceeded", "rate_limit_exceeded"} {
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

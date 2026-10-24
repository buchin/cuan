package task

import (
	"testing"
)

func TestCleanTerminalStatusLine(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ansi escape codes and spinner",
			input:    "\x1B[16D\x1B[?25h\x1B[?25l\r\x1B[3A⢿  \x1B[90mRunning command..\x1B[m",
			expected: "Running command..",
		},
		{
			name:     "bullet marker and ctrl hint",
			input:    "• Scheduled 1 timer (ctrl+o to expand)",
			expected: "Scheduled 1 timer",
		},
		{
			name:     "narrative text with spaces",
			input:    "Paket internal/server sedang dikompilasi... Menunggu proses linking biner akhir.",
			expected: "Paket internal/server sedang dikompilasi... Menunggu proses linking biner akhir.",
		},
		{
			name:     "braille spinner with tool action",
			input:    "⣾  \x1B[90mRunning command...\x1B[m",
			expected: "Running command...",
		},
		{
			name:     "thought indicator",
			input:    "○ Thought for 10.1s (Creating a test...)",
			expected: "Thought for 10.1s (Creating a test...)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CleanTerminalStatusLine(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestExtractRunnerLiveActivity(t *testing.T) {
	// Sample lines mimicking Image 2
	lines := []string{
		"• Scheduled 1 timer (ctrl+o to expand)",
		"Paket internal/server sedang dikompilasi... Menunggu proses linking biner akhir.",
		"• Ran 2 commands (ls -lh /opt/cuan/re...)",
		"• Scheduled 1 timer (ctrl+o to expand)",
		"Menjalankan unit test untuk internal/terminal...",
		"• Ran 1 command, checked 1 task",
		"• Scheduled 1 timer (ctrl+o to expand)",
		"Menjalankan pengujian untuk komponen store task...",
		"○ Thought for 10.1s (Creating a test...)",
		"• Checked 1 task (task-771)",
		"• Scheduled 1 timer (10s: Check task...)",
		"Ran (export PATH=\"/home/ubuntu/.loc...)",
		"• Scheduled 1 timer (ctrl+o to expand)",
		"  Scheduling...",
		"\x1B[90m────────────────────────────────────────────\x1B[m",
		"\x1B[90mesc to cancel\x1B[m",
	}

	got := ExtractRunnerLiveActivity(lines)
	// Should prioritize narrative status from the runner, e.g. "Menjalankan pengujian untuk komponen store task..."
	if got != "Menjalankan pengujian untuk komponen store task..." {
		t.Fatalf("expected narrative line 'Menjalankan pengujian untuk komponen store task...', got %q", got)
	}

	// If no narrative line, should fall back to latest tool action
	onlyToolLines := []string{
		"• Ran 2 commands (ls -lh /opt/cuan/re...)",
		"• Checked 1 task (task-771)",
		"• Scheduled 1 timer (10s: Check task...)",
	}
	gotTool := ExtractRunnerLiveActivity(onlyToolLines)
	if gotTool != "Scheduled 1 timer (10s: Check task...)" {
		t.Fatalf("expected 'Scheduled 1 timer (10s: Check task...)', got %q", gotTool)
	}
}

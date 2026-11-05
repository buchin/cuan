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

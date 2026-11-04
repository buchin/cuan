package server

import (
	"context"
	"strings"
)

// formatAttemptTerminalLogs retrieves the last N log lines for a task attempt
// to provide deep failure diagnostic context to the Scrum Master.
func (s *Server) formatAttemptTerminalLogs(ctx context.Context, attemptID string, maxLines int) string {
	if s == nil || s.taskSvc == nil || attemptID == "" {
		return ""
	}
	if maxLines <= 0 {
		maxLines = 50
	}
	if maxLines > 500 {
		maxLines = 500
	}
	logs, err := s.taskSvc.GetRecentAttemptLogs(ctx, attemptID, maxLines)
	if err != nil || len(logs) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, l := range logs {
		line := strings.TrimSpace(l.Line)
		if line != "" {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return strings.TrimSpace(sb.String())
}

func indentLines(text, prefix string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

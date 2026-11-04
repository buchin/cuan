package task

import "context"

// RecentLogStore reads the tail directly without walking an execution's history.
type RecentLogStore interface {
	GetRecentLogs(context.Context, string, int) ([]*LogRecord, error)
}

// Retain recent output rather than freezing diagnostics at the start of a run.
const maxRetainedLogBytes = 256 * 1024

func boundedLogLine(line string) string {
	if len(line)+1 > maxRetainedLogBytes {
		return "[Output line too long; not saved]"
	}
	return line
}

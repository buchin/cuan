package task

// Retain recent output rather than freezing diagnostics at the start of a run.
const maxRetainedLogBytes = 256 * 1024

func boundedLogLine(line string) string {
	if len(line)+1 > maxRetainedLogBytes {
		return "[Output line too long; not saved]"
	}
	return line
}

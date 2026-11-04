package diagnostics

import (
	"errors"
	"os"
)

// Task execution has its own reporting capability. Ambient owner credentials
// and the host database must never turn its output into a new owner command.
// This is a tool safety boundary, not isolation against arbitrary host code.
func boundTaskSession() bool {
	return os.Getenv("CUAN_TASK_CAPABILITY_FILE") != "" || os.Getenv("CUAN_TASK_CAPABILITY_JSON") != ""
}

func taskSessionOwnerWriteError() error {
	if boundTaskSession() {
		return errors.New("bound task sessions cannot act as Owner; use task report for progress/results through CUAN_TASK_EXECUTABLE, then let the Inbox coordinator report to the user")
	}
	return nil
}

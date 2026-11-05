package task

import "testing"

func TestExecutionTimeoutRequiresNativeFailureEvidence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    int
		message string
		status  string
		want    bool
	}{
		{"adaptive", 124, "Execution timed out after adaptive timeout (45m0s)", AttemptStatusFailed, true},
		{"hard", 1, "Task runner exceeded hard execution timeout (20m0s).", AttemptStatusFailed, true},
		{"generic exit", 124, "command failed", AttemptStatusFailed, false},
		{"wrong exit", 1, "Execution timed out after adaptive timeout (45m0s)", AttemptStatusFailed, false},
		{"successful", 124, "Execution timed out after adaptive timeout (45m0s)", AttemptStatusSucceeded, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &TaskAttempt{Status: tc.status, ExitCode: &tc.code, ErrorMessage: tc.message}
			if IsExecutionTimeout(a) != tc.want {
				t.Fatal("incorrect timeout classification")
			}
		})
	}
	if IsExecutionTimeout(nil) || IsExecutionTimeout(&TaskAttempt{Status: AttemptStatusFailed}) {
		t.Fatal("missing evidence accepted")
	}
}

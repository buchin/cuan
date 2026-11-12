package task

import "strings"

// AttemptFailureClass distinguishes why an attempt stopped. Only quality
// failures are worth retrying; prerequisites and gate defects repeat the same
// failure forever and must stop instead of consuming the attempt budget.
type AttemptFailureClass string

const (
	// FailureQuality means the work itself fell short and another attempt can
	// plausibly succeed with corrections.
	FailureQuality AttemptFailureClass = "quality"
	// FailurePrerequisite means required environment, tooling or setup is
	// missing. Repeating the same attempt cannot fix it.
	FailurePrerequisite AttemptFailureClass = "prerequisite"
	// FailureGate means an orchestration gate rejected an otherwise finished
	// attempt. It is a defect in decision-making, not in the delivered work.
	FailureGate AttemptFailureClass = "gate"
	// FailureUnknown is an unrecognised cause; treated conservatively.
	FailureUnknown AttemptFailureClass = "unknown"
)

// ClassifyAttemptFailure maps a recorded attempt error to its cause class.
// The classification is intentionally keyword-based over the messages the
// runner and server already persist; it governs retry policy, never acceptance.
func ClassifyAttemptFailure(message string) AttemptFailureClass {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return FailureUnknown
	}
	for _, term := range []string{
		"auto_deploy",
		"publication could not be independently verified",
		"verified production deployment receipt is required",
	} {
		if strings.Contains(lower, term) {
			return FailureGate
		}
	}
	for _, term := range []string{
		"phpunit target or runtime is unavailable",
		"phpunit target unavailable",
		"dependency preparation failed",
		"locked frontend dependency preparation failed",
		"command not found",
		"no such file or directory",
		"runner token",
		"verifier is unavailable",
		"saved task project context is unavailable",
		"prerequisite",
	} {
		if strings.Contains(lower, term) {
			return FailurePrerequisite
		}
	}
	for _, term := range []string{
		"project verification failed",
		"live result inspection project checks did not pass",
		"definition of done",
		"dod",
		"tests failed",
		"build failed",
		"acceptance",
	} {
		if strings.Contains(lower, term) {
			return FailureQuality
		}
	}
	return FailureUnknown
}

// RetryWorthy reports whether another attempt could plausibly change the
// outcome. Gate defects and missing prerequisites repeat identically.
func RetryWorthy(message string) bool {
	switch ClassifyAttemptFailure(message) {
	case FailureQuality, FailureUnknown:
		return true
	default:
		return false
	}
}

// RepeatedFailure reports whether the latest two attempts stopped with the same
// normalised reason. Repeating an identical reason means the loop is not
// learning; the caller should surface the cause instead of requeueing.
func RepeatedFailure(latest, previous *TaskAttempt) bool {
	if latest == nil || previous == nil {
		return false
	}
	current := normalizeFailure(latest.ErrorMessage)
	if current == "" {
		return false
	}
	return current == normalizeFailure(previous.ErrorMessage)
}

func normalizeFailure(message string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(message)), " "))
}

// businessAttempts counts attempts that represent real work and returns the two
// most recent of them. Canceled and infrastructure-only attempts (exit 137/143,
// lost leases, runner restarts) do not consume the budget, matching the rule
// the retry path has always used. Keeping this in one place prevents the
// attempt limit, the retry path and the DoD evaluator from disagreeing.
func businessAttempts(attempts []*TaskAttempt) (count int, latest, previous *TaskAttempt) {
	for _, a := range attempts {
		if a == nil || a.Status == AttemptStatusCanceled || a.CancelRequested || a.CanceledAt != nil {
			continue
		}
		if a.Status == AttemptStatusInterrupted {
			continue
		}
		if a.ExitCode != nil && (*a.ExitCode == 137 || *a.ExitCode == 143) {
			continue
		}
		errLower := strings.ToLower(a.ErrorMessage)
		infrastructure := false
		for _, term := range []string{"lease expired", "heartbeat lost", "runner killed", "runner restart", "connection lost", "interrupted"} {
			if strings.Contains(errLower, term) {
				infrastructure = true
				break
			}
		}
		if infrastructure {
			continue
		}
		count++
		if latest == nil || a.CreatedAt.After(latest.CreatedAt) {
			previous = latest
			latest = a
		} else if previous == nil || a.CreatedAt.After(previous.CreatedAt) {
			previous = a
		}
	}
	return count, latest, previous
}

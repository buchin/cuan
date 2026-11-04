package task

import (
	"strings"
	"testing"
	"time"
)

func TestVerificationRuntimeRequiresFreshWellFormedRenderEvidence(t *testing.T) {
	now := time.Now()
	for _, scenario := range []string{"valid", "stale", "future", "bad fingerprint", "no version", "nil"} {
		t.Run(scenario, func(t *testing.T) {
			evidence := &VerificationRuntime{Fingerprint: strings.Repeat("a", 64), Version: "1.64.1", CheckedAt: now}
			switch scenario {
			case "stale":
				evidence.CheckedAt = now.Add(-11 * time.Minute)
			case "future":
				evidence.CheckedAt = now.Add(time.Minute)
			case "bad fingerprint":
				evidence.Fingerprint = strings.Repeat("z", 64)
			case "no version":
				evidence.Version = ""
			case "nil":
				evidence = nil
			}
			if got := evidence.Fresh(now); got != (scenario == "valid") {
				t.Fatalf("fresh=%v", got)
			}
		})
	}
}

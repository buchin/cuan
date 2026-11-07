package server

import (
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestPreparedSourceInspectionRequiresNativeHistoryAndCurrentContainment(t *testing.T) {
	source := strings.Repeat("a", 40)
	for _, scenario := range []string{"valid", "no history", "wrong source", "no observation", "unverified", "not required", "uncontained", "unknown containment", "different result", "missing release"} {
		t.Run(scenario, func(t *testing.T) {
			prepared := &task.NativeReleasePreparation{SourceCommit: source}
			contained := true
			observed := &reviewPublication{Required: true, Verified: true, Commit: source, ObservedRelease: strings.Repeat("b", 40), ResultCommitContained: &contained}
			switch scenario {
			case "no history":
				prepared = nil
			case "wrong source":
				prepared.SourceCommit = strings.Repeat("c", 40)
			case "no observation":
				observed = nil
			case "unverified":
				observed.Verified = false
			case "not required":
				observed.Required = false
			case "uncontained":
				contained = false
			case "unknown containment":
				observed.ResultCommitContained = nil
			case "different result":
				observed.Commit = strings.Repeat("c", 40)
			case "missing release":
				observed.ObservedRelease = ""
			}
			if preparedSourceMatchesPublication(prepared, source, observed) != (scenario == "valid") {
				t.Fatal("wrong current-outcome inspection admission", scenario)
			}
		})
	}
}

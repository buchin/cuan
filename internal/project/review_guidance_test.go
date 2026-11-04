package project

import (
	"strings"
	"testing"
)

func TestTaskReviewGuidanceSnapshotFormats(t *testing.T) {
	for _, marker := range []string{snapshotMarker, legacySnapshotMarker} {
		original := "Approved scope" + marker + `{"goal":"Original goal","instructions":"Original rules","environment":"PRIVATE=value","deploy_branch":"private-branch"}` + "\n```"
		goal, guidance, err := TaskReviewGuidance(original)
		if err != nil || goal != "Original goal" || guidance != "Original rules" {
			t.Fatalf("lost historical context: %q %q %v", goal, guidance, err)
		}
		if strings.Contains(goal+guidance, "PRIVATE") {
			t.Fatal("execution environment exposed")
		}
	}
	goal, guidance, err := TaskReviewGuidance("Legacy scope without snapshot")
	if err != nil || goal != "" || guidance != "" {
		t.Fatal("invented project guidance")
	}
}

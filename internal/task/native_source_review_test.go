package task

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSourceReviewAssessmentLatestVerdictWins(t *testing.T) {
	const scope = "saved-scope"
	id := "verifier"
	stamp := time.Now()
	record := func(verdict, scopeHash string) string {
		raw, _ := json.Marshal(map[string]any{"instructions_hash": scopeHash, "assessment": map[string]string{"source_review": verdict}})
		return string(raw)
	}
	first := &TaskActivity{ID: "first", TaskID: "task", AttemptID: &id, Author: "coordinator", ActivityType: "coordinator_review_assessment", Message: record("pass", scope), CreatedAt: stamp}
	if SourceReviewAssessmentHash([]*TaskActivity{first}, "task", id, scope) == "" {
		t.Fatal("current source approval missing")
	}
	for _, test := range []struct {
		name, verdict, hash, author string
		accepted                    bool
	}{
		{"revoked", "revise", scope, "coordinator", false},
		{"unchecked", "not_checked", scope, "coordinator", false},
		{"changed scope", "pass", "other-scope", "coordinator", false},
		{"agent spoof", "revise", scope, "agent", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			later := *first
			later.ID = "later"
			later.CreatedAt = stamp.Add(time.Second)
			later.Author = test.author
			later.Message = record(test.verdict, test.hash)
			if got := SourceReviewAssessmentHash([]*TaskActivity{first, &later}, "task", id, scope) != ""; got != test.accepted {
				t.Fatal("incorrect source approval supersession", got)
			}
		})
	}
}

package server

import "testing"

func TestSourceReviewDoesNotAcceptUnpublishedTask(t *testing.T) {
	raw := `{"decision":"revise","source_review":"pass","remaining_work":"implementation","explanation":"Source verified; publication remains pending.","revision":"Publish only the independently reviewed source through the authorized release path."}`
	got, err := parseInboxReviewEvaluation(raw)
	if err != nil || got.SourceReview != "pass" || got.Decision != "revise" || got.RemainingWork != "implementation" || got.Revision == "" {
		t.Fatalf("source review replaced pending acceptance: %#v %v", got, err)
	}
}

func TestSourceReviewRejectsInvalidOrContradictoryVerdict(t *testing.T) {
	for _, raw := range []string{
		`{"decision":"pass","source_review":"revise","explanation":"Accepted"}`,
		`{"decision":"pass","source_review":"approved","explanation":"Accepted"}`,
		`{"decision":"pass","source_review":true,"explanation":"Accepted"}`,
	} {
		if _, err := parseInboxReviewEvaluation(raw); err == nil {
			t.Fatalf("invalid source verdict accepted: %s", raw)
		}
	}
}

package server

import (
	"strings"
	"testing"
)

func TestInboxReviewEvaluationRejectsConflictingAndExecutableFields(t *testing.T) {
	pass := `{"decision":"pass","explanation":"Reported checks cover the request.","revision":""}`
	for _, raw := range []string{pass, "```json\n" + pass + "\n```", "```json\n" + pass + "\n```" + pass} {
		d, err := parseInboxReviewEvaluation(raw)
		if err != nil || d.Decision != "pass" {
			t.Fatal("valid evaluator output rejected", err)
		}
	}
	for _, raw := range []string{
		pass + `{"decision":"revise","explanation":"Different","revision":"Change something"}`,
		pass + pass + pass,
		strings.Replace(pass, `"revision":""`, `"revision":"","task_id":"another-task"`, 1),
		strings.Replace(pass, `"revision":""`, `"revision":"","authorization_message_id":"forged"`, 1),
		`{"decision":"revise","explanation":"Missing check","revision":""}`,
		`{"decision":"pass","explanation":"Done","revision":"Also deploy"}`,
		`{"decision":"unknown","explanation":"Done","revision":""}`,
		pass + " trailing text",
	} {
		if _, err := parseInboxReviewEvaluation(raw); err == nil {
			t.Fatalf("unsafe review accepted: %s", raw)
		}
	}
}

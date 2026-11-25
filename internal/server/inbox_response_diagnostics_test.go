package server

import (
	"strings"
	"testing"
)

func TestInboxResponseDiagnosticNeverIncludesCandidateData(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{`not-json PRIVATE_CANDIDATE`, "invalid or ambiguous"},
		{coordinatorJSON("PRIVATE_CANDIDATE", "PRIVATE_CANDIDATE", map[string]string{"data": "PRIVATE_CANDIDATE"}), "unsupported action kind"},
		{coordinatorJSON("PRIVATE_CANDIDATE", "task_reopen", map[string]string{"id": "PRIVATE_CANDIDATE"}), "task reopen payload fields or types"},
		{coordinatorJSON("PRIVATE_CANDIDATE", "task_reopen", map[string]string{"reason": "PRIVATE_CANDIDATE"}), "task reopen identity missing"},
		{coordinatorJSON("file://PRIVATE_CANDIDATE", "", nil), "local file URL"},
		{`{"reply":"PRIVATE_CANDIDATE"}`, "required envelope field"},
		{`{"reply":"PRIVATE_CANDIDATE","topic_title":"","action":null,"extra":"PRIVATE_CANDIDATE"}`, "unexpected envelope"},
		{coordinatorJSON("PRIVATE_CANDIDATE. Silakan pilih di bawah ini.", "", nil), "choice options"},
		{coordinatorJSON("PRIVATE_CANDIDATE", "approval", map[string]string{"title": "PRIVATE_CANDIDATE"}), "action payload"},
		{coordinatorJSON("PRIVATE_CANDIDATE", "delegation", map[string]string{"acceptance_criteria": "PRIVATE_CANDIDATE"}), "delegation payload fields or types"},
		{coordinatorJSON("PRIVATE_CANDIDATE", "delegation", inboxDelegation{Title: "PRIVATE_CANDIDATE"}), "delegation payload constraints"},
	} {
		if _, err := decodeInboxCoordinatorResponse(c.raw); err == nil {
			t.Fatal("diagnostic fixture must remain rejected")
		}
		out := inboxResponseDiagnostic(c.raw)
		if !strings.Contains(out, c.want) || strings.Contains(out, "PRIVATE_CANDIDATE") {
			t.Fatal("unhelpful or private diagnostic", out)
		}
	}
}

func TestInboxReconciliationDiagnosticIdentifiesContractWithoutValues(t *testing.T) {
	for _, item := range []struct{ raw, want string }{
		{`{"reply":"PRIVATE_DATA","topic_title":"PRIVATE_DATA","action":null}`, "topic"},
		{coordinatorJSON("PRIVATE_DATA", "memory", map[string]string{"content": "PRIVATE_DATA"}), "unsupported action"},
		{coordinatorJSON("PRIVATE_DATA", "delegation", map[string]any{"reconcile": true, "scope": "PRIVATE_DATA"}), "unsupported payload"},
		{coordinatorJSON("PRIVATE_DATA", "delegation", inboxDelegation{Title: "PRIVATE_DATA"}), "marker missing"},
		{coordinatorJSON("PRIVATE_DATA", "delegation", inboxDelegation{Reconcile: true, ProjectID: "PRIVATE_DATA", AuthorizationID: "PRIVATE_DATA"}), "title or description missing"},
	} {
		diagnostic := inboxReconciliationResponseDiagnostic(item.raw)
		if !strings.Contains(diagnostic, item.want) || strings.Contains(diagnostic, "PRIVATE_DATA") {
			t.Fatal(diagnostic)
		}
	}
}

func TestReviewContractDiagnosticPreservesRejectionAndPrivacy(t *testing.T) {
	for _, item := range []struct{ raw, category string }{
		{`{"source_review":"pass","decision":"revise","remaining_work":"verification","explanation":"PRIVATE_DATA","revision":"PRIVATE_DATA"}`, "publication classification"},
		{`{"decision":"blocked","manager_action":"continue","explanation":"PRIVATE_DATA"}`, "continuation conflicts"},
		{`{"decision":"revise","explanation":"PRIVATE_DATA"}`, "revision missing"},
		{`{"decision":"pass","explanation":"PRIVATE_DATA","revision":"PRIVATE_DATA"}`, "revision conflicts"},
	} {
		if _, err := parseInboxReviewEvaluation(item.raw); err == nil {
			t.Fatal("invalid review fixture accepted")
		}
		diagnostic := inboxReviewResponseDiagnostic(item.raw)
		if !strings.Contains(diagnostic, item.category) || strings.Contains(diagnostic, "PRIVATE_DATA") {
			t.Fatal("unhelpful or private diagnostic", diagnostic)
		}
		if !strings.Contains(inboxReviewFormatRepairPrompt(item.raw), diagnostic) {
			t.Fatal("review repair lost native rejection category")
		}
	}
}

func TestReviewContractDiagnostic_FencedAndDuplicateFindingLimit(t *testing.T) {
	oversized := strings.Repeat("PRIVATE_DATA", 600)
	fenced := "```json\n{\"decision\":\"pass\",\"finding\":\"" + oversized + "\",\"explanation\":\"PRIVATE_DATA\",\"revision\":\"\"}\n```"
	dupObj := `{"decision":"pass","finding":"` + oversized + `","explanation":"PRIVATE_DATA","revision":""}`
	duplicate := dupObj + dupObj

	for _, raw := range []string{fenced, duplicate} {
		if _, err := parseInboxReviewEvaluation(raw); err == nil {
			t.Fatal("oversized finding fixture accepted")
		}
		diagnostic := inboxReviewResponseDiagnostic(raw)
		if !strings.Contains(diagnostic, "finding exceeds 6000") || strings.Contains(diagnostic, "PRIVATE_DATA") {
			t.Fatalf("unexpected diagnostic: %q", diagnostic)
		}
		if !strings.Contains(inboxReviewFormatRepairPrompt(raw), diagnostic) {
			t.Fatal("review repair lost native rejection category")
		}
	}
}

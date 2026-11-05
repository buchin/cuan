package server

import (
	"strings"
	"testing"
)

func TestInboxResponseDiagnosticNeverIncludesCandidateData(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{`not-json PRIVATE_CANDIDATE`, "invalid or ambiguous"},
		{`{"reply":"PRIVATE_CANDIDATE"}`, "required envelope field"},
		{`{"reply":"PRIVATE_CANDIDATE","topic_title":"","action":null,"extra":"PRIVATE_CANDIDATE"}`, "unexpected envelope"},
		{coordinatorJSON("PRIVATE_CANDIDATE. Silakan pilih di bawah ini.", "", nil), "choice options"},
		{coordinatorJSON("PRIVATE_CANDIDATE", "approval", map[string]string{"title": "PRIVATE_CANDIDATE"}), "action payload"},
	} {
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

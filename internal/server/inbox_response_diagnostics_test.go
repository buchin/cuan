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

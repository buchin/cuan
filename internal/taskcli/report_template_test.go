package taskcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReportTemplatePreservesCriteriaWithoutCopyingClaimsOrSecrets(t *testing.T) {
	gets := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets++
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer private-token" {
			t.Error("template used mutation or wrong capability")
		}
		w.Write([]byte(`{"id":"bound","latest_attempt":{"id":"attempt"},"instructions":"PRIVATE_CONTEXT","summary":"OLD_SUCCESS","environment":"PRIVATE_SECRET","dod_checklist":[{"id":"original-2","text":"Keep exact \"quotes\" 文","completed":true,"evidence":"OLD_EVIDENCE"},{"id":"original-1","text":"Next criterion","completed":true,"evidence":"OLD_EVIDENCE"}]}`))
	}))
	defer api.Close()
	capability, _ := json.Marshal(Capability{API: api.URL, Token: "private-token", TaskID: "bound", AttemptID: "attempt"})
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(capability))
	var out, errs bytes.Buffer
	if code := Run([]string{"report-template"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	var report struct {
		State, Summary string
		CompleteNote   string `json:"complete_note"`
		Criteria       []struct {
			ID, Text, Evidence string
			Completed          bool
		} `json:"dod_checklist"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if gets != 1 || report.State != "review" || report.Summary != "" || report.CompleteNote != "" || len(report.Criteria) != 2 || report.Criteria[0].ID != "original-2" || report.Criteria[0].Text != "Keep exact \"quotes\" 文" || report.Criteria[1].ID != "original-1" {
		t.Fatal("original criteria or reporting defaults changed", out.String())
	}
	for _, criterion := range report.Criteria {
		if criterion.Completed || criterion.Evidence != "" {
			t.Fatal("template fabricated current evidence")
		}
	}
	for _, private := range []string{"private-token", "PRIVATE_CONTEXT", "PRIVATE_SECRET", "OLD_SUCCESS", "OLD_EVIDENCE"} {
		if strings.Contains(out.String(), private) {
			t.Fatal("private/history context copied", private)
		}
	}
}

func TestReportTemplateRejectsMisboundOrMalformedContext(t *testing.T) {
	for _, body := range []string{`{"id":"other","latest_attempt":{"id":"attempt"}}`, `{"id":"bound","latest_attempt":{"id":"older"}}`, `{"id":"bound"}`, `{"id":"bound","latest_attempt":{"id":"attempt"},"dod_checklist":[{"text":""}]}`, `not json`} {
		t.Run(body, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer api.Close()
			capability, _ := json.Marshal(Capability{API: api.URL, Token: "private-token", TaskID: "bound", AttemptID: "attempt"})
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(capability))
			var out, errs bytes.Buffer
			if code := Run([]string{"report-template"}, &out, &errs); code == 0 || out.Len() != 0 {
				t.Fatal("invalid current context produced a template", out.String())
			}
		})
	}
}

package dodeval

import (
	"strings"
	"testing"
)

func TestParseChecklist_MarkdownStyles(t *testing.T) {
	input := `
# Task Sprint Backlog

Here is some introductory context.

## Acceptance Criteria
- [ ] Implement pkg/taskfsm State Machine Engine
- [x] Write comprehensive unit tests
* [ ] Parser markdown checklist & criteria acceptance
* [X] Zero external dependencies
1. [ ] Clean import ke runner Cuan
2. [x] Live production deployment & smoke test (/healthz)
`
	items := ParseChecklist(input)
	if len(items) != 6 {
		t.Fatalf("expected 6 parsed criteria items, got %d", len(items))
	}

	// 1. - [ ] Implement pkg/taskfsm State Machine Engine
	if items[0].Checked || !strings.Contains(items[0].Text, "pkg/taskfsm") {
		t.Errorf("item[0] mismatch: %+v", items[0])
	}

	// 2. - [x] Write comprehensive unit tests
	if !items[1].Checked || !strings.Contains(items[1].Text, "unit tests") {
		t.Errorf("item[1] mismatch: %+v", items[1])
	}

	// 3. * [ ] Parser markdown checklist
	if items[2].Checked || !strings.Contains(items[2].Text, "Parser markdown") {
		t.Errorf("item[2] mismatch: %+v", items[2])
	}

	// 4. * [X] Zero external dependencies
	if !items[3].Checked || !strings.Contains(items[3].Text, "Zero external") {
		t.Errorf("item[3] mismatch: %+v", items[3])
	}

	// 5. 1. [ ] Clean import
	if items[4].Checked || !strings.Contains(items[4].Text, "Clean import") {
		t.Errorf("item[4] mismatch: %+v", items[4])
	}

	// 6. 2. [x] Live production deployment
	if !items[5].Checked || !strings.Contains(items[5].Text, "Live production") {
		t.Errorf("item[5] mismatch: %+v", items[5])
	}
}

func TestExtractDoDSections_HeaderDetection(t *testing.T) {
	input := `
# Title of Task

Some background explanations that shouldn't be parsed as DoD.
1. Just a random note
2. Another note

## Acceptance Criteria
- [ ] Phase 1: pkg/taskfsm
- [ ] Phase 2: pkg/dodeval
- [ ] Phase 3: Integrasi & Live Deployment

## Out of Scope
- Large database migration
`

	criteria := ParseAcceptanceCriteria(input)
	if len(criteria) != 3 {
		t.Fatalf("expected 3 acceptance criteria items, got %d", len(criteria))
	}

	expectedPrefixes := []string{"Phase 1:", "Phase 2:", "Phase 3:"}
	for i, prefix := range expectedPrefixes {
		if !strings.HasPrefix(criteria[i].Text, prefix) {
			t.Errorf("item[%d] text %q does not start with %q", i, criteria[i].Text, prefix)
		}
	}
}

func TestTokenExtractionAndStopWords(t *testing.T) {
	text := "Phase 1: pkg/taskfsm (State Machine Engine) untuk pengelolaan siklus"
	tokens := ExtractSignificantTokens(text)

	// "phase", "1", "untuk" should be filtered; "pkg/taskfsm", "state", "machine", "engine", "pengelolaan", "siklus" retained
	foundTaskFSM := false
	foundEngine := false
	for _, tok := range tokens {
		if tok == "taskfsm" || tok == "pkg" {
			foundTaskFSM = true
		}
		if tok == "engine" {
			foundEngine = true
		}
		if tok == "untuk" || tok == "phase" {
			t.Errorf("stop word %q should have been filtered out", tok)
		}
	}

	if !foundTaskFSM || !foundEngine {
		t.Errorf("expected significant tokens taskfsm/engine, got: %v", tokens)
	}
}

func TestAnalyzeGaps_AllCriteriaMet(t *testing.T) {
	criteria := []Criterion{
		{Text: "Phase 1: pkg/taskfsm State Machine Engine"},
		{Text: "Phase 2: pkg/dodeval Definition of Done Evaluator"},
		{Text: "Phase 3: Integrasi Cuan & Live Deployment smoke test"},
	}

	evidence := Evidence{
		Summary: "Implementasi taskfsm, dodeval, dan integrasi runner live deployment sukses",
		CompleteNote: "Phase 1: pkg/taskfsm state machine engine 100% lulus test. " +
			"Phase 2: pkg/dodeval Definition of Done Evaluator gap analyzer tuntas. " +
			"Phase 3: Integrasi runner Cuan clean import & live deployment smoke test /healthz berhasil.",
		CleanWorkingTree:  true,
		TargetedTestsPass: true,
		BuildZeroError:    true,
	}

	res := AnalyzeGaps(criteria, evidence)
	if !res.Passed {
		t.Fatalf("expected evaluation to pass, got gaps: %+v", res.Gaps)
	}
	if res.MetCriteria != 3 || res.UnmetCriteria != 0 {
		t.Errorf("met: %d, unmet: %d, expected 3/0", res.MetCriteria, res.UnmetCriteria)
	}
	if res.ComplianceRate != 100.0 {
		t.Errorf("expected 100.0%% compliance, got %.1f%%", res.ComplianceRate)
	}
	if !strings.Contains(res.Details, "LULUS (100%") {
		t.Errorf("details string missing passed header: %s", res.Details)
	}
}

func TestAnalyzeGaps_MissingEvidenceGap(t *testing.T) {
	criteria := []Criterion{
		{Text: "Phase 1: pkg/taskfsm State Machine Engine"},
		{Text: "Phase 2: pkg/dodeval Definition of Done Evaluator"},
		{Text: "Phase 3: Integrasi Cuan & Live Deployment smoke test"},
	}

	// Evidence only reports Phase 1
	evidence := Evidence{
		Summary:      "Phase 1 selesai",
		CompleteNote: "Implementasi pkg/taskfsm state machine engine selesai.",
	}

	res := AnalyzeGaps(criteria, evidence)
	if res.Passed {
		t.Fatal("expected evaluation to fail due to missing Phase 2 & Phase 3 evidence")
	}
	if res.MetCriteria != 1 {
		t.Errorf("expected 1 met criterion, got %d", res.MetCriteria)
	}
	if res.UnmetCriteria != 2 {
		t.Errorf("expected 2 unmet criteria, got %d", res.UnmetCriteria)
	}
	if len(res.Gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(res.Gaps))
	}
	if res.Gaps[0].Type != GapMissingEvidence {
		t.Errorf("expected GapMissingEvidence, got %s", res.Gaps[0].Type)
	}
	if !strings.Contains(res.Details, "BELUM LULUS") {
		t.Errorf("expected details to report failure status: %s", res.Details)
	}
}

func TestAnalyzeGaps_ContradictionFailureReported(t *testing.T) {
	criteria := []Criterion{
		{Text: "Phase 1: pkg/taskfsm State Machine"},
	}

	evidence := Evidence{
		Summary:      "Percobaan pengerjaan taskfsm",
		CompleteNote: "taskfsm failed due to syntax error in state machine",
	}

	res := AnalyzeGaps(criteria, evidence)
	if res.Passed {
		t.Fatal("expected failure due to explicit contradiction in evidence")
	}
	if len(res.Gaps) != 1 || res.Gaps[0].Type != GapFailedEvidence {
		t.Errorf("expected GapFailedEvidence, got %+v", res.Gaps)
	}
}

func TestAnalyzeGaps_PreCheckedChecklistItems(t *testing.T) {
	criteria := []Criterion{
		{Text: "Pre-verified requirement", Checked: true},
		{Text: "Unchecked requirement needing evidence", Checked: false},
	}

	evidence := Evidence{
		Summary:      "Work completed",
		CompleteNote: "Unchecked requirement needing evidence verified successfully.",
	}

	res := AnalyzeGaps(criteria, evidence)
	if !res.Passed {
		t.Fatalf("expected all to pass, got gaps: %+v", res.Gaps)
	}
	if res.MetCriteria != 2 {
		t.Errorf("expected 2 met criteria, got %d", res.MetCriteria)
	}
}

func TestEvaluate_EndToEnd(t *testing.T) {
	instructions := `
# Sprint Plan: Ekstraksi Modul Task Orchestration

## Acceptance Criteria
- [ ] Phase 1: pkg/taskfsm
- [ ] Phase 2: pkg/dodeval
- [ ] Phase 3: Integrasi & Live Deployment
`

	evidence := Evidence{
		Summary: "Ekstraksi taskfsm dan dodeval serta integrasi live deployment selesai",
		CompleteNote: "1. Phase 1: pkg/taskfsm unit tests pass\n" +
			"2. Phase 2: pkg/dodeval parser & gap analyzer lulus\n" +
			"3. Phase 3: Integrasi & live deployment smoke test terverifikasi",
	}

	res := Evaluate(instructions, evidence)
	if !res.Passed {
		t.Fatalf("expected Evaluate to pass 100%%, got: %+v", res)
	}
	if res.ComplianceRate != 100.0 {
		t.Errorf("expected 100%% compliance, got %.1f%%", res.ComplianceRate)
	}
}

func TestEvaluate_EmptyCriteria(t *testing.T) {
	res := Evaluate("Casual instructions without criteria", Evidence{Summary: "Done"})
	if !res.Passed || res.ComplianceRate != 100.0 {
		t.Errorf("expected pass for empty criteria, got %+v", res)
	}
}

func BenchmarkDoDEval_Evaluate(b *testing.B) {
	instructions := `
## Acceptance Criteria
- [ ] Phase 1: pkg/taskfsm
- [ ] Phase 2: pkg/dodeval
- [ ] Phase 3: Integrasi & Live Deployment
`
	evidence := Evidence{
		Summary: "Ekstraksi taskfsm dan dodeval serta integrasi live deployment selesai",
		CompleteNote: "Phase 1: pkg/taskfsm lulus. Phase 2: pkg/dodeval lulus. Phase 3: Integrasi & live deployment lulus.",
	}

	for i := 0; i < b.N; i++ {
		_ = Evaluate(instructions, evidence)
	}
}

package dodeval

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// GapType classifies why a criterion was not fulfilled.
type GapType string

const (
	// GapUnchecked indicates a checklist item was not marked completed and no evidence was provided.
	GapUnchecked GapType = "unchecked_checklist"

	// GapMissingEvidence indicates criterion has no corresponding verification evidence.
	GapMissingEvidence GapType = "missing_evidence"

	// GapFailedEvidence indicates evidence explicitly describes failure or unfulfilled state.
	GapFailedEvidence GapType = "failed_evidence"
)

// GapItem represents an unmet requirement identified during evaluation.
type GapItem struct {
	Criterion Criterion `json:"criterion"`
	Type      GapType   `json:"type"`
	Reason    string    `json:"reason"`
}

// Evidence represents all verification reports and artifacts provided for DoD evaluation.
type Evidence struct {
	Summary           string   `json:"summary"`
	CompleteNote      string   `json:"complete_note"`
	TestOutput        string   `json:"test_output,omitempty"`
	CommitSHA         string   `json:"commit_sha,omitempty"`
	Artifacts         []string `json:"artifacts,omitempty"`
	CleanWorkingTree  bool     `json:"clean_working_tree"`
	TargetedTestsPass bool     `json:"targeted_tests_pass"`
	BuildZeroError    bool     `json:"build_zero_error"`
	RawEvidence       string   `json:"raw_evidence,omitempty"`
}

// AnalysisResult holds the objective verification outcome.
type AnalysisResult struct {
	Passed         bool      `json:"passed"`
	TotalCriteria  int       `json:"total_criteria"`
	MetCriteria    int       `json:"met_criteria"`
	UnmetCriteria  int       `json:"unmet_criteria"`
	ComplianceRate float64   `json:"compliance_rate"`
	Gaps           []GapItem `json:"gaps,omitempty"`
	Details        string    `json:"details"`
}

var tokenSplitter = regexp.MustCompile(`[\s,;:|()\[\]{}"'/\\]+`)

// stopWords to filter out when extracting significant keywords from criteria.
var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true,
	"by": true, "for": true, "from": true, "has": true, "he": true, "in": true, "is": true,
	"it": true, "its": true, "of": true, "on": true, "that": true, "the": true, "to": true,
	"was": true, "were": true, "will": true, "with": true,
	// Indonesian stop words
	"dan": true, "di": true, "ke": true, "dari": true, "ini": true, "itu": true, "untuk": true,
	"yang": true, "pada": true, "adalah": true, "dengan": true, "atau": true, "tiap": true,
	"fase": true, "phase": true, "task": true,
}

var failureKeywords = []string{
	"failed", "failure", "gagal", "error", "unresolved", "incomplete", "belum selesai",
	"tidak lulus", "timeout", "broken", "panic", "crash",
}

// ExtractSignificantTokens extracts core identifier tokens from a criterion text.
func ExtractSignificantTokens(text string) []string {
	rawTokens := tokenSplitter.Split(strings.ToLower(text), -1)
	var tokens []string
	seen := make(map[string]bool)

	for _, t := range rawTokens {
		t = strings.TrimSpace(t)
		if len(t) < 2 {
			continue
		}
		if stopWords[t] {
			continue
		}
		if !seen[t] {
			seen[t] = true
			tokens = append(tokens, t)
		}
	}
	return tokens
}

// CheckContradiction reports whether the evidence text contains explicit failure indicators related to keywords.
func CheckContradiction(evidenceText string, tokens []string) (bool, string) {
	lowerEvidence := strings.ToLower(evidenceText)

	// Look for failure phrases near or mentioning these tokens
	for _, failWord := range failureKeywords {
		if strings.Contains(lowerEvidence, failWord) {
			// Check if failure is associated with any of our tokens
			for _, tok := range tokens {
				// If evidence mentions e.g. "taskfsm failed" or "build error"
				pattern1 := failWord + " " + tok
				pattern2 := tok + " " + failWord
				if strings.Contains(lowerEvidence, pattern1) || strings.Contains(lowerEvidence, pattern2) {
					return true, fmt.Sprintf("Bukti verifikasi melaporkan kegagalan eksplisit: %s %s", tok, failWord)
				}
			}
		}
	}
	return false, ""
}

// AnalyzeGaps evaluates a set of criteria against verification evidence.
func AnalyzeGaps(criteria []Criterion, evidence Evidence) *AnalysisResult {
	if len(criteria) == 0 {
		return &AnalysisResult{
			Passed:         true,
			TotalCriteria:  0,
			MetCriteria:    0,
			UnmetCriteria:  0,
			ComplianceRate: 100.0,
			Details:        "Tidak ada kriteria spesifik yang dideklarasikan; evaluasi DoD lolos default.",
		}
	}

	evidenceCorpus := strings.ToLower(strings.Join([]string{
		evidence.Summary,
		evidence.CompleteNote,
		evidence.TestOutput,
		evidence.RawEvidence,
		strings.Join(evidence.Artifacts, " "),
	}, " "))

	res := &AnalysisResult{
		TotalCriteria: len(criteria),
		Gaps:          make([]GapItem, 0),
	}

	for _, crit := range criteria {
		tokens := ExtractSignificantTokens(crit.Text)

		// 1. Check for contradictory negative evidence first
		if hasContradiction, reason := CheckContradiction(evidenceCorpus, tokens); hasContradiction {
			res.Gaps = append(res.Gaps, GapItem{
				Criterion: crit,
				Type:      GapFailedEvidence,
				Reason:    reason,
			})
			continue
		}

		// 2. If criterion was already marked completed in checklist (- [x])
		if crit.Checked {
			res.MetCriteria++
			continue
		}

		// 3. Unchecked criterion: analyze if evidence contains substantial proof
		if len(tokens) == 0 {
			// Very short generic criterion without tokens, fallback to literal substring
			if strings.Contains(evidenceCorpus, strings.ToLower(crit.Text)) {
				res.MetCriteria++
			} else {
				res.Gaps = append(res.Gaps, GapItem{
					Criterion: crit,
					Type:      GapMissingEvidence,
					Reason:    fmt.Sprintf("Kriteria %q belum terbukti pada laporan verifikasi", crit.Text),
				})
			}
			continue
		}

		// Count token matches in evidence
		matchCount := 0
		for _, tok := range tokens {
			if strings.Contains(evidenceCorpus, tok) {
				matchCount++
			}
		}

		// Threshold: At least 50% of significant tokens must appear in evidence,
		// or at least 1 match if only 1 token exists
		requiredMatches := int(math.Ceil(float64(len(tokens)) * 0.45))
		if requiredMatches < 1 {
			requiredMatches = 1
		}

		if matchCount >= requiredMatches {
			res.MetCriteria++
		} else {
			res.Gaps = append(res.Gaps, GapItem{
				Criterion: crit,
				Type:      GapMissingEvidence,
				Reason:    fmt.Sprintf("Kriteria %q tidak memiliki bukti verifikasi yang memadai (%d/%d kata kunci ditemukan)", crit.Text, matchCount, len(tokens)),
			})
		}
	}

	res.UnmetCriteria = len(res.Gaps)
	if res.TotalCriteria > 0 {
		res.ComplianceRate = math.Round((float64(res.MetCriteria)/float64(res.TotalCriteria))*1000) / 10
	}
	res.Passed = (res.UnmetCriteria == 0)

	// Format structured details
	var details strings.Builder
	details.WriteString("### Evaluasi Definition of Done (DoD)\n")
	if res.Passed {
		details.WriteString(fmt.Sprintf("**Status:** LULUS (100%% Kriteria Terpenuhi — %d/%d)\n\n", res.MetCriteria, res.TotalCriteria))
		details.WriteString("**Kriteria yang Terpenuhi:**\n")
		for _, c := range criteria {
			details.WriteString(fmt.Sprintf("- [x] %s\n", c.Text))
		}
	} else {
		details.WriteString(fmt.Sprintf("**Status:** BELUM LULUS (Tingkat Kepatuhan: %.1f%% — %d/%d Terpenuhi)\n\n", res.ComplianceRate, res.MetCriteria, res.TotalCriteria))
		details.WriteString("**Celah (Gaps) yang Ditemukan:**\n")
		for _, g := range res.Gaps {
			details.WriteString(fmt.Sprintf("- [ ] %s: %s\n", g.Criterion.Text, g.Reason))
		}
	}
	res.Details = details.String()

	return res
}

// Evaluate performs end-to-end DoD evaluation from markdown instructions and evidence.
func Evaluate(instructionsMarkdown string, evidence Evidence) *AnalysisResult {
	criteria := ParseAcceptanceCriteria(instructionsMarkdown)
	return AnalyzeGaps(criteria, evidence)
}

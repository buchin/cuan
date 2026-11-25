package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestInboxReviewFormatRepairPreservesClassificationWithoutReinspection(t *testing.T) {
	for _, scenario := range []string{"repaired", "still invalid", "tool rejected"} {
		t.Run(scenario, func(t *testing.T) {
			home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
			valid := `{"decision":"revise","source_review":"not_checked","remaining_work":"publication","explanation":"Source checked; publication remains.","revision":"Reconcile retained source and publish within original scope.","visual_inspection":"Existing supplied image inspected.","finding":"Publication remains."}`
			invalid := strings.TrimSuffix(valid, "}") + `,"unknown":true}`
			wrap := func(response string) string {
				b, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": response}})
				return string(b)
			}
			script := "#!/bin/sh\nif [ ! -e \"$HOME/review-first\" ]; then\n touch \"$HOME/review-first\"\n printf '%s\\n' '" + wrap(invalid) + "'\nelse\n"
			script += `case "$2" in *"REVIEW TRANSPORT FORMAT REPAIR ONLY"*"Source checked; publication remains."*) ;; *) exit 23;; esac
case "$2" in *"PRIVATE_REVIEW_CONTEXT"*) exit 24;; esac
case "$*" in *"--print-timeout 40s"*) ;; *) exit 25;; esac
for arg in "$@"; do [ "$arg" != "--add-dir" ] || exit 26; done
printf x >> "$HOME/review-repairs"
`
			if scenario == "tool rejected" {
				script += `printf '%s\n' '{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}'
sleep 1
`
			}
			result := valid
			if scenario == "still invalid" {
				result = invalid
			}
			script += "printf '%s\\n' '" + wrap(result) + "'\nfi\n"
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			installInboxSandboxStub(t, bin)
			t.Setenv("PATH", bin+":/usr/bin:/bin")
			s := &Server{cfg: &config.Config{Environment: "staging"}}
			ctx := context.WithValue(context.Background(), inboxTaskReviewKey{}, true)
			var trace []string
			out, failure := s.executeInboxCLI(ctx, "Coordinator", "agy", "PRIVATE_REVIEW_CONTEXT", home, workspace, func(k, v string) { trace = append(trace, k+":"+v) })
			calls, _ := os.ReadFile(filepath.Join(home, "review-repairs"))
			if string(calls) != "x" {
				t.Fatal("expected one format repair", string(calls), failure)
			}
			if scenario == "repaired" {
				d, err := parseInboxReviewEvaluation(out)
				if failure != "" || err != nil || d.RemainingWork != "publication" || d.Revision == "" {
					t.Fatal("lost review classification", out, failure, err)
				}
			} else if failure == "" || out != "" {
				t.Fatal("invalid review accepted", out, failure)
			}
			joined := strings.Join(trace, "\n")
			if !strings.Contains(joined, "response_recovery") || strings.Contains(joined, "Source checked; publication remains.") {
				t.Fatal("missing safe trace or leaked candidate", joined)
			}
		})
	}
}

func TestInboxReviewCLIAcceptsDetailedAuditPassWithoutValidationError(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	detailedFinding := strings.TrimSpace(strings.Repeat("Temuan audit read-only: akar masalah terverifikasi pada commit 2ba0214b dan log att_95b06c8d. ", 15))
	validBytes, _ := json.Marshal(inboxReviewEvaluation{
		Decision:         "pass",
		SourceReview:     "not_checked",
		RequestedOutcome: "completed",
		RemainingWork:    "none",
		VisualInspection: "",
		Finding:          detailedFinding,
		Explanation:      "Audit read-only selesai dan memenuhi seluruh kriteria.",
		Revision:         "",
	})
	wrapBytes, _ := json.Marshal(map[string]any{
		"event": "result",
		"result": map[string]string{
			"status":   "SUCCESS",
			"response": string(validBytes),
		},
	})
	script := "#!/bin/sh\nprintf '%s\\n' '" + strings.ReplaceAll(string(wrapBytes), "'", "'\\''") + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")

	s := &Server{cfg: &config.Config{Environment: "staging"}}
	ctx := context.WithValue(context.Background(), inboxTaskReviewKey{}, true)
	out, failure := s.executeInboxCLI(ctx, "Coordinator", "agy", "PRIVATE_REVIEW_CONTEXT", home, workspace, nil)
	if failure != "" {
		t.Fatalf("expected audit review response to validate cleanly, got failure=%q", failure)
	}
	eval, err := parseInboxReviewEvaluation(out)
	if err != nil || eval.Decision != "pass" || eval.Finding != detailedFinding {
		t.Fatalf("unexpected validated output: out=%q err=%v eval=%+v", out, err, eval)
	}
}

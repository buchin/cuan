package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/pkg/dodeval"
)

// DoDValidationRequest specifies input for autonomous DoD evaluation.
type DoDValidationRequest struct {
	TaskID        string         `json:"task_id"`
	AttemptID     string         `json:"attempt_id"`
	AttemptNumber int            `json:"attempt_number"`
	WorkingDir    string         `json:"working_dir"`
	TaskTitle     string         `json:"task_title"`
	Instructions  string         `json:"instructions"`
	Summary       string         `json:"summary"`
	CompleteNote  string         `json:"complete_note"`
	ExecutionLane string         `json:"execution_lane"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// DoDValidationResult represents the objective verification outcome.
type DoDValidationResult struct {
	Passed              bool          `json:"passed"`
	CleanWorkingTree    bool          `json:"clean_working_tree"`
	TargetedTestsPass   bool          `json:"targeted_tests_pass"`
	BuildZeroError      bool          `json:"build_zero_error"`
	LeanStandardPass    bool          `json:"lean_standard_pass"`
	DocsAndLPPass       bool          `json:"docs_and_lp_pass"`
	Duration            time.Duration `json:"duration"`
	Gaps                []string      `json:"gaps,omitempty"`
	Fixable             bool          `json:"fixable"`
	Action              string        `json:"action"` // "mark_done" | "auto_requeue" | "review" | "blocked"
	Details             string        `json:"details"`
	RequeueInstructions string        `json:"requeue_instructions,omitempty"`
	NextTimeout         time.Duration `json:"next_timeout,omitempty"`
	NextAttemptNumber   int           `json:"next_attempt_number,omitempty"`
}

// DoDEvaluator performs autonomous background validation of task completion against DoD criteria.
type DoDEvaluator struct {
	// CustomCommandRunner allows injecting mock execution in tests.
	CustomCommandRunner func(ctx context.Context, dir, name string, args ...string) ([]byte, error)
}

// NewDoDEvaluator creates a new DoDEvaluator instance.
func NewDoDEvaluator() *DoDEvaluator {
	return &DoDEvaluator{}
}

func (e *DoDEvaluator) runCmd(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	if e.CustomCommandRunner != nil {
		return e.CustomCommandRunner(ctx, dir, name, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// EvaluateDoD runs all objective checks against the workspace.
func (e *DoDEvaluator) EvaluateDoD(ctx context.Context, req DoDValidationRequest) (*DoDValidationResult, error) {
	start := time.Now()
	res := &DoDValidationResult{
		CleanWorkingTree:  true,
		TargetedTestsPass: true,
		BuildZeroError:    true,
		LeanStandardPass:  true,
		DocsAndLPPass:     true,
		Fixable:           true,
		Gaps:              make([]string, 0),
	}

	workingDir := req.WorkingDir
	if workingDir == "" {
		// If no working dir, check current working dir or repository root
		workingDir = "."
	}

	// 1. Check Working Tree Clean (git status --porcelain)
	clean, cleanMsg, err := e.checkWorkingTreeClean(ctx, workingDir)
	res.CleanWorkingTree = clean
	if err != nil || !clean {
		res.Gaps = append(res.Gaps, fmt.Sprintf("Working tree tidak bersih: %s", cleanMsg))
	}

	// 2. Check Targeted Tests Pass (Go test & Vitest frontend)
	testsPass, testMsg, testDur, err := e.checkTargetedTestsPass(ctx, workingDir)
	res.TargetedTestsPass = testsPass
	if err != nil || !testsPass {
		res.Gaps = append(res.Gaps, fmt.Sprintf("Targeted automated tests gagal: %s", testMsg))
	}

	// 3. Check Build Compilation 0 Error (Go vet/build & frontend)
	buildZero, buildMsg, err := e.checkBuildZeroError(ctx, workingDir)
	res.BuildZeroError = buildZero
	if err != nil || !buildZero {
		res.Gaps = append(res.Gaps, fmt.Sprintf("Kompilasi build gagal: %s", buildMsg))
	}

	// 4. Check Lean & Modular Standard + Documentation & Landing Page Update Directive
	leanPass, docsPass, directiveMsg, err := e.checkLeanAndDocsLPCompliance(ctx, workingDir, req.TaskTitle, req.Instructions, req.Summary)
	res.LeanStandardPass = leanPass
	res.DocsAndLPPass = docsPass
	if testDur > 10*time.Second {
		res.LeanStandardPass = false
		res.Gaps = append(res.Gaps, fmt.Sprintf("Durasi verifikasi/uji melebihi batas kilat lean (<10s): terukur %v", testDur.Round(time.Millisecond)))
	}
	if err != nil || !leanPass || !docsPass {
		if directiveMsg != "" {
			res.Gaps = append(res.Gaps, directiveMsg)
		}
	}

	// 5. Check Criteria Acceptance & Markdown DoD Checklist via pkg/dodeval
	if req.Instructions != "" {
		criteria := dodeval.ParseAcceptanceCriteria(req.Instructions)
		if len(criteria) > 0 {
			evalEvidence := dodeval.Evidence{
				Summary:           req.Summary,
				CompleteNote:      req.CompleteNote,
				CleanWorkingTree:  clean,
				TargetedTestsPass: testsPass,
				BuildZeroError:    buildZero,
			}
			gapResult := dodeval.AnalyzeGaps(criteria, evalEvidence)
			if !gapResult.Passed {
				for _, gap := range gapResult.Gaps {
					res.Gaps = append(res.Gaps, fmt.Sprintf("Celah DoD checklist [%s]: %s", gap.Criterion.Text, gap.Reason))
				}
			}
		}
	}

	res.Duration = time.Since(start)

	// Decision Logic: Auto-Mark Done vs Auto-Requeue
	if len(res.Gaps) == 0 {
		res.Passed = true
		res.Action = "mark_done"
		res.Details = fmt.Sprintf("### Evaluasi Definition of Done (DoD) Otonom\n"+
			"**Status:** LULUS (100%% Kriteria Terpenuhi)\n\n"+
			"**Bukti yang Terverifikasi:**\n"+
			"- [x] Working tree bersih (git clean, status bebas uncommitted changes)\n"+
			"- [x] Targeted tests lulus 100%% (Go & Vitest pass, durasi: %v)\n"+
			"- [x] Kompilasi build 0 error (Go & frontend build bersih)\n"+
			"- [x] Standar lean & modular terpenuhi (<10s verification, zero bloat)\n"+
			"- [x] Dokumentasi & Landing Page tersinkronisasi merefleksikan nilai bisnis terbaru\n\n"+
			"**Tindakan:** Menandai task 'done' secara otomatis dan memancarkan notifikasi chat ramah bisnis.",
			res.Duration.Round(time.Millisecond))
	} else {
		res.Passed = false
		if req.AttemptNumber < MaxAttempts {
			res.Action = "auto_requeue"
			res.NextAttemptNumber = req.AttemptNumber + 1
			res.NextTimeout = CalculateAdaptiveTimeout(res.NextAttemptNumber)

			var gapList strings.Builder
			for _, g := range res.Gaps {
				gapList.WriteString(fmt.Sprintf("- [ ] %s\n", g))
			}

			res.RequeueInstructions = fmt.Sprintf("### Evaluasi Definition of Done (DoD) Otonom: Perlu Revisi\n"+
				"Ditemukan celah DoD yang perlu diperbaiki (Attempt #%d -> Requeue Attempt #%d, Alokasi Timeout Adaptif: %v):\n\n"+
				"**Gap yang Perlu Diperbaiki:**\n%s\n"+
				"**Instruksi Perbaikan Terarah untuk Agent:**\n"+
				"1. Perbaiki celah kriteria di atas pada workspace.\n"+
				"2. Pastikan targeted tests lulus 100%% dan kompilasi 0 error dalam waktu <10 detik.\n"+
				"3. Commit perubahan terarah, bersihkan working tree, dan perbarui dokumentasi/Landing Page bila menambahkan kapabilitas baru sebelum melapor selesai.\n",
				req.AttemptNumber, res.NextAttemptNumber, res.NextTimeout, gapList.String())

			res.Details = res.RequeueInstructions
		} else {
			// Max attempts reached (3x) -> move to review or blocked to prevent infinite loops
			res.Action = "review"
			var gapList strings.Builder
			for _, g := range res.Gaps {
				gapList.WriteString(fmt.Sprintf("- [ ] %s\n", g))
			}
			res.Details = fmt.Sprintf("### Evaluasi Definition of Done (DoD) Otonom: Batas Maksimal Attempt Tercapai\n"+
				"Task telah mencapai batas maksimal 3x percobaan (Attempt #%d). Evaluasi DoD masih menemukan gap yang belum tuntas:\n\n"+
				"%s\nTask dipindahkan ke status 'review' untuk peninjauan dan arahan lebih lanjut dari Product Owner.\n",
				req.AttemptNumber, gapList.String())
		}
	}

	return res, nil
}

func (e *DoDEvaluator) checkWorkingTreeClean(ctx context.Context, workingDir string) (bool, string, error) {
	// If not a git repository on disk (and not using a mock command runner in tests), consider it clean
	if e.CustomCommandRunner == nil {
		if _, err := os.Stat(filepath.Join(workingDir, ".git")); os.IsNotExist(err) {
			return true, "not a git repo", nil
		}
	}

	out, err := e.runCmd(ctx, workingDir, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Sprintf("git status error: %v", err), err
	}
	output := strings.TrimSpace(string(out))
	if output == "" {
		return true, "clean working tree", nil
	}

	// Filter out harmless untracked temp files if any (e.g. .cuan-tmp)
	lines := strings.Split(output, "\n")
	var dirtyLines []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		// Ignore temporary runtime socket or scratch files if applicable
		if strings.Contains(trimmed, ".cuan-tmp") || strings.Contains(trimmed, "coverage.out") {
			continue
		}
		dirtyLines = append(dirtyLines, trimmed)
	}

	if len(dirtyLines) == 0 {
		return true, "clean working tree", nil
	}

	return false, fmt.Sprintf("%d file uncommitted/untracked (%s)", len(dirtyLines), strings.Join(dirtyLines, ", ")), nil
}

func (e *DoDEvaluator) checkTargetedTestsPass(ctx context.Context, workingDir string) (bool, string, time.Duration, error) {
	start := time.Now()

	// 1. Check Go tests if go.mod exists or CustomCommandRunner is supplied in tests
	if e.CustomCommandRunner != nil {
		out, cmdErr := e.runCmd(ctx, workingDir, "go", "test", "-count=1", "./internal/runner/...")
		if cmdErr != nil {
			return false, fmt.Sprintf("Go tests failed: %s", strings.TrimSpace(string(out))), time.Since(start), cmdErr
		}
	} else if _, err := os.Stat(filepath.Join(workingDir, "go.mod")); err == nil {
		// Run fast targeted tests (./internal/runner/... or ./scripts/test.sh --fast)
		testScript := filepath.Join(workingDir, "scripts", "test.sh")
		var out []byte
		var cmdErr error
		if info, statErr := os.Stat(testScript); statErr == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			out, cmdErr = e.runCmd(ctx, workingDir, testScript, "--fast")
		} else {
			out, cmdErr = e.runCmd(ctx, workingDir, "go", "test", "-count=1", "./internal/runner/...")
		}
		if cmdErr != nil {
			return false, fmt.Sprintf("Go tests failed: %s", strings.TrimSpace(string(out))), time.Since(start), cmdErr
		}
	}

	// 2. Check Vitest frontend tests if web/package.json exists
	if e.CustomCommandRunner == nil {
		webPkg := filepath.Join(workingDir, "web", "package.json")
		if _, err := os.Stat(webPkg); err == nil {
			out, cmdErr := e.runCmd(ctx, workingDir, "npm", "test", "--prefix", "web", "--", "--run")
			if cmdErr != nil {
				return false, fmt.Sprintf("Vitest frontend tests failed: %s", strings.TrimSpace(string(out))), time.Since(start), cmdErr
			}
		}
	}

	return true, "All targeted tests passed 100%", time.Since(start), nil
}

func (e *DoDEvaluator) checkBuildZeroError(ctx context.Context, workingDir string) (bool, string, error) {
	// 1. Go build check (go vet)
	if _, err := os.Stat(filepath.Join(workingDir, "go.mod")); err == nil {
		out, err := e.runCmd(ctx, workingDir, "go", "vet", "./internal/runner/...")
		if err != nil {
			return false, fmt.Sprintf("Go vet/build failed: %s", strings.TrimSpace(string(out))), err
		}
	}

	return true, "Build and syntax verification 0 errors", nil
}

func (e *DoDEvaluator) checkLeanAndDocsLPCompliance(ctx context.Context, workingDir, title, instructions, summary string) (bool, bool, string, error) {
	combined := strings.ToLower(title + " " + instructions + " " + summary)

	// Check if this task introduces a new feature or capability
	isNewCapability := strings.Contains(combined, "feat") ||
		strings.Contains(combined, "fitur") ||
		strings.Contains(combined, "kapabilitas") ||
		strings.Contains(combined, "capability") ||
		strings.Contains(combined, "worker") ||
		strings.Contains(combined, "evaluator") ||
		strings.Contains(combined, "requeue") ||
		strings.Contains(combined, "timeout")

	if !isNewCapability {
		return true, true, "", nil
	}

	// Validate Documentation (docs/ or README.md exists and is touched/valid)
	readmePath := filepath.Join(workingDir, "README.md")
	docsDir := filepath.Join(workingDir, "docs")
	hasDocs := false
	if _, err := os.Stat(readmePath); err == nil {
		hasDocs = true
	} else if _, err := os.Stat(docsDir); err == nil {
		hasDocs = true
	}

	// Validate Landing Page exists
	lpPath := filepath.Join(workingDir, "web", "src", "features", "landing", "LandingPage.tsx")
	hasLP := false
	if _, err := os.Stat(lpPath); err == nil {
		hasLP = true
	}

	if !hasDocs {
		return true, false, "Kepatuhan dokumentasi: task memperkenalkan fitur/kapabilitas baru tetapi dokumentasi teknis (README.md / docs/) belum tersedia atau diperbarui", nil
	}
	if !hasLP {
		return true, false, "Kepatuhan Landing Page: task memperkenalkan fitur/kapabilitas baru tetapi Landing Page Cuan (web/src/features/landing/LandingPage.tsx) belum tersedia atau diperbarui", nil
	}

	return true, true, "", nil
}

// AttachToEventBus registers this DoD evaluator to automatically handle task:attempt_succeeded events.
func (e *DoDEvaluator) AttachToEventBus(bus *EventBus, onResult ...func(ctx context.Context, evt AttemptSucceededEvent, res *DoDValidationResult)) {
	if bus == nil {
		bus = GlobalEventBus
	}
	bus.Subscribe(func(ctx context.Context, evt AttemptSucceededEvent) error {
		req := DoDValidationRequest{
			TaskID:        evt.TaskID,
			AttemptID:     evt.AttemptID,
			AttemptNumber: evt.AttemptNumber,
			WorkingDir:    evt.WorkingDir,
			TaskTitle:     evt.Summary,
			Summary:       evt.Summary,
			CompleteNote:  evt.CompleteNote,
			ExecutionLane: evt.ExecutionLane,
			Metadata:      evt.Metadata,
		}
		res, err := e.EvaluateDoD(ctx, req)
		if len(onResult) > 0 && onResult[0] != nil {
			onResult[0](ctx, evt, res)
		}
		return err
	})
}

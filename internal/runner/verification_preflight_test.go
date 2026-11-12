package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

// Stale project configuration used to surface only after the agent finished its
// work: the attempt was spent, then verification refused to run. Detect it first.
func TestStaleRegisteredTargetIsReportedBeforeWork(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tests", "Feature"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "Feature", "RealTest.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := `["tests/Feature/MissingTest.php"]`
	got := validateRegisteredVerificationTargets(root, raw)
	if got == "" {
		t.Fatal("missing declared target was accepted")
	}
	if task.RetryWorthy(got) {
		t.Fatalf("stale configuration must not consume the attempt budget: %q", got)
	}
	if task.ClassifyAttemptFailure(got) != task.FailurePrerequisite {
		t.Fatalf("stale configuration classified as %q, want prerequisite", task.ClassifyAttemptFailure(got))
	}
}

// A workspace whose declared checks exist must proceed, otherwise a correct
// project would be blocked by its own valid configuration.
func TestExistingRegisteredTargetIsAccepted(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tests", "Feature"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests", "Feature", "RealTest.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := validateRegisteredVerificationTargets(root, `["tests/Feature/RealTest.php"]`); got != "" {
		t.Fatalf("existing declared target was rejected: %q", got)
	}
}

// A target that escapes the workspace is configuration abuse, not a check.
func TestRegisteredTargetOutsideWorkspaceIsRejected(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "Outside.php")
	if err := os.WriteFile(outside, []byte("<?php"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := validateRegisteredVerificationTargets(root, `["../`+filepath.Base(filepath.Dir(outside))+`/Outside.php"]`); got == "" {
		t.Fatal("target escaping the workspace was accepted")
	}
}

// No declared targets means the project uses its own verification, not a gate.
func TestNoRegisteredTargetsIsNotAPrerequisite(t *testing.T) {
	if got := validateRegisteredVerificationTargets(t.TempDir(), ""); got != "" {
		t.Fatalf("empty configuration reported as prerequisite: %q", got)
	}
}

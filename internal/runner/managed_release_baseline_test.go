package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cuangit "github.com/buchin/cuan/internal/git"
)

func TestManagedDeveloperRejectsStaleCheckpointUntilLocalBaselineReconciled(t *testing.T) {
	root := documentationFixture(t)
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("feature.go", "package feature\n")
	run("add", "feature.go")
	run("commit", "-m", "Approved local change")
	run("checkout", "dev")
	write("baseline.go", "package baseline\n")
	run("add", "baseline.go")
	run("commit", "-m", "New project baseline")
	baseline := run("rev-parse", "HEAD")
	run("checkout", "task")
	runner := &Runner{gitClient: cuangit.NewClient()}
	workspace := &cuangit.TaskWorkspace{Path: root, BaseCommit: baseline}
	head, refs, status := run("rev-parse", "HEAD"), run("show-ref"), run("status", "--porcelain")
	if err := runner.verifyManagedDeveloperBaseline(context.Background(), workspace); err == nil || !strings.Contains(err.Error(), "private task branch") {
		t.Fatal("stale unchanged checkpoint was accepted for another source review", err)
	}
	if head != run("rev-parse", "HEAD") || refs != run("show-ref") || status != run("status", "--porcelain") {
		t.Fatal("baseline check mutated saved work")
	}
	// The developer may reconcile locally; no remote publication is needed.
	run("merge", "--no-edit", "dev")
	if err := runner.verifyManagedDeveloperBaseline(context.Background(), workspace); err != nil {
		t.Fatal("reconciled task branch rejected", err)
	}
	if err := runner.verifyManagedDeveloperBaseline(context.Background(), &cuangit.TaskWorkspace{Path: root}); err == nil {
		t.Fatal("unknown baseline silently accepted")
	}
}

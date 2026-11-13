package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProjectVerifierReusesOnlyExecutionBuildSettings(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts/test.sh"), []byte("#!/bin/sh\nset -eu\ntest -z \"${CUAN_VERIFIER_PROJECT_SECRET:-}\"\ntest \"$GOENV\" = off\nprintf x >> \"$GOCACHE/checks\"\nprintf '%s\\n' \"$GOCACHE\" \"$GOMODCACHE\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "wrong-ambient-cache"))
	t.Setenv("CUAN_VERIFIER_PROJECT_SECRET", "")
	for _, owner := range []string{"owner-a", "owner-b"} {
		cache := t.TempDir()
		settings := map[string]string{"GOCACHE": cache, "GOMODCACHE": filepath.Join(cache, "modules"), "GOENV": "off", "CUAN_VERIFIER_PROJECT_SECRET": "must-not-inherit"}
		ctx := withProjectVerifierBuildEnvironment(context.Background(), settings)
		// A later mutation of the model's environment must not change the checks.
		settings["GOCACHE"] = "/unavailable-mutated-cache"
		for i := 0; i < 2; i++ {
			got := runProjectVerification(ctx, root)
			if !got.Passed || !strings.Contains(got.Summary, cache+"\n"+filepath.Join(cache, "modules")) {
				t.Fatalf("%s native checks lost execution cache: %+v", owner, got)
			}
		}
		raw, err := os.ReadFile(filepath.Join(cache, "checks"))
		if err != nil || string(raw) != "xx" {
			t.Fatalf("%s did not reuse its cache: %q, %v", owner, raw, err)
		}
	}
}

func TestProjectVerifierDeadlineStopsDescendantHoldingOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(root, "child.pid")
	t.Setenv("VERIFIER_FIXTURE_PID", pidFile)
	if err := os.WriteFile(filepath.Join(root, "scripts/test.sh"), []byte("#!/bin/sh\nsleep 30 &\nchild=$!\nprintf '%s' \"$child\" > \"$VERIFIER_FIXTURE_PID\"\nwait \"$child\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		raw, _ := os.ReadFile(pidFile)
		pid, _ := strconv.Atoi(string(raw))
		if pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	result := make(chan verificationResult, 1)
	go func() { result <- runProjectVerification(ctx, root) }()
	select {
	case got := <-result:
		if got.Passed || got.Error != "Project verification stopped at the task deadline" {
			t.Fatal("timed-out verifier reported an incorrect outcome", got)
		}
		if !strings.Contains(got.Summary, "Native verifier result: Project verification stopped at the task deadline") {
			t.Fatal("native diagnostic lost timeout reason", got)
		}
		if strings.Contains(got.Summary, "exit 0") {
			t.Fatal("failed verifier claimed exit 0")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("task deadline left verifier waiting for descendant output")
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal("fixture child never started", err)
	}
	pid, err := strconv.Atoi(string(raw))
	if err != nil || pid <= 0 {
		t.Fatal("invalid fixture child PID")
	}
	stopBy := time.Now().Add(500 * time.Millisecond)
	for {
		proc, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		end := strings.LastIndex(string(proc), ") ")
		if end >= 0 && (string(proc)[end+2] == 'Z' || string(proc)[end+2] == 'X') {
			break
		}
		if time.Now().After(stopBy) {
			t.Fatal("verifier descendant survived task deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}

}

func TestProjectVerifierOutputTailIsBoundedDuringWrites(t *testing.T) {
	var output verifierOutputTail
	chunk := strings.Repeat("diagnostic", 2000)
	for i := 0; i < 40; i++ {
		if n, err := output.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatal("writer lost byte count")
		}
	}
	output.Write([]byte("FINAL-DIAGNOSTIC"))
	if len(output.String()) != projectVerifierOutputLimit || cap(output.data) > projectVerifierOutputLimit || !strings.HasSuffix(output.String(), "FINAL-DIAGNOSTIC") {
		t.Fatal("diagnostic output is unbounded or lost its tail")
	}
}

func TestProjectVerifierStartFailureNeverClaimsSuccessfulExit(t *testing.T) {
	got := runProjectVerifierCommand(context.Background(), t.TempDir(), "/missing-cuan-fixture-verifier")
	if got.Passed || got.Error != "Project verifier could not start" || !strings.Contains(got.Summary, "exit -1") {
		t.Fatal("misleading verifier startup evidence", got)
	}
}

func TestProjectVerifierExitedShellCannotLeaveChildPipeOpen(t *testing.T) {
	root := t.TempDir()
	command := filepath.Join(root, "verify.sh")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nsleep 30 &\nprintf 'parent finished\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	got := runProjectVerifierCommand(context.Background(), root, command)
	if got.Passed || got.Error != "Project verifier exited without closing child output" || strings.HasPrefix(got.Summary, "Project verification:") || time.Since(start) > 2*time.Second {
		t.Fatal("orphaned output pipe accepted or not bounded", got)
	}
}

func TestProjectVerifierDeadlineStopsDetachedBrowserStyleChild(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("skipping on platforms without setsid")
	}
	root := t.TempDir()
	pidFile := filepath.Join(root, "detached.pid")
	script := filepath.Join(root, "verify.sh")
	body := fmt.Sprintf("#!/bin/sh\nsetsid sh -c 'echo $$ > \"$1\"; sleep 30' detached %q &\nwait\n", pidFile)
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		raw, _ := os.ReadFile(pidFile)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
		if pid > 0 {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	result := runProjectVerifierCommand(ctx, root, script)
	if result.Passed || result.Error != "Project verification stopped at the task deadline" {
		t.Fatal("detached verifier incorrectly completed", result)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal("detached child never started", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		t.Fatal("invalid detached pid")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		state, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if os.IsNotExist(err) || strings.Contains(string(state), ") Z") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("detached browser-style child survived verifier deadline")
}

func TestProjectVerifierCleansCompilerScratchAfterExitAndTimeout(t *testing.T) {
	for _, mode := range []string{"success", "failure", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			record := filepath.Join(root, "scratch-path")
			t.Setenv("CUAN_FIXTURE_SCRATCH_RECORD", record)
			cache := t.TempDir()
			preserved := filepath.Join(cache, "cached-object")
			if err := os.WriteFile(preserved, []byte("retained cache"), 0600); err != nil {
				t.Fatal(err)
			}
			script := filepath.Join(root, "verify.sh")
			body := "#!/bin/sh\nset -eu\nprintf '%s' \"$GOTMPDIR\" > \"$CUAN_FIXTURE_SCRATCH_RECORD\"\nmkdir \"$GOTMPDIR/go-build123\"\nprintf compiler-output > \"$GOTMPDIR/go-build123/object\"\n"
			if mode == "failure" {
				body += "exit 1\n"
			} else if mode == "timeout" {
				body += "sleep 30\n"
			}
			if err := os.WriteFile(script, []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			ctx := withProjectVerifierBuildEnvironment(context.Background(), map[string]string{"GOCACHE": cache})
			if mode == "timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 1500*time.Millisecond)
				defer cancel()
			}
			result := runProjectVerifierCommand(ctx, root, script)
			if result.Passed != (mode == "success") {
				t.Fatal("incorrect check outcome", result)
			}
			raw, err := os.ReadFile(record)
			if err != nil || len(raw) == 0 {
				t.Fatal("compiler scratch was not exercised", err)
			}
			if _, err := os.Stat(string(raw)); !os.IsNotExist(err) {
				t.Fatal("compiler scratch survived command completion", err)
			}
			if raw, err := os.ReadFile(preserved); err != nil || string(raw) != "retained cache" {
				t.Fatal("shared cache was changed", err)
			}
		})
	}
}

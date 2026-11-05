package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

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
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	result := make(chan verificationResult, 1)
	go func() { result <- runProjectVerification(ctx, root) }()
	select {
	case got := <-result:
		if got.Passed || got.Error != "Project verification stopped at the task deadline" {
			t.Fatal("timed-out verifier reported an incorrect outcome", got)
		}
		if strings.Contains(got.Summary, "exit 0") {
			t.Fatal("failed verifier claimed exit 0")
		}
	case <-time.After(2 * time.Second):
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

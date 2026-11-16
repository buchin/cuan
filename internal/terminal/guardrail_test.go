package terminal

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/buchin/agentrunner"
)

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestGetTerminalMaxRSSMB(t *testing.T) {
	// Default
	t.Setenv("CUAN_TERMINAL_MAX_RSS_MB", "")
	if got := GetTerminalMaxRSSMB(); got != DefaultMaxRSSMB {
		t.Fatalf("expected default %d, got %d", DefaultMaxRSSMB, got)
	}

	// Custom valid
	t.Setenv("CUAN_TERMINAL_MAX_RSS_MB", "2048")
	if got := GetTerminalMaxRSSMB(); got != 2048 {
		t.Fatalf("expected 2048, got %d", got)
	}

	// Invalid fallback
	t.Setenv("CUAN_TERMINAL_MAX_RSS_MB", "not-a-number")
	if got := GetTerminalMaxRSSMB(); got != DefaultMaxRSSMB {
		t.Fatalf("expected fallback to default, got %d", got)
	}
}

func TestGetProcessRSS(t *testing.T) {
	pid := os.Getpid()
	rss, comm, err := GetProcessRSS(pid)
	if err != nil {
		t.Fatalf("GetProcessRSS for current pid %d failed: %v", pid, err)
	}
	if rss == 0 {
		t.Fatalf("expected non-zero RSS for pid %d", pid)
	}
	t.Logf("PID %d RSS: %d bytes (%d MB), Comm: %q", pid, rss, rss/(1024*1024), comm)
}

func TestRunTerminalMemoryGuardrail_TerminatesRunawayProcess(t *testing.T) {
	// Spawn a parent shell process that spawns a python child allocating memory
	cmd := exec.Command("bash", "-c", "python3 -c 'import time; time.sleep(30)' & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Skipf("skipping: bash or python3 not available: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	parentPID := cmd.Process.Pid
	time.Sleep(300 * time.Millisecond)

	children := GetTerminalChildPIDs(parentPID)
	if len(children) == 0 {
		t.Skip("no child processes discovered")
	}

	outBuf := &safeBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2 MB threshold: python3 uses > 10MB RSS so it will be terminated immediately
	go RunTerminalMemoryGuardrail(ctx, parentPID, outBuf, 2, 100*time.Millisecond)

	// Wait up to 3 seconds for guardrail to detect and terminate child
	deadline := time.Now().Add(3 * time.Second)
	childTerminated := false
	for time.Now().Before(deadline) {
		anyAlive := false
		for _, cpid := range children {
			if cpid != parentPID && agentrunner.IsProcessAlive(cpid) {
				anyAlive = true
			}
		}
		if !anyAlive {
			childTerminated = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !childTerminated {
		t.Fatal("expected python3 child to be terminated by memory guardrail")
	}

	// Verify warning message was written to outBuf
	if !strings.Contains(outBuf.String(), "[Cuan Guardrail]") {
		t.Errorf("expected guardrail warning in pty output, got: %q", outBuf.String())
	}
}

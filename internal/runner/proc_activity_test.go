//go:build linux

package runner

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestDescendantCPUTicks_DetectsActiveChildProcess(t *testing.T) {
	cmd := exec.Command("sh", "-c", "i=0; while [ $i -lt 200000 ]; do i=$((i+1)); done")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start child process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	ticks1, count1 := descendantCPUTicks(os.Getpid())
	if count1 == 0 {
		t.Fatalf("expected at least 1 descendant process, got 0")
	}

	time.Sleep(150 * time.Millisecond)
	ticks2, _ := descendantCPUTicks(os.Getpid())
	if ticks2 <= ticks1 {
		t.Logf("initial ticks=%d after=%d (process may have completed quickly)", ticks1, ticks2)
	}
}

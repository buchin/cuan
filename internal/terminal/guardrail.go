package terminal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/buchin/agentrunner"
)

const (
	DefaultMaxRSSMB = 1536 // 1.5 GB default
)

// ProcessMemoryInfo holds RSS bytes and executable name for a process.
type ProcessMemoryInfo struct {
	PID  int
	RSS  uint64
	Comm string
}

// GetTerminalMaxRSSMB reads CUAN_TERMINAL_MAX_RSS_MB from environment or returns default.
func GetTerminalMaxRSSMB() int {
	if val := os.Getenv("CUAN_TERMINAL_MAX_RSS_MB"); val != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return parsed
		}
	}
	return DefaultMaxRSSMB
}

// GetProcessRSS returns the resident set size (in bytes) and command name for a given PID.
func GetProcessRSS(pid int) (uint64, string, error) {
	if pid <= 0 {
		return 0, "", fmt.Errorf("invalid pid %d", pid)
	}

	if runtime.GOOS == "linux" {
		// Read /proc/[pid]/statm
		statmPath := fmt.Sprintf("/proc/%d/statm", pid)
		data, err := os.ReadFile(statmPath)
		if err != nil {
			return 0, "", err
		}
		fields := strings.Fields(string(data))
		if len(fields) < 2 {
			return 0, "", fmt.Errorf("malformed statm for pid %d", pid)
		}
		pages, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, "", err
		}
		pageSize := uint64(os.Getpagesize())
		rssBytes := pages * pageSize

		comm := ""
		if commData, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
			comm = strings.TrimSpace(string(commData))
		}
		return rssBytes, comm, nil
	}

	// Fallback using ps command (macOS and BSD)
	cmd := exec.Command("ps", "-o", "rss=,comm=", "-p", strconv.Itoa(pid))
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, "", err
	}

	fields := strings.Fields(strings.TrimSpace(out.String()))
	if len(fields) == 0 {
		return 0, "", fmt.Errorf("no ps output for pid %d", pid)
	}
	rssKB, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, "", err
	}
	comm := ""
	if len(fields) > 1 {
		comm = filepath.Base(fields[1])
	}
	return rssKB * 1024, comm, nil
}

// GetTerminalChildPIDs returns all descendant PIDs under the given root PID.
func GetTerminalChildPIDs(pid int) []int {
	if pid <= 0 {
		return nil
	}
	if runtime.GOOS == "linux" {
		return agentrunner.GetChildPIDs(pid)
	}
	// Fallback for macOS / Darwin: use pgrep -P
	var results []int
	cmd := exec.Command("pgrep", "-P", strconv.Itoa(pid))
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if childPID, err := strconv.Atoi(trimmed); err == nil && childPID > 0 {
			results = append(results, childPID)
			results = append(results, GetTerminalChildPIDs(childPID)...)
		}
	}
	return results
}

// RunTerminalMemoryGuardrail periodically inspects children of rootPID. If any child exceeds
// maxRSSMB, it issues a warning to ptyWriter and terminates the runaway process.
func RunTerminalMemoryGuardrail(ctx context.Context, rootPID int, ptyWriter io.Writer, maxRSSMB int, interval time.Duration) {
	if maxRSSMB <= 0 || rootPID <= 0 {
		return
	}
	maxBytes := uint64(maxRSSMB) * 1024 * 1024
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			children := GetTerminalChildPIDs(rootPID)
			for _, childPID := range children {
				if childPID == rootPID {
					continue
				}
				rss, comm, err := GetProcessRSS(childPID)
				if err != nil {
					continue
				}
				if rss > maxBytes {
					rssMB := rss / (1024 * 1024)
					log.Printf("[TerminalGuardrail] Process %s (PID %d) exceeded memory limit: %d MB > %d MB limit. Terminating...", comm, childPID, rssMB, maxRSSMB)
					if ptyWriter != nil {
						msg := fmt.Sprintf("\r\n\x1b[33;1m[Cuan Guardrail]\x1b[0m Process %s (PID %d) exceeded memory limit (%d MB > %d MB). Terminated to protect server stability.\r\n",
							comm, childPID, rssMB, maxRSSMB)
						_, _ = ptyWriter.Write([]byte(msg))
					}
					// Terminate runaway child
					_ = syscall.Kill(childPID, syscall.SIGTERM)
					go func(targetPID int) {
						time.Sleep(3 * time.Second)
						if agentrunner.IsProcessAlive(targetPID) {
							_ = syscall.Kill(targetPID, syscall.SIGKILL)
						}
					}(childPID)
				}
			}
		}
	}
}

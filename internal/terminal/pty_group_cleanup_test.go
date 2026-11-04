//go:build linux

package terminal

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A command can ignore terminal hangup and TERM even when the agent leader exits.
// Wait/Close must not report a stopped PTY while that command can still edit files.
func TestPTYStopsOwnedGroupWhenLeaderExits(t *testing.T) {
	for _, scenario := range []string{"context cancellation", "natural leader exit", "explicit close"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			pidFile := filepath.Join(dir, "child.pid")
			heartbeat := filepath.Join(dir, "heartbeat")
			script := `trap 'exit 0' TERM HUP
sh -c 'trap "" TERM HUP; echo $$ > "$CHILD_PID_FILE"; while :; do printf x >> "$HEARTBEAT_FILE"; sleep 0.02; done' &
while [ ! -s "$CHILD_PID_FILE" ]; do sleep 0.01; done
`
			if scenario == "natural leader exit" {
				script += "exit 0\n"
			} else {
				script += "wait\n"
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			proc, err := SpawnPTY(ctx, PTYConfig{TaskID: "group-cleanup", WorkDir: dir, Shell: "/bin/sh", Args: []string{"-c", script}, Env: []string{"CHILD_PID_FILE=" + pidFile, "HEARTBEAT_FILE=" + heartbeat}})
			if err != nil {
				t.Fatal(err)
			}
			// Cleanup is restricted to the group created by this local fixture, even when
			// the regression assertion fails. No production process is inspected/killed.
			defer func() { _ = syscall.Kill(-proc.PID(), syscall.SIGKILL); _ = proc.Close() }()
			deadline := time.Now().Add(2 * time.Second)
			var child int
			for time.Now().Before(deadline) {
				raw, _ := os.ReadFile(pidFile)
				child, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
				if child > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if child <= 0 {
				t.Fatal("fixture child did not start")
			}
			if scenario == "context cancellation" {
				group, err := syscall.Getpgid(child)
				if err != nil || group != proc.PID() {
					t.Fatal("fixture child is outside the owned PTY group", group, err)
				}
				cancel()
			}
			if scenario == "explicit close" {
				closed := make(chan error, 1)
				go func() { closed <- proc.Close() }()
				select {
				case err := <-closed:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("explicit Close exceeded its grace period")
				}
			}
			finished := make(chan error, 1)
			go func() { finished <- proc.Wait() }()
			select {
			case <-finished:
			case <-time.After(4 * time.Second):
				t.Fatal("PTY cleanup exceeded its bounded grace period")
			}
			raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(child), "stat"))
			if err == nil {
				tail := string(raw)
				at := strings.LastIndex(tail, ") ")
				if at < 0 || len(tail) <= at+2 {
					t.Fatal("invalid fixture process state")
				}
				// An already dead orphan may await PID1 reaping in the test container.
				if state := tail[at+2]; state != 'Z' && state != 'X' {
					t.Fatalf("child %d still executing after PTY Wait returned (state %c)", child, state)
				}
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(heartbeat)
			time.Sleep(100 * time.Millisecond)
			after, _ := os.ReadFile(heartbeat)
			if len(after) != len(before) {
				t.Fatal("old command kept writing after the PTY was reported stopped")
			}
		})
	}
}

func TestPTYConcurrentCloseWaitsForOwnedGroupCleanup(t *testing.T) {
	proc, err := SpawnPTY(context.Background(), PTYConfig{TaskID: "concurrent-close", WorkDir: t.TempDir(), Shell: "/bin/sh", Args: []string{"-c", "sleep 10"}})
	if err != nil {
		t.Fatal(err)
	}
	defer proc.Close()
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { done <- proc.Close() }()
	}
	deadline := time.After(3 * time.Second)
	for i := 0; i < 8; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-deadline:
			t.Fatal("concurrent Close deadlocked")
		}
	}
	select {
	case <-proc.waitDone:
	default:
		t.Fatal("Close returned before process-group cleanup")
	}
}

func TestPTYClosingOneGroupPreservesAnotherSession(t *testing.T) {
	control, err := SpawnPTY(context.Background(), PTYConfig{TaskID: "unrelated-pty", WorkDir: t.TempDir(), Shell: "/bin/sh", Args: []string{"-c", "sleep 10"}})
	if err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	target, err := SpawnPTY(context.Background(), PTYConfig{TaskID: "target-pty", WorkDir: t.TempDir(), Shell: "/bin/sh", Args: []string{"-c", "sleep 10"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if control.IsClosed() {
		t.Fatal("closing target session stopped an unrelated PTY")
	}
	if err := syscall.Kill(control.PID(), 0); err != nil {
		t.Fatal("unrelated PTY leader was killed", err)
	}
}

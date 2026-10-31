// Package taskprocess provides durable, OS-backed ownership for local task
// agents. It fences agent processes, not arbitrary external side effects.
package taskprocess

import (
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const Command = "--cuan-task-supervisor"
const ConflictExit = 75

const (
	DefaultTaskTimeout = 20 * time.Minute
	DefaultGracePeriod = 10 * time.Second
)

func getTaskTimeout() time.Duration {
	if v := os.Getenv("CUAN_TASK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return DefaultTaskTimeout
}

func getGracePeriod() time.Duration {
	if v := os.Getenv("CUAN_TASK_GRACE_PERIOD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return DefaultGracePeriod
}

var ErrActive = errors.New("a task process still owns this execution")

// OpenLock never truncates or unlinks a live lock. All hosts for this local
// runner must use the same persistent lock directory.
func OpenLock(path string) (*os.File, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("task lock path must be absolute")
	}
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
func Acquire(path string) (*os.File, error) {
	file, err := OpenLock(path)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrActive
		}
		return nil, err
	}
	return file, nil
}
func Active(path string) (bool, error) {
	file, err := Acquire(path)
	if errors.Is(err, ErrActive) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	file.Close()
	return false, nil
}

// Run is a private re-exec entry in the same application binary. It keeps the
// lock independently of provider FD handling, forwards terminal shutdown, and
// kills its agent if its own parent disappears. Arguments are never shell text.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 4 || args[2] != "--" {
		fmt.Fprintln(stderr, "Invalid task supervisor invocation")
		return 2
	}
	expectedParent, parseErr := strconv.Atoi(args[1])
	if parseErr != nil || expectedParent <= 1 || os.Getppid() != expectedParent {
		fmt.Fprintln(stderr, "The task owner process is no longer available.")
		return 74
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	file, err := Acquire(args[0])
	if err != nil {
		if errors.Is(err, ErrActive) {
			fmt.Fprintln(stderr, "The previous task process is still active.")
			return ConflictExit
		}
		fmt.Fprintln(stderr, "Task execution ownership could not be verified.")
		return 74
	}
	defer func() {
		if file != nil {
			_ = file.Close()
		}
	}()

	// Demote task supervisor and all child processes to background CPU priority (nice 19)
	// so agent workloads (compiles, test suites, builds) never starve the main HTTP server or DB.
	_ = unix.Setpriority(unix.PRIO_PROCESS, 0, 19)

	initialParent := os.Getppid()
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	child := exec.Command(args[3], args[4:]...)
	child.Stdin, child.Stdout, child.Stderr = stdin, stdout, stderr
	childEnv := os.Environ()
	hasGoMemLimit, hasGoMaxProcs, hasGoGC, hasNodeOptions := false, false, false, false
	for _, e := range childEnv {
		if strings.HasPrefix(e, "GOMEMLIMIT=") {
			hasGoMemLimit = true
		}
		if strings.HasPrefix(e, "GOMAXPROCS=") {
			hasGoMaxProcs = true
		}
		if strings.HasPrefix(e, "GOGC=") {
			hasGoGC = true
		}
		if strings.HasPrefix(e, "NODE_OPTIONS=") {
			hasNodeOptions = true
		}
	}
	if !hasGoMemLimit {
		childEnv = append(childEnv, "GOMEMLIMIT=400MiB")
	}
	if !hasGoMaxProcs {
		childEnv = append(childEnv, "GOMAXPROCS=1")
	}
	if !hasGoGC {
		childEnv = append(childEnv, "GOGC=50")
	}
	if !hasNodeOptions {
		childEnv = append(childEnv, "NODE_OPTIONS=--max-old-space-size=512")
	}
	child.Env = childEnv
	// The agent cannot survive an abruptly killed supervisor. LockOSThread keeps
	// the Linux parent-death binding tied to this live supervisor thread.
	child.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL, Setpgid: true}
	// The lock is intentionally not inherited by provider children: a deployed
	// service may outlive the task without retaining task execution ownership.
	if err := child.Start(); err != nil {
		fmt.Fprintln(stderr, "The task agent could not start.")
		return 127
	}
	_ = unix.Setpriority(unix.PRIO_PROCESS, child.Process.Pid, 19)
	if tty, ok := stdin.(*os.File); ok {
		// Hand the controlling PTY to the agent's own group. The supervisor can
		// then terminate the full group without killing itself or losing lock
		// ownership before its agent has exited.
		signal.Ignore(syscall.SIGTTOU)
		ttyErr := unix.IoctlSetPointerInt(int(tty.Fd()), unix.TIOCSPGRP, child.Process.Pid)
		signal.Reset(syscall.SIGTTOU)
		if ttyErr != nil && !errors.Is(ttyErr, syscall.ENOTTY) {
			_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
			_ = child.Wait()
			fmt.Fprintln(stderr, "The task terminal could not be prepared.")
			return 74
		}
		_ = syscall.Kill(-child.Process.Pid, syscall.SIGCONT)
	}
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	parentCheck := time.NewTicker(200 * time.Millisecond)
	defer parentCheck.Stop()

	taskTimeout := getTaskTimeout()
	timeoutTimer := time.NewTimer(taskTimeout)
	defer timeoutTimer.Stop()

	var timedOut bool
	var timeoutStopping <-chan time.Time
	var timeoutGraceTimer *time.Timer
	defer func() {
		if timeoutGraceTimer != nil {
			timeoutGraceTimer.Stop()
		}
	}()

	var stopping <-chan time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	stop := func(sig os.Signal) {
		if stopping != nil {
			return
		}
		_ = syscall.Kill(-child.Process.Pid, sig.(syscall.Signal))
		timer = time.NewTimer(100 * time.Millisecond)
		stopping = timer.C
	}
	for {
		select {
		case <-waited:
			code := child.ProcessState.ExitCode()
			if stopping != nil || timeoutStopping != nil || timedOut || code != 0 {
				_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
			}
			if timedOut {
				// Cleanly release lock file before exiting on timeout
				if file != nil {
					_ = file.Close()
					file = nil
				}
				return 124
			}
			if code < 0 {
				if status, ok := child.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
					return 128 + int(status.Signal())
				}
				return 1
			}
			return code
		case sig := <-signals:
			stop(sig)
		case <-parentCheck.C:
			if os.Getppid() != initialParent {
				stop(syscall.SIGTERM)
			}
		case <-timeoutTimer.C:
			if !timedOut && stopping == nil {
				timedOut = true
				fmt.Fprintln(stderr, "Task execution exceeded hard timeout.")
				// Stage 1: Send graceful SIGTERM to the process tree (pgid)
				_ = syscall.Kill(-child.Process.Pid, syscall.SIGTERM)
				grace := getGracePeriod()
				timeoutGraceTimer = time.NewTimer(grace)
				timeoutStopping = timeoutGraceTimer.C
			}
		case <-timeoutStopping:
			// Stage 2: Grace period expired, escalate to SIGKILL
			_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
			timeoutStopping = nil
		case <-stopping:
			_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
			stopping = nil
		}
	}
}

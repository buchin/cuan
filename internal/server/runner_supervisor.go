package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/runner"
)

// RunnerSupervisor monitors the availability of task worker daemons (cuan-runner).
// In production with systemd, an external cuan-runner service sends regular heartbeats,
// so this supervisor remains dormant. In standalone, dev, Docker, or Instabox environments
// without a managed service, this supervisor automatically launches and supervises
// cuan-runner (or an embedded runner) to ensure task queues are never starved.
type RunnerSupervisor struct {
	server      *Server
	cfg         *config.Config
	mu          sync.Mutex
	childCmd    *exec.Cmd
	embeddedRun *runner.Runner
	isRunning   bool
}

// NewRunnerSupervisor creates a new runner supervisor bound to the server instance.
func NewRunnerSupervisor(s *Server) *RunnerSupervisor {
	return &RunnerSupervisor{
		server: s,
		cfg:    s.cfg,
	}
}

// Start launches the supervisor loop in the background.
func (rs *RunnerSupervisor) Start(ctx context.Context) {
	if rs.cfg == nil || !rs.cfg.RunnerAutoSupervise {
		log.Println("[RunnerSupervisor] Auto-runner supervision disabled by configuration.")
		return
	}

	go rs.superviseLoop(ctx)
}

func (rs *RunnerSupervisor) superviseLoop(ctx context.Context) {
	log.Printf("[RunnerSupervisor] Started worker supervisor daemon (server: :%s, check interval: 10s)", rs.cfg.ServerPort)

	// Grace period on initial startup: allow 5 seconds for existing external workers (e.g. systemd) to send heartbeat
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Initial check
	rs.checkAndHeal(ctx)

	for {
		select {
		case <-ctx.Done():
			rs.stopChild()
			return
		case <-ticker.C:
			rs.checkAndHeal(ctx)
		}
	}
}

// isExternalRunnerAlive checks if any runner has checked in recently (< 30 seconds).
func (rs *RunnerSupervisor) isExternalRunnerAlive(ctx context.Context) bool {
	if rs.server == nil || rs.server.runnerSvc == nil {
		return false
	}

	runners, err := rs.server.runnerSvc.ListRunners(ctx)
	if err != nil || len(runners) == 0 {
		return false
	}

	threshold := 30 * time.Second
	now := time.Now().UTC()

	for _, rn := range runners {
		// Do not confuse our own supervised child's lingering heartbeat with an external runner
		if rn.ID == "cuan-runner" {
			continue
		}
		if !rn.LastHeartbeatAt.IsZero() && now.Sub(rn.LastHeartbeatAt) <= threshold {
			return true
		}
	}

	return false
}

// checkAndHeal ensures that at least one task runner daemon is active.
func (rs *RunnerSupervisor) checkAndHeal(ctx context.Context) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// 1. If we already have a child process running and alive, keep monitoring
	if rs.childCmd != nil && rs.childCmd.Process != nil && rs.isRunning {
		return
	}

	// 2. Check if an external runner is actively sending heartbeats
	if rs.isExternalRunnerAlive(ctx) {
		return
	}

	// 3. No runner is alive — self-heal by starting cuan-runner
	rs.startRunnerLocked(ctx)
}

// resolveRunnerBinary looks for cuan-runner binary candidates.
func (rs *RunnerSupervisor) resolveRunnerBinary() string {
	bin := runner.ResolveRunnerBinary(rs.cfg)
	if bin != "" {
		if fi, err := os.Stat(bin); err == nil && !fi.IsDir() {
			return bin
		}
		if looked, err := exec.LookPath(bin); err == nil {
			return looked
		}
	}
	return ""
}

// startRunnerLocked starts cuan-runner as a child process or in-process fallback.
func (rs *RunnerSupervisor) startRunnerLocked(ctx context.Context) {
	binPath := rs.resolveRunnerBinary()
	serverURL := fmt.Sprintf("http://127.0.0.1:%s", rs.cfg.ServerPort)
	if rs.cfg.RunnerServerURL != "" && !strings.Contains(rs.cfg.RunnerServerURL, "localhost") {
		serverURL = rs.cfg.RunnerServerURL
	}

	token := rs.cfg.AuthSecret
	if rs.cfg.RunnerToken != "" {
		token = rs.cfg.RunnerToken
	}

	if binPath != "" {
		log.Printf("[RunnerSupervisor] No active worker daemon detected. Auto-spawning cuan-runner from %s...", binPath)
		args := []string{
			"--server-url=" + serverURL,
			"--runner-token=" + token,
			"--runner-id=cuan-runner",
		}

		cmd := exec.CommandContext(context.Background(), binPath, args...)
		cmd.Env = os.Environ()
		configureSupervisorChildProcAttr(cmd)

		// Write runner logs to temp file
		if logFile, err := os.OpenFile(filepath.Join(os.TempDir(), "cuan-auto-runner.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}

		if err := cmd.Start(); err != nil {
			log.Printf("[RunnerSupervisor] Failed to start cuan-runner child: %v, attempting in-process fallback...", err)
			rs.startEmbeddedRunnerLocked(ctx)
			return
		}

		rs.childCmd = cmd
		rs.isRunning = true

		go func() {
			err := cmd.Wait()
			rs.mu.Lock()
			rs.isRunning = false
			rs.childCmd = nil
			rs.mu.Unlock()
			log.Printf("[RunnerSupervisor] Supervised cuan-runner child process exited: %v", err)
		}()
		return
	}

	// Fallback to in-process embedded runner if binary is not yet compiled on disk
	rs.startEmbeddedRunnerLocked(ctx)
}

func (rs *RunnerSupervisor) startEmbeddedRunnerLocked(ctx context.Context) {
	if rs.embeddedRun != nil {
		return
	}

	adapter, err := agent.NewAGYAdapter("")
	if err != nil {
		log.Printf("[RunnerSupervisor] In-process runner unavailable: %v", err)
		return
	}

	log.Printf("[RunnerSupervisor] Starting embedded in-process runner daemon (adapter: %s)...", adapter.Name())
	r := runner.NewRunner(rs.cfg, adapter)
	rs.embeddedRun = r

	go func() {
		if err := r.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[RunnerSupervisor] Embedded runner terminated: %v", err)
		}
	}()
}

func (rs *RunnerSupervisor) stopChild() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.childCmd != nil && rs.childCmd.Process != nil && rs.isRunning {
		log.Println("[RunnerSupervisor] Gracefully terminating supervised cuan-runner child...")
		_ = rs.childCmd.Process.Signal(syscall.SIGTERM)

		done := make(chan struct{})
		go func() {
			for i := 0; i < 30; i++ {
				rs.mu.Lock()
				running := rs.isRunning
				rs.mu.Unlock()
				if !running {
					close(done)
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = rs.childCmd.Process.Kill()
		}
		rs.isRunning = false
		rs.childCmd = nil
	}
}

// CancelAttempt signals the active in-process runner to terminate a running attempt.
func (rs *RunnerSupervisor) CancelAttempt(attemptID string) {
	rs.mu.Lock()
	r := rs.embeddedRun
	rs.mu.Unlock()
	if r != nil {
		r.CancelSpecificAttempt(attemptID)
	}
}

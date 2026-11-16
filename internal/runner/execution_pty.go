package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

// executeWithBrains spawns agy inside a native pseudo-terminal (creack/pty) and binds
// Unix domain sockets for decoupled streaming to cuan-server.
// Finish one process before switching brains. Resume the same task and workspace,
// never a native provider session or a fresh task, and reconcile partial effects.
func (r *Runner) executeWithBrains(ctx context.Context, claim ClaimTaskResponse, t *task.Task, att *task.TaskAttempt, prompt, workingDir string, env map[string]string, timeout time.Duration, count *int64) (*agent.ExecutionResult, error) {
	if strings.HasPrefix(t.Title, "Terminal ·") && timeout < 24*time.Hour {
		timeout = 24 * time.Hour
	}
	if claim.OwnerID == "" {
		return r.executeWithPTY(ctx, claim, t, att, prompt, workingDir, env, timeout, count)
	}
	routed, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	r.mu.Lock()
	if act := r.activeAttempts[att.ID]; act != nil {
		act.cancel = cancel
	}
	r.mu.Unlock()
	tried := map[string]bool{}
	for len(tried) < 16 && routed.Err() == nil {
		home, brain, release, err := agent.LeaseHealthyBrainForTask(claim.OwnerID, tried, claim.RequiresVision)
		if err != nil {
			return &agent.ExecutionResult{Status: "QUOTA_LIMITED", ExitCode: 75, Error: agent.ErrBrainCapacity.Error()}, nil
		}
		activeModel := brain.ExecutionModelID()
		tried[agent.BrainRouteKey(brain.ID, activeModel)] = true
		_ = agent.PrepareAntigravitySettings(home, workingDir)
		_ = agent.EnsureAntigravityOnboarding(home)
		childEnv := map[string]string{}
		for key, value := range env {
			childEnv[key] = value
		}
		for _, item := range agent.AntigravityProfileEnvironment(home) {
			if key, value, ok := strings.Cut(item, "="); ok {
				childEnv[key] = value
			}
		}
		childEnv["CUAN_BRAIN_MODEL"] = activeModel
		nextPrompt := prompt
		if len(tried) > 1 {
			nextPrompt = "Continue the SAME approved task after an AI capacity interruption. Inspect existing files, git state, saved task reports and external effects FIRST. Preserve completed work. Reconcile uncertain operations before repeating them; do not repeat deployments, messages or other external writes without verification. Perform only unfinished original scope.\n\n" + prompt
			r.streamLogLine(routed, att.ID, "system", "Continuing saved work automatically with available AI capacity.")
		}
		result, runErr := func() (*agent.ExecutionResult, error) {
			defer release()
			return r.executeWithPTY(routed, claim, t, att, nextPrompt, workingDir, childEnv, timeout, count)
		}()
		if runErr != nil || result == nil || result.Status != "QUOTA_LIMITED" || routed.Err() != nil {
			return result, runErr
		}
		duration, _, _ := agent.ParseProviderQuotaResetDuration(result.Error)
		if err := agent.RecordProviderModelQuotaWithDuration(claim.OwnerID, brain.ID, activeModel, duration); err != nil {
			return nil, fmt.Errorf("could not retain AI capacity cooldown")
		}
	}
	if routed.Err() != nil {
		return nil, routed.Err()
	}
	return &agent.ExecutionResult{Status: "QUOTA_LIMITED", ExitCode: 75, Error: agent.ErrBrainCapacity.Error()}, nil
}

func (r *Runner) executeWithPTY(
	ctx context.Context,
	claim ClaimTaskResponse,
	t *task.Task,
	att *task.TaskAttempt,
	prompt, workingDir string,
	env map[string]string,
	timeoutDuration time.Duration,
	liveStreamedCount *int64,
) (*agent.ExecutionResult, error) {
	agyBin, err := agent.ResolveAGYBinaryPath()
	if err != nil || agyBin == "" {
		return nil, fmt.Errorf("resolve agy binary: %w", err)
	}

	execCtx, cancelExec := context.WithTimeout(ctx, timeoutDuration)
	defer cancelExec()

	r.mu.Lock()
	if act, ok := r.activeAttempts[att.ID]; ok && claim.OwnerID == "" {
		act.cancel = cancelExec
	}
	r.mu.Unlock()

	ptyArgs := []string{
		"--dangerously-skip-permissions",
		"--add-dir", workingDir,
	}
	if !strings.HasPrefix(t.Title, "Terminal ·") && prompt != "" {
		ptyArgs = append(ptyArgs, "--prompt-interactive", prompt)
	}
	diagnosticDir, err := os.MkdirTemp("", "cuan-provider-log-")
	if err != nil {
		if errors.Is(err, syscall.ENOSPC) {
			return nil, errors.New(StorageInterruptionMessage)
		}
		return nil, fmt.Errorf("prepare private provider diagnostics: %w", err)
	}
	defer os.RemoveAll(diagnosticDir)
	deadline, _ := execCtx.Deadline()
	hookRoot, hookEnv, hookErr := prepareAGYReportHook(diagnosticDir, env, deadline)
	if hookErr != nil {
		return nil, hookErr
	}
	env = hookEnv
	if hookRoot != "" {
		ptyArgs = append(ptyArgs, "--add-dir", hookRoot)
	}
	diagnosticPath := filepath.Join(diagnosticDir, "provider.log")
	ptyArgs = append(ptyArgs, "--log-file", diagnosticPath)

	if model := env["CUAN_BRAIN_MODEL"]; model != "" {
		ptyArgs = append(ptyArgs, "--model", model)
	}

	ptyEnv := []string{
		"SSH_CONNECTION=127.0.0.1 22 127.0.0.1 22",
		"SSH_CLIENT=127.0.0.1 22 22",
		"SSH_TTY=/dev/pts/0",
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	}
	for k, v := range env {
		ptyEnv = append(ptyEnv, fmt.Sprintf("%s=%s", k, v))
	}

	proc, err := terminal.SpawnPTY(execCtx, terminal.PTYConfig{
		TaskID:       t.ID,
		WorkDir:      workingDir,
		Shell:        agyBin,
		Args:         ptyArgs,
		Env:          ptyEnv,
		DelegatedEnv: project.ParseEnvironment(claim.Environment),
		InitialCols:  80,
		InitialRows:  24,
	})
	if err != nil {
		return nil, fmt.Errorf("spawn pty: %w", err)
	}
	defer proc.Close()

	sessionID := "runner-sess-" + att.ID
	sess := terminal.NewSession(sessionID, t.ID, proc, terminal.ModeInteractive)
	defer sess.Close()

	// Expose Unix domain socket bridge for both task ID and attempt ID
	bridgeTask, _ := terminal.NewSocketBridge(sess, terminal.SocketPath(t.ID))
	if bridgeTask != nil {
		defer bridgeTask.Close()
	}
	bridgeAtt, _ := terminal.NewSocketBridge(sess, terminal.SocketPath(att.ID))
	if bridgeAtt != nil {
		defer bridgeAtt.Close()
	}

	// Register steering function
	steerFunc := func(steerPrompt string) error {
		_, wErr := sess.WriteInput("steer", []byte(steerPrompt+"\r"))
		return wErr
	}

	r.mu.Lock()
	var steerCh <-chan string
	if act, ok := r.activeAttempts[att.ID]; ok {
		act.steerFunc = steerFunc
		steerCh = act.steerChan
	}
	r.mu.Unlock()

	if steerCh != nil {
		go func() {
			for {
				select {
				case <-execCtx.Done():
					return
				case p, ok := <-steerCh:
					if !ok {
						return
					}
					_ = steerFunc(p)
				}
			}
		}()
	}

	// Subscribe logger to feed database logs in real time. A fast process may exit
	// before this point; that is completion, not a failure, so retain its output.
	logSubID := "logger-" + att.ID
	logSub, history, subscribeErr := sess.Subscribe(logSubID, terminal.ModeWatch)
	if subscribeErr != nil {
		if !errors.Is(subscribeErr, terminal.ErrSessionClosed) {
			return nil, fmt.Errorf("subscribe task output: %w", subscribeErr)
		}
		early := &agent.ExecutionResult{Status: "SUCCESS", ExitCode: proc.ExitCode()}
		if early.ExitCode != 0 {
			early.Status = "FAILED"
			early.Error = fmt.Sprintf("exit code %d", early.ExitCode)
		}
		if output := sess.Scrollback(); len(output) > 0 {
			probe := &ptyDiagnosticCollector{}
			lines, _ := probe.Feed(output)
			if len(lines) > 0 {
				r.streamOutputLogs(ctx, att.ID, strings.Join(lines, "\n"), "")
			}
		}
		return early, nil
	}
	defer sess.Unsubscribe(logSubID)

	var quotaHit atomic.Bool
	var quotaReset string
	var diagnostics ptyDiagnosticCollector
	var logMu sync.Mutex
	// Progress watchdog: the attempt stops when the agent stops producing work,
	// not when a fixed wall-clock guess expires. The execution deadline remains a
	// hard ceiling so a genuinely runaway process still ends.
	var lastProgress atomic.Int64
	lastProgress.Store(time.Now().UnixNano())
	lastProgressActivity := func() {
		lastProgress.Store(time.Now().UnixNano())
	}
	var stalled atomic.Bool
	persistLines := func(logCtx context.Context, lines []string) {
		if len(lines) == 0 {
			return
		}
		atomic.AddInt64(liveStreamedCount, int64(len(lines)))
		// One request per output batch instead of one per terminal redraw.
		r.streamOutputLogs(logCtx, att.ID, strings.Join(lines, "\n"), "")
	}
	consumeOutput := func(chunk []byte) {
		logMu.Lock()
		defer logMu.Unlock()
		lines, quota := diagnostics.Feed(chunk)
		if diagnostics.Progressed() {
			lastProgressActivity()
		}
		if quota != "" && !quotaHit.Swap(true) {
			if _, reset, ok := agent.ParseProviderQuotaResetDuration(quota); ok {
				quotaReset = reset
			}
			r.streamLogLine(ctx, att.ID, "system", "AI capacity limit detected. Preserving work before selecting another available route.")
			cancelExec()
			_ = sess.Close()
		}
		persistLines(ctx, lines)
	}
	defer func() {
		logMu.Lock()
		defer logMu.Unlock()
		// The execution deadline must stop the agent, not discard its final
		// diagnostic fragment. This separate budget grants no execution time.
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		persistLines(flushCtx, diagnostics.Flush())
	}()
	// Stop a stalled attempt early instead of waiting for the wall-clock ceiling.
	// A working agent keeps refreshing progress, so silence is the real signal.
	// Start only after the output subscription exists: a very short fixture can
	// finish before this point, and that is completion, not a stall.
	watchdogDone := make(chan struct{})
	startWatchdog := func() {
		go func() {
			defer close(watchdogDone)
			idle := task.ProgressIdleTimeout()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-execCtx.Done():
					return
				case <-ticker.C:
					// A finished agent is not a stalled one; redraw animations are
					// excluded from progress, so check liveness before acting.
					if sess.IsClosed() {
						return
					}
					silent := time.Since(time.Unix(0, lastProgress.Load()))
					if silent < idle {
						continue
					}
					stalled.Store(true)
					r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("No new work output for %v; stopping this attempt so saved progress can continue on the next one.", silent.Round(time.Second)))
					cancelExec()
					_ = sess.Close()
					return
				}
			}
		}()
	}

	consumeOutput(history)
	startWatchdog()
	defer func() {
		cancelExec()
		<-watchdogDone
	}()
	loggerDone := make(chan struct{})
	go func() {
		defer close(loggerDone)
		for {
			select {
			case <-logSub.CloseChan:
				if !sess.IsClosed() {
					r.streamLogLine(ctx, att.ID, "system", "Diagnostic log viewer fell behind and detached. Retained output may be incomplete; live execution continues.")
				}
				for {
					select {
					case chunk := <-logSub.SendChan:
						consumeOutput(chunk)
					default:
						return
					}
				}
			case <-execCtx.Done():
				return
			case chunk, ok := <-logSub.SendChan:
				if !ok {
					return
				}
				consumeOutput(chunk)
			}
		}
	}()

	// Only a runner-initiated reported stop may normalize a signal exit.
	// A final report alone cannot turn a genuinely failed process into success.
	var reportedStop atomic.Bool
	var stoppedReport atomic.Value
	var reportMu sync.Mutex
	reportState := func() string {
		reportMu.Lock()
		defer reportMu.Unlock()
		return r.finalAgentReportState(execCtx, claim)
	}
	reportReady := func() bool {
		state := reportState()
		return state == task.TaskStatusReview || state == task.TaskStatusDone
	}
	// Monitor agent task report completion in background
	reportCheckTicker := time.NewTicker(500 * time.Millisecond)
	defer reportCheckTicker.Stop()

	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		for {
			select {
			case <-execCtx.Done():
				return
			case <-reportCheckTicker.C:
				if state := reportState(); state != "" {
					// Drain final output, but do not stop a newer progress turn.
					drain := time.NewTimer(1500 * time.Millisecond)
					select {
					case <-execCtx.Done():
						drain.Stop()
						return
					case <-drain.C:
					}
					if current := reportState(); current == "" || current != state {
						continue
					}
					if sess.IsClosed() {
						return
					}
					stoppedReport.Store(state)
					reportedStop.Store(true)
					_ = sess.Close()
					return
				}
			}
		}
	}()

	defer func() { cancelExec(); <-doneChan }()

	// Wait for PTY process completion or context cancellation
	waitErr := proc.Wait()
	exitCode := proc.ExitCode()
	// Give the existing reader a bounded opportunity to persist the final batch.
	select {
	case <-loggerDone:
	case <-time.After(2 * time.Second):
	}

	res := &agent.ExecutionResult{
		Status:   "SUCCESS",
		ExitCode: exitCode,
		Duration: timeoutDuration,
	}
	finalState := ""
	if reportedStop.Load() || exitCode == 0 {
		finalState = reportState()
	}
	if exitCode != 0 {
		// Preserve real process failures, even when a final report was saved.
		if reportedStop.Load() && reportedStopExit(exitCode) && finalState != "" && stoppedReport.Load() == finalState {
			r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Confirmed final report; runner closed the terminal (native exit %d).", exitCode))
			res.Status = "SUCCESS"
			res.ExitCode = 0
			res.Error = ""
		} else {
			res.Status = "FAILED"
			res.Error = fmt.Sprintf("exit code %d", exitCode)
		}
	}
	if res.Status == "SUCCESS" && finalState == task.TaskStatusBlocked {
		res.Status = "BLOCKED"
		res.Error = "Agent reported a blocker; saved findings require review."
	}
	if waitErr != nil && stalled.Load() {
		// The agent stopped producing work before the ceiling was reached. Report
		// the real cause so the retry can continue the saved progress.
		res.Status = "STALLED"
		res.Error = "The agent stopped producing work; saved progress is preserved for the next attempt."
	} else if waitErr != nil && errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		res.Status = "TIMEOUT"
		res.Error = "execution timed out"
	} else if waitErr != nil && errors.Is(execCtx.Err(), context.Canceled) {
		res.Status = "CANCELED"
		res.Error = "execution canceled"
	}

	if res.Status == "BLOCKED" {
		return res, nil
	}

	if quotaHit.Load() && ctx.Err() == nil {
		if (exitCode == 0 || (reportedStop.Load() && reportedStopExit(exitCode))) && reportReady() {
			res.Status = "SUCCESS"
			res.ExitCode = 0
			res.Error = ""
			return res, nil
		}
		res.Status = "QUOTA_LIMITED"
		res.ExitCode = 75
		res.Error = "Provider quota is limited."
		logMu.Lock()
		if quotaReset != "" {
			res.Error += " Resets in " + quotaReset + "."
		}
		logMu.Unlock()
	}
	if res.Status == "FAILED" && ctx.Err() == nil && providerStorageExhausted(diagnosticPath, env["HOME"]) {
		res.Status = "INTERRUPTED"
		res.Error = StorageInterruptionMessage
	}
	return res, nil
}

func reportedStopExit(code int) bool {
	return code == -1 || code == 128+int(syscall.SIGTERM) || code == 128+int(syscall.SIGKILL)
}

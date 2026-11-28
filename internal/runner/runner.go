package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

// DefaultExecutionOutputFormat configures the runner adapter for streaming chunks.
// OutputFormat: "stream-json"
const DefaultExecutionOutputFormat = "stream-json"

type activeAttempt struct {
	attempt        *task.TaskAttempt
	lane           string
	cancel         context.CancelFunc
	extendDeadline func(time.Time)
	steerFunc      func(prompt string) error
	steerChan      chan string
	detached       bool
	pid            int
	lastActivity   atomic.Int64
}

type Runner struct {
	completionMu     sync.Mutex
	browserMu        sync.Mutex
	browserEvidence      *task.VerificationRuntime
	browserNextProbe     time.Time
	browserLastFullProbe time.Time
	browserProbing       bool
	cfg              *config.Config
	adapter          agent.Adapter
	gitClient        *git.Client
	httpClient       *http.Client
	baseURL          string

	// Multi-Lane Concurrency Mutexes (Pemisahan Mutex Runner)
	gitDeployMu sync.Mutex   // Code lane: git operations, builds, deploys
	opsMu       sync.Mutex   // Ops lane: non-blocking operational tasks (artisan, sync, etc.)
	readersMu   sync.RWMutex // Readonly lane: shared reader pool

	mu             sync.Mutex // Protects active attempts registry & backward-compatible state
	activeAttempts map[string]*activeAttempt
	currentAttempt *task.TaskAttempt
	cancelAttempt  context.CancelFunc

	drainCoord *DrainCoordinator
	updater    *UpdateManager

	dodEvaluator *DoDEvaluator
	eventBus     *EventBus

	// Detached Worker Registry & Launcher (Pola #1: Detached Worker Process)
	registry *WorkerRegistry
	launcher *DetachedLauncher

	// Task Affinity for run-to-completion active task bundles (Implementer -> Verifier)
	preferredTaskID string
	preferredUntil  time.Time
}

func NewRunner(cfg *config.Config, adapter agent.Adapter, customHTTPClient ...*http.Client) *Runner {
	client := http.DefaultClient
	if len(customHTTPClient) > 0 && customHTTPClient[0] != nil {
		client = customHTTPClient[0]
	}

	baseURL := strings.TrimRight(cfg.RunnerServerURL, "/")

	registryDir := cfg.RunnerWorkerRegistryDir
	if registryDir == "" {
		registryDir = filepath.Join(os.TempDir(), "cuan-workers")
	}
	registry := NewWorkerRegistry(registryDir)
	launcher := NewDetachedLauncher(registry)

	return &Runner{
		cfg:            cfg,
		adapter:        adapter,
		gitClient:      git.NewClient(),
		httpClient:     client,
		baseURL:        baseURL,
		activeAttempts: make(map[string]*activeAttempt),
		drainCoord:     NewDrainCoordinator(""),
		updater:        NewUpdateManager(""),
		dodEvaluator:   NewDoDEvaluator(),
		eventBus:       GlobalEventBus,
		registry:       registry,
		launcher:       launcher,
	}
}

func (r *Runner) SetDoDEvaluator(eval *DoDEvaluator) {
	if eval != nil {
		r.dodEvaluator = eval
	}
}

func (r *Runner) DoDEvaluator() *DoDEvaluator {
	return r.dodEvaluator
}

func (r *Runner) SetEventBus(bus *EventBus) {
	if bus != nil {
		r.eventBus = bus
	}
}

func (r *Runner) EventBus() *EventBus {
	return r.eventBus
}

func (r *Runner) SetGitClient(client *git.Client) {
	r.gitClient = client
}

func (r *Runner) SetDrainCoordinator(dc *DrainCoordinator) {
	if dc != nil {
		r.drainCoord = dc
	}
}

func (r *Runner) SetUpdateManager(um *UpdateManager) {
	if um != nil {
		r.updater = um
	}
}

func (r *Runner) DrainCoordinator() *DrainCoordinator {
	return r.drainCoord
}

func (r *Runner) UpdateManager() *UpdateManager {
	return r.updater
}

func (r *Runner) SetWorkerRegistry(reg *WorkerRegistry) {
	if reg != nil {
		r.registry = reg
	}
}

func (r *Runner) WorkerRegistry() *WorkerRegistry {
	return r.registry
}

func (r *Runner) SetDetachedLauncher(l *DetachedLauncher) {
	if l != nil {
		r.launcher = l
	}
}

func (r *Runner) DetachedLauncher() *DetachedLauncher {
	return r.launcher
}

func (r *Runner) RequestDrain(reason string) {
	if r.drainCoord != nil {
		r.drainCoord.RequestDrain(reason)
	}
}

func (r *Runner) CancelDrain() {
	if r.drainCoord != nil {
		r.drainCoord.CancelDrain()
	}
}

func (r *Runner) IsDraining() bool {
	if r.drainCoord != nil {
		return r.drainCoord.IsDraining()
	}
	return false
}

func (r *Runner) DrainReason() string {
	if r.drainCoord != nil {
		return r.drainCoord.DrainReason()
	}
	return ""
}

func (r *Runner) ActiveAttemptsCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.activeAttempts)
}

func (r *Runner) WaitForInFlight(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if r.ActiveAttemptsCount() == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (r *Runner) CheckAndPerformSelfUpdate() bool {
	if r.updater == nil {
		return false
	}
	hasUpdate, newSHA, err := r.updater.CheckForUpdate()
	if err != nil {
		log.Printf("[Runner %s] Self-update check note: %v", r.cfg.RunnerID, err)
		return false
	}
	if !hasUpdate {
		return false
	}
	log.Printf("[Runner %s] New binary detected on disk (SHA: %s). Initiating graceful self-update handover...", r.cfg.RunnerID, newSHA[:12])
	if err := r.updater.PerformSelfUpdate(os.Args, os.Environ()); err != nil {
		log.Printf("[Runner %s] Self-update restart failed: %v", r.cfg.RunnerID, err)
		return false
	}
	return true
}

// Start runs the runner daemon: background heartbeat & queue polling loop.
func (r *Runner) Start(ctx context.Context) error {
	version, err := r.adapter.Version()
	if err != nil {
		return fmt.Errorf("check agent adapter: %w", err)
	}

	hostname, _ := os.Hostname()

	log.Printf("[Runner %s] Started on host '%s' with agent '%s' (ver: %s)", r.cfg.RunnerID, hostname, r.adapter.Name(), version)
	log.Printf("[Runner %s] Outbound server URL: %s, poll interval: %v", r.cfg.RunnerID, r.baseURL, r.cfg.RunnerInterval)

	// Initial workspace GC on runner startup to clean up leftover checkouts from prior crashes or restarts
	if r.gitClient != nil && r.cfg.WorkspacesDir != "" {
		if pruned, _ := r.gitClient.PruneStaleWorkspaces(r.cfg.WorkspacesDir, 24*time.Hour, "", 5); pruned > 0 {
			log.Printf("[Runner %s] Startup GC: pruned %d stale workspace(s)", r.cfg.RunnerID, pruned)
		}
	}

	// Start background drain flag monitor
	if r.drainCoord != nil {
		go r.drainCoord.StartMonitor(ctx, 1*time.Second)
	}

	// Reconcile existing detached workers on startup (surviving supervisor restarts)
	if err := r.ReconcileStartup(ctx); err != nil {
		log.Printf("[Runner %s] Startup worker reconciliation note: %v", r.cfg.RunnerID, err)
	}

	// Immediate initial heartbeat (reports reconciled workers to renew leases)
	r.sendHeartbeat(ctx, hostname)

	// Heartbeat ticker runs in its own goroutine so long poll/claim operations cannot delay heartbeats.
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		heartbeatTicker := time.NewTicker(3 * time.Second)
		defer heartbeatTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatTicker.C:
				r.sendHeartbeat(ctx, hostname)
			}
		}
	}()

	// Queue poll ticker
	pollTicker := time.NewTicker(r.cfg.RunnerInterval)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			<-hbDone
			log.Printf("[Runner %s] Shutdown signaled, initiating graceful supervisor stop...", r.cfg.RunnerID)
			r.RequestDrain("shutdown context canceled")

			r.mu.Lock()
			detachedCount := 0
			inProcessCount := 0
			for _, act := range r.activeAttempts {
				if act.detached {
					detachedCount++
				} else {
					inProcessCount++
				}
			}
			r.mu.Unlock()

			if detachedCount > 0 {
				log.Printf("[Runner %s] Zero-Kill Deploy: %d detached worker(s) continue executing independently without interruption.",
					r.cfg.RunnerID, detachedCount)
			}

			// In-process tasks wait if any non-detached work is running
			if inProcessCount > 0 {
				drained := r.WaitForInFlight(15 * time.Second)
				if !drained {
					log.Printf("[Runner %s] Graceful drain timeout reached for in-process work...", r.cfg.RunnerID)
					r.cancelCurrentExecution()
				}
			} else {
				log.Printf("[Runner %s] Supervisor stopped cleanly.", r.cfg.RunnerID)
			}

			return nil

		case <-pollTicker.C:
			r.pollAndExecute(ctx)
		}
	}
}

func (r *Runner) sendHeartbeat(ctx context.Context, hostname string) {
	go r.replayCompletions(ctx)
	metrics := CollectHostMetrics()

	r.mu.Lock()
	var attemptIDs []string
	var activeProgressIDs []string
	primaryAttemptID := ""
	idleWindow := task.ProgressIdleTimeout()
	for attID, act := range r.activeAttempts {
		attemptIDs = append(attemptIDs, attID)
		if primaryAttemptID == "" {
			primaryAttemptID = attID
		}
		if act != nil {
			if last := act.lastActivity.Load(); last > 0 && time.Since(time.Unix(0, last)) < idleWindow {
				activeProgressIDs = append(activeProgressIDs, attID)
			}
		}
	}
	if primaryAttemptID == "" && r.currentAttempt != nil {
		primaryAttemptID = r.currentAttempt.ID
		attemptIDs = append(attemptIDs, primaryAttemptID)
	}
	r.mu.Unlock()

	for _, pending := range r.pendingCompletionIDs() {
		found := false
		for _, active := range attemptIDs {
			if active == pending {
				found = true
				break
			}
		}
		if !found {
			attemptIDs = append(attemptIDs, pending)
		}
		if primaryAttemptID == "" {
			primaryAttemptID = pending
		}
	}
	reqPayload := HeartbeatRequest{
		BrowserRuntime:           r.browserRuntimeEvidence(ctx),
		DependencyPreparation:    r.phpPreparationEvidence(),
		RunnerID:                 r.cfg.RunnerID,
		Hostname:                 hostname,
		SupportedAgents:          []string{r.adapter.Name()},
		CurrentAttemptID:         primaryAttemptID,
		ActiveAttemptIDs:         attemptIDs,
		ActiveProgressAttemptIDs: activeProgressIDs,
		CPUPercent:               metrics.CPUPercent,
		MemoryUsedBytes:       metrics.MemoryUsedBytes,
		MemoryTotalBytes:      metrics.MemoryTotalBytes,
		DiskUsedBytes:         metrics.DiskUsedBytes,
		DiskTotalBytes:        metrics.DiskTotalBytes,
	}

	resp, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/heartbeat", reqPayload)
	if err != nil {
		log.Printf("[Runner %s] Heartbeat failed: %v", r.cfg.RunnerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var hbResp HeartbeatResponse
		if err := json.NewDecoder(resp.Body).Decode(&hbResp); err == nil {
			if len(hbResp.CanceledAttemptIDs) > 0 {
				for _, cid := range hbResp.CanceledAttemptIDs {
					log.Printf("[Runner %s] Heartbeat received cancellation directive for attempt %s", r.cfg.RunnerID, cid)
					r.cancelSpecificAttempt(cid)
				}
			} else if hbResp.ShouldCancel {
				log.Printf("[Runner %s] Heartbeat received cancellation directive for active attempts", r.cfg.RunnerID)
				r.cancelCurrentExecution()
			}

			// Process interactive steering directives from server
			if len(hbResp.SteeringDirectives) > 0 {
				for _, dir := range hbResp.SteeringDirectives {
					log.Printf("[Runner %s] Heartbeat received steering directive for attempt %s: %s", r.cfg.RunnerID, dir.AttemptID, dir.Prompt)
					if sErr := r.SteerAttempt(dir.AttemptID, dir.Prompt); sErr != nil {
						r.streamLogLine(ctx, dir.AttemptID, "system", fmt.Sprintf("Steering fallback: %v. Arahan dicatat sebagai follow-up.", sErr))
					} else {
						r.streamLogLine(ctx, dir.AttemptID, "system", fmt.Sprintf("Interactive steering injected ke agent (dari %s): %s", dir.Author, dir.Prompt))
					}
				}
			}

			// Apply activity-extended execution deadlines from server
			if len(hbResp.ExtendedDeadlines) > 0 {
				r.mu.Lock()
				for attID, newDeadline := range hbResp.ExtendedDeadlines {
					if act, ok := r.activeAttempts[attID]; ok && act != nil && act.extendDeadline != nil {
						act.extendDeadline(newDeadline)
					}
				}
				r.mu.Unlock()
			}
		}
	}
}

func (r *Runner) recordAttemptActivity(attemptID string) {
	if attemptID == "" {
		return
	}
	r.mu.Lock()
	act := r.activeAttempts[attemptID]
	r.mu.Unlock()
	if act != nil {
		act.lastActivity.Store(time.Now().UnixNano())
	}
}

func (r *Runner) setAttemptDeadlineExtender(attemptID string, fn func(time.Time)) {
	if attemptID == "" {
		return
	}
	r.mu.Lock()
	if act := r.activeAttempts[attemptID]; act != nil {
		act.extendDeadline = fn
	}
	r.mu.Unlock()
}

// SteerAttempt delivers an interactive steering prompt into the stdin of an active agent execution.
func (r *Runner) SteerAttempt(attemptID, prompt string) error {
	r.mu.Lock()
	act, exists := r.activeAttempts[attemptID]
	r.mu.Unlock()

	if !exists || act == nil {
		return fmt.Errorf("attempt %s is not active on runner %s", attemptID, r.cfg.RunnerID)
	}

	if act.steerFunc != nil {
		return act.steerFunc(prompt)
	}
	if act.steerChan != nil {
		select {
		case act.steerChan <- prompt:
			return nil
		default:
			return errors.New("steer channel is full or blocked")
		}
	}

	// For detached worker processes, forward steering prompt via Unix domain socket bridge
	if terminal.SocketExists(attemptID) {
		return terminal.SendSocketInput(attemptID, prompt+"\r")
	}
	if act.attempt != nil && terminal.SocketExists(act.attempt.TaskID) {
		return terminal.SendSocketInput(act.attempt.TaskID, prompt+"\r")
	}

	return errors.New("agent adapter does not support interactive steering")
}

func (r *Runner) pollAndExecute(ctx context.Context) {
	// If runner is in graceful drain mode, reject claiming new tasks
	if r.IsDraining() {
		return
	}

	// Determine available lanes based on active executions
	var availableLanes []string
	r.mu.Lock()
	hasCode := false
	hasOps := false
	readCount := 0
	for _, act := range r.activeAttempts {
		switch act.lane {
		case task.ExecutionCategoryCode:
			hasCode = true
		case task.ExecutionCategoryOps:
			hasOps = true
		case task.ExecutionCategoryReadOnly:
			readCount++
		}
	}
	if !hasCode {
		availableLanes = append(availableLanes, task.ExecutionCategoryCode)
	}
	if !hasOps {
		availableLanes = append(availableLanes, task.ExecutionCategoryOps)
	}
	if readCount < 4 {
		availableLanes = append(availableLanes, task.ExecutionCategoryReadOnly)
	}
	prefTaskID := ""
	if r.preferredTaskID != "" && time.Now().Before(r.preferredUntil) {
		prefTaskID = r.preferredTaskID
	}
	r.mu.Unlock()

	if len(availableLanes) == 0 {
		return
	}

	claimReq := ClaimTaskRequest{
		PreferredTaskID:                  prefTaskID,
		SupportsNativeRelease:            r.cfg.RunnerNativeRelease,
		SupportsNativeVisualVerification: true,
		SupportsCurrentOutcomeInspection: true,
		SupportsEvidenceRevalidation:     true,
		SupportsLiveInspectionRecovery:   true,
		SupportsLiveInspection:           true,
		SupportsVerificationFinalization: true,
		RunnerID:                         r.cfg.RunnerID,
		SupportedAgents:                  []string{r.adapter.Name()},
		AvailableLanes:                   availableLanes,
	}

	resp, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/claim", claimReq)
	if err != nil {
		log.Printf("[Runner %s] Claim poll failed: %v", r.cfg.RunnerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var claimResp ClaimTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&claimResp); err != nil || !claimResp.Claimed || claimResp.Attempt == nil {
		return
	}

	lane := task.NormalizeExecutionCategory(claimResp.Task.ExecutionCategory)
	if lane == "" {
		lane = task.ExecutionCategoryCode
	}

	log.Printf("[Runner %s] Claimed %s task attempt %s (Task: '%s')", r.cfg.RunnerID, lane, claimResp.Attempt.ID, claimResp.Task.Title)

	r.mu.Lock()
	if claimResp.Task != nil {
		if claimResp.Task.ID == r.preferredTaskID {
			// Kept full affinity on active task bundle, extend window
			r.preferredUntil = time.Now().Add(15 * time.Minute)
		} else {
			r.preferredTaskID = claimResp.Task.ID
			r.preferredUntil = time.Now().Add(15 * time.Minute)
		}
	}
	r.mu.Unlock()

	// Acquire specific lane mutex:
	// - code: git deployment mutex (build/git isolated)
	// - ops: operational mutex (non-blocking vs code)
	// - readonly: reader pool RLock (shared concurrent readers)
	switch lane {
	case task.ExecutionCategoryCode:
		r.gitDeployMu.Lock()
	case task.ExecutionCategoryOps:
		r.opsMu.Lock()
	case task.ExecutionCategoryReadOnly:
		r.readersMu.RLock()
	}

	// In Detached Worker mode (Pola #1), spawn an isolated child worker process
	// decoupled from the supervisor's systemd lifecycle.
	if r.cfg.RunnerDetachedWorkers && r.launcher != nil {
		rec, spawnErr := r.launcher.Spawn(ctx, r.cfg, claimResp)
		if spawnErr != nil {
			log.Printf("[Runner %s] Notice: failed to spawn detached worker (%v), falling back to in-process execution", r.cfg.RunnerID, spawnErr)
		} else {
			r.mu.Lock()
			if r.activeAttempts == nil {
				r.activeAttempts = make(map[string]*activeAttempt)
			}
			r.activeAttempts[claimResp.Attempt.ID] = &activeAttempt{
				attempt:  claimResp.Attempt,
				lane:     lane,
				detached: true,
				pid:      rec.PID,
			}
			r.currentAttempt = claimResp.Attempt
			r.mu.Unlock()

			// Monitor detached worker in background until completion
			go r.monitorDetachedWorker(claimResp.Attempt.ID, rec.PID, lane)
			return
		}
	}

	execCtx, cancelFunc := context.WithCancel(ctx)
	steerChan := make(chan string, 16)
	r.mu.Lock()
	if r.activeAttempts == nil {
		r.activeAttempts = make(map[string]*activeAttempt)
	}
	act := &activeAttempt{
		attempt:   claimResp.Attempt,
		lane:      lane,
		cancel:    cancelFunc,
		steerChan: steerChan,
	}
	act.lastActivity.Store(time.Now().UnixNano())
	r.activeAttempts[claimResp.Attempt.ID] = act
	r.currentAttempt = claimResp.Attempt
	r.cancelAttempt = cancelFunc
	r.mu.Unlock()

	// Execute attempt in background goroutine so heartbeat ticker and other lanes continue running concurrently
	go func(claim ClaimTaskResponse, claimedLane string) {
		defer func() {
			switch claimedLane {
			case task.ExecutionCategoryCode:
				r.gitDeployMu.Unlock()
			case task.ExecutionCategoryOps:
				r.opsMu.Unlock()
			case task.ExecutionCategoryReadOnly:
				r.readersMu.RUnlock()
			}

			r.mu.Lock()
			if claim.Task != nil {
				r.preferredTaskID = claim.Task.ID
				r.preferredUntil = time.Now().Add(15 * time.Minute)
			}
			delete(r.activeAttempts, claim.Attempt.ID)
			if r.currentAttempt != nil && r.currentAttempt.ID == claim.Attempt.ID {
				r.currentAttempt = nil
				r.cancelAttempt = nil
				for _, rem := range r.activeAttempts {
					r.currentAttempt = rem.attempt
					r.cancelAttempt = rem.cancel
					break
				}
			}
			remainingInFlight := len(r.activeAttempts)
			r.mu.Unlock()

			// Check for self-update once all in-flight attempts complete
			if remainingInFlight == 0 {
				r.CheckAndPerformSelfUpdate()
			}
		}()

		r.runAttempt(execCtx, claim)
	}(claimResp, lane)
}

// CancelCurrentExecution terminates all actively running attempts on this runner.
func (r *Runner) CancelCurrentExecution() {
	r.cancelCurrentExecution()
}

func (r *Runner) cancelCurrentExecution() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancelAttempt != nil {
		r.cancelAttempt()
	}
	for _, act := range r.activeAttempts {
		if act.cancel != nil {
			act.cancel()
		}
		if act.pid > 0 {
			terminal.TerminateProcessGroup(act.pid)
		}
	}
}

// CancelSpecificAttempt cancels and terminates a specific attempt by ID.
func (r *Runner) CancelSpecificAttempt(attemptID string) {
	r.cancelSpecificAttempt(attemptID)
}

func (r *Runner) cancelSpecificAttempt(attemptID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if act, ok := r.activeAttempts[attemptID]; ok {
		if act.cancel != nil {
			act.cancel()
		}
		if act.pid > 0 {
			terminal.TerminateProcessGroup(act.pid)
		}
	}
	if r.registry != nil {
		if rec, err := r.registry.Get(attemptID); err == nil && rec != nil && rec.PID > 0 {
			terminal.TerminateProcessGroup(rec.PID)
		}
	}
}

func (r *Runner) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	reqURL := r.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CuanRunner/"+r.cfg.RunnerID)
	// Runner authentication via token
	if r.cfg.RunnerToken != "" {
		req.Header.Set("X-Runner-Token", r.cfg.RunnerToken)
		req.Header.Set("Authorization", "Bearer "+r.cfg.RunnerToken)
	}

	return r.httpClient.Do(req)
}


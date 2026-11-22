package runner

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// ReconcileStartup discovers and tracks active detached workers across supervisor restarts.
func (r *Runner) ReconcileStartup(ctx context.Context) error {
	if r.registry == nil {
		return nil
	}

	records, err := r.registry.List()
	if err != nil {
		return err
	}

	for _, rec := range records {
		if r.registry.IsProcessAlive(rec.PID) {
			log.Printf("[Runner %s] Reconciliation: Detached worker PID %d is actively executing attempt %s (Task: '%s', Lane: %s)",
				r.cfg.RunnerID, rec.PID, rec.AttemptID, rec.TaskID, rec.Lane)
			r.registerReconciledWorker(rec)
		} else {
			log.Printf("[Runner %s] Reconciliation: Cleaning up stale worker record for dead PID %d (attempt %s)",
				r.cfg.RunnerID, rec.PID, rec.AttemptID)
			_ = r.registry.Delete(rec.AttemptID)
		}
	}
	return nil
}

func (r *Runner) registerReconciledWorker(rec *DetachedWorkerRecord) {
	lane := task.NormalizeExecutionCategory(rec.Lane)
	if lane == "" {
		lane = task.ExecutionCategoryCode
	}

	// Acquire lane lock to protect against duplicate task claim
	switch lane {
	case task.ExecutionCategoryCode:
		r.gitDeployMu.Lock()
	case task.ExecutionCategoryOps:
		r.opsMu.Lock()
	case task.ExecutionCategoryReadOnly:
		r.readersMu.RLock()
	}

	r.mu.Lock()
	if r.activeAttempts == nil {
		r.activeAttempts = make(map[string]*activeAttempt)
	}
	r.activeAttempts[rec.AttemptID] = &activeAttempt{
		attempt: &task.TaskAttempt{
			ID:     rec.AttemptID,
			TaskID: rec.TaskID,
		},
		lane:     lane,
		detached: true,
		pid:      rec.PID,
	}
	if r.currentAttempt == nil {
		r.currentAttempt = r.activeAttempts[rec.AttemptID].attempt
	}
	r.mu.Unlock()

	// Monitor detached worker in background until it finishes
	go r.monitorDetachedWorker(rec.AttemptID, rec.PID, lane)
}

func (r *Runner) monitorDetachedWorker(attemptID string, pid int, lane string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if !r.registry.IsProcessAlive(pid) {
			break
		}
	}

	// Detached worker process has exited; release lane concurrency lock and update active attempts
	switch lane {
	case task.ExecutionCategoryCode:
		r.gitDeployMu.Unlock()
	case task.ExecutionCategoryOps:
		r.opsMu.Unlock()
	case task.ExecutionCategoryReadOnly:
		r.readersMu.RUnlock()
	}

	r.mu.Lock()
	if act, ok := r.activeAttempts[attemptID]; ok && act.attempt != nil {
		r.preferredTaskID = act.attempt.TaskID
		r.preferredUntil = time.Now().Add(60 * time.Second)
	}
	delete(r.activeAttempts, attemptID)
	if r.currentAttempt != nil && r.currentAttempt.ID == attemptID {
		r.currentAttempt = nil
		for _, rem := range r.activeAttempts {
			r.currentAttempt = rem.attempt
			break
		}
	}
	remaining := len(r.activeAttempts)
	r.mu.Unlock()

	_ = r.registry.Delete(attemptID)

	log.Printf("[Runner %s] Detached worker PID %d for attempt %s finished; lane '%s' released (remaining active: %d).",
		r.cfg.RunnerID, pid, attemptID, lane, remaining)

	// Trigger an immediate poll after detached worker finishes, with a short delay (1.5s) to allow coordinator review to queue the continuation attempt
	go func() {
		time.Sleep(1500 * time.Millisecond)
		r.pollAndExecute(context.Background())
	}()
}

// RunDetachedAttempt executes a claimed attempt inside an isolated detached worker process.
func (r *Runner) RunDetachedAttempt(ctx context.Context, claim ClaimTaskResponse) error {
	if claim.Attempt == nil || claim.Task == nil {
		return fmt.Errorf("invalid claim payload: attempt and task required")
	}

	attID := claim.Attempt.ID
	defer func() {
		if r.registry != nil {
			_ = r.registry.Delete(attID)
		}
	}()

	// Periodic registry heartbeat update
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ticker.C:
				if r.registry != nil {
					_ = r.registry.UpdateHeartbeat(attID)
				}
			}
		}
	}()

	r.runAttempt(ctx, claim)
	return nil
}

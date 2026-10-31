package server

import (
	"context"
	"errors"
	"log"
	"os"
	"syscall"
	"time"
)

// isTaskActiveOnHost checks whether a task has an active, live process on the host.
func (s *Server) isTaskActiveOnHost(taskID string) bool {
	if s.terminalManager == nil {
		return false
	}
	sess, exists := s.terminalManager.GetSession(taskID)
	if !exists || sess == nil || sess.IsClosed() {
		active, err := s.terminalManager.TaskExecutionActive(taskID)
		if err != nil || !active {
			return false
		}
	}
	if sess != nil && sess.PTY() != nil {
		pid := sess.PTY().PID()
		if pid <= 0 {
			return false
		}
		if err := syscall.Kill(pid, 0); err != nil && errors.Is(err, syscall.ESRCH) {
			return false
		}
	}
	return true
}

// ReconcileZombies reconciles zombie task attempts and launches whose OS process is dead
// or has exceeded the hard execution timeout, freeing the project lane automatically.
func (s *Server) ReconcileZombies(ctx context.Context) error {
	timeout := 20 * time.Minute
	if v := os.Getenv("CUAN_TASK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			timeout = d
		}
	}

	if s.terminalManager != nil {
		s.terminalManager.ReconcileZombieSessions(timeout)
	}

	if s.taskSvc != nil {
		if err := s.taskSvc.ReconcileZombieAttempts(ctx, timeout, s.isTaskActiveOnHost); err != nil {
			log.Printf("[ZombieReaper] ReconcileZombieAttempts error: %v", err)
		}
		if err := s.taskSvc.ReconcileOrphanLaunches(ctx, time.Now().UTC()); err != nil {
			log.Printf("[ZombieReaper] ReconcileOrphanLaunches error: %v", err)
		}
	}

	if s.runnerSvc != nil {
		if _, err := s.runnerSvc.ReconcileExpiredLeases(ctx); err != nil {
			log.Printf("[ZombieReaper] ReconcileExpiredLeases error: %v", err)
		}
	}

	s.kickTaskLaunches()
	return nil
}

// StartZombieReaper starts a periodic background worker for zombie task reconciliation.
func (s *Server) StartZombieReaper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.ReconcileZombies(ctx)
			}
		}
	}()
}

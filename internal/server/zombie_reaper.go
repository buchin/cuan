package server

import (
	"context"
	"errors"
	"log"
	"strings"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

// isTaskActiveOnHost checks whether a task has an active, live process on the host.
func (s *Server) isTaskActiveOnHost(taskID string) bool {
	if s.terminalManager == nil {
		return false
	}
	sess, exists := s.terminalManager.GetSession(taskID)
	if !exists || sess == nil || sess.IsClosed() {
		// Fallback check only for dedicated terminal tasks belonging to a project with an active persistent terminal session
		if s.taskSvc != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			t, err := s.taskSvc.Store().GetTask(ctx, taskID)
			cancel()
			if err == nil && t != nil && task.IsDedicatedTerminalTitle(t.Title) {
				targetProjID := t.ProjectID
				if targetProjID == "" && len(t.ProjectIDs) > 0 {
					targetProjID = t.ProjectIDs[0]
				}
				if targetProjID != "" {
					projKey := terminal.ProjectSessionKey(targetProjID)
					if pSess, ok := s.terminalManager.GetSession(projKey); ok && pSess != nil && !pSess.IsClosed() {
						sess = pSess
						exists = true
						s.terminalManager.RegisterTaskAlias(taskID, projKey)
					}
					// Also check if decoupled project worker has an active Unix domain socket!
					if !exists && terminal.ProjectSocketExists(targetProjID) {
						s.terminalManager.RegisterTaskAlias(taskID, projKey)
						return true
					}
				}
			}
		}
		if !exists || sess == nil || sess.IsClosed() {
			if terminal.SocketExists(taskID) {
				return true
			}
			active, err := s.terminalManager.TaskExecutionActive(taskID)
			if err != nil || !active {
				return false
			}
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
	timeout := task.DefaultMaxCapTimeout + task.GetGracePeriod()
	deadlines := map[string]time.Time{}
	var recoveryTasks []*task.Task
	var latestMap map[string]*task.TaskAttempt
	if s.taskSvc != nil {
		if tasks, e := s.taskSvc.RecoveryTasks(ctx); e == nil {
			recoveryTasks = tasks
			latestMap = s.taskSvc.LatestAttemptsMap(ctx, tasks)
			for _, t := range tasks {
				if a := latestMap[t.ID]; a != nil && a.Status == task.AttemptStatusRunning {
					deadlines[t.ID] = task.AttemptDeadline(t, a).Add(task.GetGracePeriod())
				}
			}
		}
	}

	if s.terminalManager != nil {
		s.terminalManager.ReconcileZombieSessions(timeout, deadlines)
	}

	if s.taskSvc != nil {
		if err := s.taskSvc.ReconcileZombieAttemptTasks(ctx, recoveryTasks, timeout, s.isTaskActiveOnHost); err != nil {
			log.Printf("[ZombieReaper] ReconcileZombieAttempts error: %v", err)
		}
		if err := s.taskSvc.ReconcileOrphanLaunches(ctx, time.Now().UTC()); err != nil {
			log.Printf("[ZombieReaper] ReconcileOrphanLaunches error: %v", err)
		}
	}

	// Renew local attempt leases for all live local terminal and project sessions before lease reconciliation
	if s.taskSvc != nil && s.terminalManager != nil {
		for _, sess := range s.terminalManager.ActiveSessions() {
			if sess == nil || sess.IsClosed() {
				continue
			}
			// 1. Regular task history
			for _, h := range s.terminalManager.GetTaskHistory(sess.TaskID()) {
				if h != nil && h.SessionID == sess.ID() && h.AttemptID != "" && h.Status == "running" {
					_ = s.taskSvc.RenewLocalAttemptLease(ctx, h.AttemptID)
				}
			}
			// 2. Project terminal session or aliased tasks
			sessionKey := sess.TaskID()
			for _, taskID := range s.terminalManager.AliasedTasks(sessionKey) {
				if a := latestMap[taskID]; a != nil && a.RunnerID == "local_terminal" && a.Status == task.AttemptStatusRunning {
					_ = s.taskSvc.RenewLocalAttemptLease(ctx, a.ID)
				} else if a == nil {
					if a, err := s.taskSvc.Store().GetLatestAttempt(ctx, taskID); err == nil && a != nil && a.RunnerID == "local_terminal" && a.Status == task.AttemptStatusRunning {
						_ = s.taskSvc.RenewLocalAttemptLease(ctx, a.ID)
					}
				}
			}
		}

		// Renew local attempt leases for project tasks whose persistent worker runs via session or Unix socket bridge.
		// Uses recoveryTasks already queried above to avoid polling projects and tasks tables every 5 seconds.
		for _, t := range recoveryTasks {
			if !strings.HasPrefix(t.Title, "Terminal ·") {
				continue
			}
			projID := t.ProjectID
			if projID == "" && len(t.ProjectIDs) > 0 {
				projID = t.ProjectIDs[0]
			}
			if projID == "" {
				continue
			}
			sessionKey := terminal.ProjectSessionKey(projID)
			sess, hasSess := s.terminalManager.GetSession(sessionKey)
			if (hasSess && sess != nil && !sess.IsClosed()) || terminal.ProjectSocketExists(projID) {
				if a := latestMap[t.ID]; a != nil && a.RunnerID == "local_terminal" && a.Status == task.AttemptStatusRunning {
					_ = s.taskSvc.RenewLocalAttemptLease(ctx, a.ID)
				} else if a == nil {
					if a, err := s.taskSvc.Store().GetLatestAttempt(ctx, t.ID); err == nil && a != nil && a.RunnerID == "local_terminal" && a.Status == task.AttemptStatusRunning {
						_ = s.taskSvc.RenewLocalAttemptLease(ctx, a.ID)
					}
				}
			}
		}
	}

	if s.runnerSvc != nil {
		if _, err := s.runnerSvc.ReconcileExpiredLeases(ctx); err != nil {
			log.Printf("[ZombieReaper] ReconcileExpiredLeases error: %v", err)
		}
	}

	s.evaluateIdleRunnerQueues(ctx)
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

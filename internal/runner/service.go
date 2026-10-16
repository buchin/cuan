package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

type ProjectFinder interface {
	Get(ctx context.Context, id string) (*project.ProjectResponse, error)
}

type RunnerService struct {
	store         Store
	taskStore     task.Store
	projectFinder ProjectFinder
}

func NewService(store Store, taskStore task.Store, projectFinder ...ProjectFinder) *RunnerService {
	var pf ProjectFinder
	if len(projectFinder) > 0 {
		pf = projectFinder[0]
	}
	return &RunnerService{
		store:         store,
		taskStore:     taskStore,
		projectFinder: pf,
	}
}

func (s *RunnerService) SetProjectFinder(f ProjectFinder) {
	s.projectFinder = f
}

// Heartbeat processes heartbeat and lease renewal from a runner daemon.
func (s *RunnerService) Heartbeat(ctx context.Context, req HeartbeatRequest, leaseDuration time.Duration) (*HeartbeatResponse, error) {
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner_id is required")
	}

	info := &RunnerInfo{
		ID:               req.RunnerID,
		Name:             req.RunnerID,
		Hostname:         req.Hostname,
		SupportedAgents:  req.SupportedAgents,
		CurrentAttemptID: req.CurrentAttemptID,
		CPUPercent:       req.CPUPercent,
		MemoryUsedBytes:  req.MemoryUsedBytes,
		MemoryTotalBytes: req.MemoryTotalBytes,
		DiskUsedBytes:    req.DiskUsedBytes,
		DiskTotalBytes:   req.DiskTotalBytes,
	}
	if req.CurrentAttemptID != "" {
		info.Status = StatusBusy
	} else {
		info.Status = StatusIdle
	}

	if err := s.store.RegisterOrUpdateHeartbeat(ctx, info); err != nil {
		return nil, err
	}

	shouldCancel := false
	if req.CurrentAttemptID != "" {
		var err error
		shouldCancel, err = s.store.RenewLease(ctx, req.CurrentAttemptID, req.RunnerID, leaseDuration)
		if err != nil {
			// If attempt not found or canceled, flag shouldCancel
			shouldCancel = true
		}
	}

	return &HeartbeatResponse{
		Status:           "ok",
		LeaseDurationSec: int(leaseDuration.Seconds()),
		ShouldCancel:     shouldCancel,
	}, nil
}

// Claim attempts to acquire the next queued task attempt for the runner.
func (s *RunnerService) Claim(ctx context.Context, req ClaimTaskRequest, leaseDuration time.Duration) (*ClaimTaskResponse, error) {
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner_id is required")
	}

	att, t, err := s.store.ClaimNextQueuedAttempt(ctx, req.RunnerID, req.SupportedAgents, leaseDuration)
	if err != nil {
		return nil, fmt.Errorf("claim attempt: %w", err)
	}

	if att == nil || t == nil {
		return &ClaimTaskResponse{Claimed: false}, nil
	}

	var leaseExpires time.Time
	if att.LeaseExpiresAt != nil {
		leaseExpires = *att.LeaseExpiresAt
	}

	resp := &ClaimTaskResponse{
		Claimed:      true,
		Attempt:      att,
		Task:         t,
		LeaseExpires: leaseExpires,
	}

	if s.projectFinder != nil && t.ProjectID != "" {
		if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
			resp.ProjectRepo = fmt.Sprintf("%s/%s", p.RepoOwner, p.RepoName)
			resp.RepoOwner = p.RepoOwner
			resp.RepoName = p.RepoName
			resp.RepoURL = p.RepoURL
			resp.DefaultBranch = p.DefaultBranch
			resp.WorkingDir = p.LocalPath
		}
	}

	return resp, nil
}

// Complete handles completion report from a runner daemon.
func (s *RunnerService) Complete(ctx context.Context, attemptID string, req CompleteAttemptRequest) (*task.TaskAttempt, error) {
	att, err := s.taskStore.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	att.FinishedAt = &now
	att.PeakRSSBytes = req.PeakRSSBytes
	att.ExitCode = &req.ExitCode

	status := req.Status
	if req.ExitCode != 0 && status != task.AttemptStatusCanceled {
		status = task.AttemptStatusFailed
	}
	att.Status = status
	att.ErrorMessage = req.ErrorMessage

	if err := s.taskStore.UpdateAttempt(ctx, att); err != nil {
		return nil, fmt.Errorf("update attempt on complete: %w", err)
	}

	// Update task status:
	// CRITICAL DoD Rule: If attempt succeeded -> task to review (owner reviews).
	// If failed -> task to blocked.
	// If canceled -> task to blocked.
	t, err := s.taskStore.GetTask(ctx, att.TaskID)
	if err == nil && t != nil {
		oldStatus := t.Status
		if status == task.AttemptStatusSucceeded {
			t.Status = task.TaskStatusReview
		} else if status == task.AttemptStatusFailed || status == task.AttemptStatusCanceled {
			t.Status = task.TaskStatusBlocked
		}

		if req.BranchName != "" {
			t.BranchName = req.BranchName
		}
		if req.PRURL != "" {
			t.PRURL = req.PRURL
		}
		if req.PRNumber > 0 {
			t.PRNumber = req.PRNumber
		}
		if req.CommitSHA != "" {
			t.CommitSHA = req.CommitSHA
		}
		if req.DiffSummary != "" {
			t.DiffSummary = req.DiffSummary
		}
		if req.TestSummary != "" {
			t.TestSummary = req.TestSummary
		}

		t.UpdatedAt = now
		_ = s.taskStore.UpdateTask(ctx, t)

		// Record activity
		msg := fmt.Sprintf("Attempt %s selesai dengan status '%s' (exit code: %d, peak RSS: %d bytes).", att.ID, status, req.ExitCode, req.PeakRSSBytes)
		if t.PRURL != "" {
			msg = fmt.Sprintf("Attempt %s selesai: %s. Branch: %s, PR: %s (commit: %s).", att.ID, status, t.BranchName, t.PRURL, t.CommitSHA)
		}

		act := &task.TaskActivity{
			ID:           fmt.Sprintf("act_%d", time.Now().UnixNano()),
			TaskID:       t.ID,
			AttemptID:    &att.ID,
			ActivityType: task.ActivityTypeAttempt,
			Author:       req.RunnerID,
			OldStatus:    oldStatus,
			NewStatus:    t.Status,
			Message:      msg,
			CreatedAt:    now,
		}
		_ = s.taskStore.CreateActivity(ctx, act)
	}

	// Reset runner status to idle
	if r, err := s.store.GetRunner(ctx, req.RunnerID); err == nil && r != nil {
		r.Status = StatusIdle
		r.CurrentTaskID = ""
		r.CurrentAttemptID = ""
		_ = s.store.RegisterOrUpdateHeartbeat(ctx, r)
	}

	return att, nil
}

// ReconcileExpiredLeases sweeps expired running attempts and marks them interrupted.
func (s *RunnerService) ReconcileExpiredLeases(ctx context.Context) ([]*task.TaskAttempt, error) {
	return s.store.MarkInterruptedLeases(ctx)
}

func (s *RunnerService) ListRunners(ctx context.Context) ([]*RunnerInfo, error) {
	return s.store.ListRunners(ctx)
}

func (s *RunnerService) GetRunner(ctx context.Context, id string) (*RunnerInfo, error) {
	return s.store.GetRunner(ctx, id)
}

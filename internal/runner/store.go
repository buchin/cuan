package runner

import (
	"context"
	"errors"
	"time"

	"github.com/buchin/cuan/internal/task"
)

var (
	ErrRunnerNotFound = errors.New("Runner not found")
	ErrAlreadyBusy    = errors.New("Runner already has an active task")
)

type Store interface {
	RegisterOrUpdateHeartbeat(ctx context.Context, info *RunnerInfo) error
	GetRunner(ctx context.Context, id string) (*RunnerInfo, error)
	ListRunners(ctx context.Context) ([]*RunnerInfo, error)
	ClaimNextQueuedAttempt(ctx context.Context, runnerID string, supportedAgents []string, leaseDuration time.Duration, availableLanes ...[]string) (*task.TaskAttempt, *task.Task, error)
	RenewLease(ctx context.Context, attemptID, runnerID string, leaseDuration time.Duration) (bool, error)
	MarkInterruptedLeases(ctx context.Context) ([]*task.TaskAttempt, error)
}

// ActivityLeaseRenewer extends Store with activity-aware lease and execution
// deadline renewal, bounded by maxCeiling so hung processes still terminate.
type ActivityLeaseRenewer interface {
	RenewLeaseWithActivity(ctx context.Context, attemptID, runnerID string, leaseDuration time.Duration, activeProgress bool, deadlineExtension, maxCeiling time.Duration) (canceled bool, extendedDeadline *time.Time, err error)
}

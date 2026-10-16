package runner

import (
	"context"
	"errors"
	"time"

	"github.com/buchin/cuan/internal/task"
)

var (
	ErrRunnerNotFound = errors.New("runner tidak ditemukan")
	ErrAlreadyBusy    = errors.New("runner sudah menjalankan task aktif")
)

type Store interface {
	RegisterOrUpdateHeartbeat(ctx context.Context, info *RunnerInfo) error
	GetRunner(ctx context.Context, id string) (*RunnerInfo, error)
	ListRunners(ctx context.Context) ([]*RunnerInfo, error)
	ClaimNextQueuedAttempt(ctx context.Context, runnerID string, supportedAgents []string, leaseDuration time.Duration) (*task.TaskAttempt, *task.Task, error)
	RenewLease(ctx context.Context, attemptID, runnerID string, leaseDuration time.Duration) (bool, error)
	MarkInterruptedLeases(ctx context.Context) ([]*task.TaskAttempt, error)
}

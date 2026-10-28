package task

import (
	"context"
	"errors"
)

var (
	ErrAttemptFinal     = errors.New("Attempt has already ended")
	ErrRequestConflict  = errors.New("The task or request has changed. Reload the saved task before trying again")
	ErrTaskActive       = errors.New("Task already has an active execution")
	ErrTaskDuplicate    = errors.New("Task already exists")
	ErrTaskNotFound     = errors.New("Task not found")
	ErrAttemptNotFound  = errors.New("Attempt not found")
	ErrInvalidTaskInput = errors.New("Invalid task input")
)

type Store interface {
	// Task CRUD
	CreateTask(ctx context.Context, t *Task) error
	GetTask(ctx context.Context, id string) (*Task, error)
	ListTasks(ctx context.Context, filter TaskFilter) ([]*Task, error)
	UpdateTask(ctx context.Context, t *Task) error
	DeleteTask(ctx context.Context, id string) error

	// Attempt operations
	CreateAttempt(ctx context.Context, a *TaskAttempt) error
	GetAttempt(ctx context.Context, id string) (*TaskAttempt, error)
	GetLatestAttempt(ctx context.Context, taskID string) (*TaskAttempt, error)
	ListAttempts(ctx context.Context, taskID string) ([]*TaskAttempt, error)
	UpdateAttempt(ctx context.Context, a *TaskAttempt) error

	// Activity log & comments
	CreateActivity(ctx context.Context, act *TaskActivity) error
	ListActivities(ctx context.Context, taskID string) ([]*TaskActivity, error)

	// Log operations
	AppendLogs(ctx context.Context, attemptID string, logs []LogRecord, maxLines int) error
	GetLogs(ctx context.Context, attemptID string, afterID int64, limit int) ([]*LogRecord, error)

	// Concurrency & project status
	IsProjectActive(ctx context.Context, projectID string, excludeTaskID string) (bool, error)
	IsProjectLaneActive(ctx context.Context, projectID string, lane string, excludeTaskID string) (bool, error)
}

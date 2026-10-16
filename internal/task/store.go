package task

import (
	"context"
	"errors"
)

var (
	ErrTaskNotFound     = errors.New("task tidak ditemukan")
	ErrAttemptNotFound  = errors.New("attempt tidak ditemukan")
	ErrInvalidTaskInput = errors.New("input task tidak valid")
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
}

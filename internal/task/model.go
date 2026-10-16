package task

import "time"

// Task status constants (Pemisahan status task dari status attempt eksekusi)
const (
	TaskStatusBacklog = "backlog"
	TaskStatusDoing   = "doing"
	TaskStatusReview  = "review"
	TaskStatusDone    = "done"
	TaskStatusBlocked = "blocked"
)

// Attempt execution status constants
const (
	AttemptStatusQueued      = "queued"
	AttemptStatusRunning     = "running"
	AttemptStatusSucceeded   = "succeeded"
	AttemptStatusFailed      = "failed"
	AttemptStatusCanceled    = "canceled"
	AttemptStatusInterrupted = "interrupted"
)

// Task priority constants
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

// Activity type constants
const (
	ActivityTypeCreated      = "task_created"
	ActivityTypeStatusChange = "status_change"
	ActivityTypeAttempt      = "attempt_transition"
	ActivityTypeComment      = "comment"
	ActivityTypeUpdated      = "task_updated"
)

// Task represents a unit of work bound to a project.
type Task struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Title         string    `json:"title"`
	Instructions  string    `json:"instructions"`
	Status        string    `json:"status"`         // backlog, doing, review, done, blocked
	Priority      string    `json:"priority"`       // low, normal, high, urgent
	AssignedAgent string    `json:"assigned_agent"` // e.g. agy
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TaskAttempt represents an execution run by a runner and agent.
type TaskAttempt struct {
	ID           string     `json:"id"`
	TaskID       string     `json:"task_id"`
	RunnerID     string     `json:"runner_id"`
	AgentName    string     `json:"agent_name"`
	Status       string     `json:"status"` // queued, running, succeeded, failed, canceled, interrupted
	ExitCode     *int       `json:"exit_code,omitempty"`
	PeakRSSBytes int64      `json:"peak_rss_bytes"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// TaskActivity represents an audit history log or comment on a task.
type TaskActivity struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"task_id"`
	AttemptID    *string   `json:"attempt_id,omitempty"`
	ActivityType string    `json:"activity_type"` // task_created, status_change, attempt_transition, comment, task_updated
	Author       string    `json:"author"`        // owner email or agent identifier
	OldStatus    string    `json:"old_status,omitempty"`
	NewStatus    string    `json:"new_status,omitempty"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}

// TaskItemResponse is returned in task listings.
type TaskItemResponse struct {
	Task
	ProjectName   string       `json:"project_name,omitempty"`
	LatestAttempt *TaskAttempt `json:"latest_attempt,omitempty"`
}

// TaskDetailResponse contains full detail including history and attempts.
type TaskDetailResponse struct {
	Task
	ProjectName   string          `json:"project_name,omitempty"`
	RepoURL       string          `json:"repo_url,omitempty"`
	LatestAttempt *TaskAttempt    `json:"latest_attempt,omitempty"`
	Attempts      []*TaskAttempt  `json:"attempts"`
	Activities    []*TaskActivity `json:"activities"`
}

// Request and filter types
type CreateTaskRequest struct {
	ProjectID     string `json:"project_id"`
	Title         string `json:"title"`
	Instructions  string `json:"instructions"`
	Priority      string `json:"priority,omitempty"`
	AssignedAgent string `json:"assigned_agent,omitempty"`
}

type UpdateTaskRequest struct {
	Title         *string `json:"title,omitempty"`
	Instructions  *string `json:"instructions,omitempty"`
	Status        *string `json:"status,omitempty"`
	Priority      *string `json:"priority,omitempty"`
	AssignedAgent *string `json:"assigned_agent,omitempty"`
}

type CreateCommentRequest struct {
	Message string `json:"message"`
}

type CreateAttemptRequest struct {
	RunnerID  string `json:"runner_id,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
}

type UpdateAttemptRequest struct {
	Status       string `json:"status"` // queued, running, succeeded, failed, canceled, interrupted
	ExitCode     *int   `json:"exit_code,omitempty"`
	PeakRSSBytes *int64 `json:"peak_rss_bytes,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type TaskFilter struct {
	ProjectID string
	Status    string
	Search    string
}

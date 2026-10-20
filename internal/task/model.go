package task

import (
	"encoding/json"
	"time"
)

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
	ActivityTypeCreated         = "task_created"
	ActivityTypeStatusChange    = "status_change"
	ActivityTypeAttempt         = "attempt_transition"
	ActivityTypeComment         = "comment"
	ActivityTypeUpdated         = "task_updated"
	ActivityTypeCancelRequested = "cancel_requested"
	ActivityTypeInterrupted     = "interrupted"
	ActivityTypeRetried         = "retried"
)

// DoDItem accepts legacy string checklists as well as editable structured items.
type DoDItem struct {
	ID        string `json:"id,omitempty"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

func (d *DoDItem) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &d.Text)
	}
	type plain DoDItem
	return json.Unmarshal(data, (*plain)(d))
}

// Task represents a unit of work bound to a project.
type Task struct {
	CreationHash    string    `json:"-"`
	SourceRoomID    string    `json:"source_room_id,omitempty"`
	SourceMessageID string    `json:"source_message_id,omitempty"`
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	ProjectIDs      []string  `json:"project_ids,omitempty"`
	Title           string    `json:"title"`
	Instructions    string    `json:"instructions"`
	Summary         string    `json:"summary"`
	CompleteNote    string    `json:"complete_note"`
	DoDChecklist    []DoDItem `json:"dod_checklist"`
	Status          string    `json:"status"`         // backlog, doing, review, done, blocked
	Priority        string    `json:"priority"`       // low, normal, high, urgent
	AssignedAgent   string    `json:"assigned_agent"` // e.g. agy
	BranchName      string    `json:"branch_name,omitempty"`
	PRURL           string    `json:"pr_url,omitempty"`
	PRNumber        int       `json:"pr_number,omitempty"`
	CommitSHA       string    `json:"commit_sha,omitempty"`
	DiffSummary     string    `json:"diff_summary,omitempty"`
	TestSummary     string    `json:"test_summary,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TaskAttempt represents an execution run by a runner and agent.
type TaskAttempt struct {
	ID              string     `json:"id"`
	RequestHash     string     `json:"-"`
	TaskID          string     `json:"task_id"`
	RunnerID        string     `json:"runner_id"`
	AgentName       string     `json:"agent_name"`
	Status          string     `json:"status"` // queued, running, succeeded, failed, canceled, interrupted
	ExitCode        *int       `json:"exit_code,omitempty"`
	PeakRSSBytes    int64      `json:"peak_rss_bytes"`
	LeaseExpiresAt  *time.Time `json:"lease_expires_at,omitempty"`
	CancelRequested bool       `json:"cancel_requested"`
	CanceledAt      *time.Time `json:"canceled_at,omitempty"`
	InterruptedAt   *time.Time `json:"interrupted_at,omitempty"`
	AttemptNumber   int        `json:"attempt_number"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
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

// LogRecord represents a line of output from agent execution.
type LogRecord struct {
	ID        int64     `json:"id"`
	AttemptID string    `json:"attempt_id"`
	Stream    string    `json:"stream"` // stdout, stderr, system
	Line      string    `json:"line"`
	LoggedAt  time.Time `json:"logged_at"`
}

// TaskItemResponse is returned in task listings.
type TaskItemResponse struct {
	Task
	ProjectName   string       `json:"project_name,omitempty"`
	ProjectNames  []string     `json:"project_names,omitempty"`
	LatestAttempt *TaskAttempt `json:"latest_attempt,omitempty"`
}

// TaskDetailResponse contains full detail including history and attempts.
type TaskDetailResponse struct {
	Launch *LaunchInfo `json:"launch,omitempty"`
	Task
	ProjectName   string          `json:"project_name,omitempty"`
	ProjectNames  []string        `json:"project_names,omitempty"`
	RepoURL       string          `json:"repo_url,omitempty"`
	LatestAttempt *TaskAttempt    `json:"latest_attempt,omitempty"`
	Attempts      []*TaskAttempt  `json:"attempts"`
	Activities    []*TaskActivity `json:"activities"`
}

// Request and filter types
type CreateTaskRequest struct {
	RequestID       string    `json:"request_id,omitempty"`
	Status          string    `json:"status,omitempty"`
	SourceRoomID    string    `json:"-"`
	SourceMessageID string    `json:"-"`
	ProjectID       string    `json:"project_id"`
	ProjectIDs      []string  `json:"project_ids,omitempty"`
	Title           string    `json:"title"`
	Instructions    string    `json:"instructions"`
	Priority        string    `json:"priority,omitempty"`
	AssignedAgent   string    `json:"assigned_agent,omitempty"`
	DoDChecklist    []DoDItem `json:"dod_checklist,omitempty"`
}

type UpdateTaskRequest struct {
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at,omitempty"`
	ProjectIDs        *[]string  `json:"project_ids,omitempty"`
	Title             *string    `json:"title,omitempty"`
	Summary           *string    `json:"summary,omitempty"`
	CompleteNote      *string    `json:"complete_note,omitempty"`
	DoDChecklist      *[]DoDItem `json:"dod_checklist,omitempty"`
	Instructions      *string    `json:"instructions,omitempty"`
	Status            *string    `json:"status,omitempty"`
	Priority          *string    `json:"priority,omitempty"`
	AssignedAgent     *string    `json:"assigned_agent,omitempty"`
	BranchName        *string    `json:"branch_name,omitempty"`
	PRURL             *string    `json:"pr_url,omitempty"`
	PRNumber          *int       `json:"pr_number,omitempty"`
	CommitSHA         *string    `json:"commit_sha,omitempty"`
	DiffSummary       *string    `json:"diff_summary,omitempty"`
	TestSummary       *string    `json:"test_summary,omitempty"`
}

type CreateCommentRequest struct {
	Message string `json:"message"`
}

type CreateAttemptRequest struct {
	RequestID   string `json:"request_id,omitempty"`
	RequestHash string `json:"-"`
	RunnerID    string `json:"runner_id,omitempty"`
	AgentName   string `json:"agent_name,omitempty"`
}

type UpdateAttemptRequest struct {
	Status       string `json:"status"` // queued, running, succeeded, failed, canceled, interrupted
	ExitCode     *int   `json:"exit_code,omitempty"`
	PeakRSSBytes *int64 `json:"peak_rss_bytes,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type TaskFilter struct {
	SourceRoomID string
	ProjectID    string
	ProjectIDs   []string
	Status       string
	Search       string
}

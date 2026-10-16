package runner

import (
	"time"

	"github.com/buchin/cuan/internal/task"
)

// Runner status constants
const (
	StatusIdle    = "idle"
	StatusBusy    = "busy"
	StatusOffline = "offline"
)

// RunnerInfo contains metadata, status, and health metrics of a runner daemon.
type RunnerInfo struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Hostname         string    `json:"hostname"`
	SupportedAgents  []string  `json:"supported_agents"`
	Status           string    `json:"status"` // idle, busy, offline
	LastHeartbeatAt  time.Time `json:"last_heartbeat_at"`
	CurrentTaskID    string    `json:"current_task_id,omitempty"`
	CurrentAttemptID string    `json:"current_attempt_id,omitempty"`
	CPUPercent       float64   `json:"cpu_percent"`
	MemoryUsedBytes  int64     `json:"memory_used_bytes"`
	MemoryTotalBytes int64     `json:"memory_total_bytes"`
	DiskUsedBytes    int64     `json:"disk_used_bytes"`
	DiskTotalBytes   int64     `json:"disk_total_bytes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// HeartbeatRequest is sent periodically by runner daemon to keep its lease alive and report metrics.
type HeartbeatRequest struct {
	RunnerID         string   `json:"runner_id"`
	Hostname         string   `json:"hostname,omitempty"`
	SupportedAgents  []string `json:"supported_agents,omitempty"`
	CurrentAttemptID string   `json:"current_attempt_id,omitempty"`
	CPUPercent       float64  `json:"cpu_percent"`
	MemoryUsedBytes  int64    `json:"memory_used_bytes"`
	MemoryTotalBytes int64    `json:"memory_total_bytes"`
	DiskUsedBytes    int64    `json:"disk_used_bytes"`
	DiskTotalBytes   int64    `json:"disk_total_bytes"`
}

// HeartbeatResponse instructs the runner on lease renewal and whether cancellation is requested.
type HeartbeatResponse struct {
	Status           string `json:"status"`
	LeaseDurationSec int    `json:"lease_duration_sec"`
	ShouldCancel     bool   `json:"should_cancel"`
}

// ClaimTaskRequest is sent by an idle runner to acquire the next queued task attempt.
type ClaimTaskRequest struct {
	RunnerID        string   `json:"runner_id"`
	SupportedAgents []string `json:"supported_agents"`
}

// ClaimTaskResponse contains the claimed attempt and task payload if a task was assigned.
type ClaimTaskResponse struct {
	Claimed      bool              `json:"claimed"`
	Attempt      *task.TaskAttempt `json:"attempt,omitempty"`
	Task         *task.Task        `json:"task,omitempty"`
	ProjectRepo  string            `json:"project_repo,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	LeaseExpires time.Time         `json:"lease_expires"`
}

// StreamLogsRequest is used by runner to stream batches of output lines to the server.
type StreamLogsRequest struct {
	RunnerID  string           `json:"runner_id"`
	AttemptID string           `json:"attempt_id"`
	Logs      []task.LogRecord `json:"logs"`
}

// CompleteAttemptRequest reports the final outcome of an attempt.
type CompleteAttemptRequest struct {
	RunnerID     string `json:"runner_id"`
	Status       string `json:"status"` // succeeded, failed, canceled
	ExitCode     int    `json:"exit_code"`
	PeakRSSBytes int64  `json:"peak_rss_bytes"`
	ErrorMessage string `json:"error_message,omitempty"`
}

package runner

import (
	"github.com/buchin/cuan/internal/executionpolicy"
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
	ActiveLanes      []string  `json:"active_lanes,omitempty"`
	ActiveAttemptIDs []string  `json:"active_attempt_ids,omitempty"`
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
	ActiveAttemptIDs []string `json:"active_attempt_ids,omitempty"`
	CPUPercent       float64  `json:"cpu_percent"`
	MemoryUsedBytes  int64    `json:"memory_used_bytes"`
	MemoryTotalBytes int64    `json:"memory_total_bytes"`
	DiskUsedBytes    int64    `json:"disk_used_bytes"`
	DiskTotalBytes   int64    `json:"disk_total_bytes"`
}

// HeartbeatResponse instructs the runner on lease renewal and whether cancellation is requested.
type HeartbeatResponse struct {
	Status             string              `json:"status"`
	LeaseDurationSec   int                 `json:"lease_duration_sec"`
	ShouldCancel       bool                `json:"should_cancel"`
	CanceledAttemptIDs []string            `json:"canceled_attempt_ids,omitempty"`
	SteeringDirectives []SteeringDirective `json:"steering_directives,omitempty"`
}

// SteeringDirective is sent from server to runner to steer an in-flight attempt.
type SteeringDirective struct {
	AttemptID string `json:"attempt_id"`
	Prompt    string `json:"prompt"`
	Author    string `json:"author,omitempty"`
}

// SteerAttemptRequest is sent by client or API to steer an active attempt.
type SteerAttemptRequest struct {
	Prompt string `json:"prompt"`
	Author string `json:"author,omitempty"`
}

// SteerAttemptResponse reports the outcome of a steering instruction.
type SteerAttemptResponse struct {
	Status   string `json:"status"` // "delivered", "queued", "fallback"
	Message  string `json:"message"`
	Fallback bool   `json:"fallback"`
}

// ClaimTaskRequest is sent by a runner to acquire the next queued task attempt for available lanes.
type ClaimTaskRequest struct {
	RunnerID        string   `json:"runner_id"`
	SupportedAgents []string `json:"supported_agents"`
	AvailableLanes  []string `json:"available_lanes,omitempty"` // code, ops, readonly
}

// ClaimTaskResponse contains the claimed attempt and task payload if a task was assigned.
type ClaimTaskResponse struct {
	PublicationLimits executionpolicy.PublicationPolicy `json:"publication_limits"`
	OwnerID           string                            `json:"owner_id,omitempty"`
	RequiresVision    bool                              `json:"requires_vision,omitempty"`
	Claimed           bool                              `json:"claimed"`
	Attempt           *task.TaskAttempt                 `json:"attempt,omitempty"`
	Task              *task.Task                        `json:"task,omitempty"`
	ProjectRepo       string                            `json:"project_repo,omitempty"`
	RepoOwner         string                            `json:"repo_owner,omitempty"`
	RepoName          string                            `json:"repo_name,omitempty"`
	RepoURL           string                            `json:"repo_url,omitempty"`
	DefaultBranch     string                            `json:"default_branch,omitempty"`
	DeployBranch      string                            `json:"deploy_branch,omitempty"`
	WorkingDir        string                            `json:"working_dir,omitempty"`
	AutoDeploy        bool                              `json:"auto_deploy,omitempty"`
	Environment       string                            `json:"environment,omitempty"`
	LeaseExpires      time.Time                         `json:"lease_expires"`
	TaskAgent         *TaskAgentCapability              `json:"task_agent,omitempty"`
	AgentPrompt       string                            `json:"agent_prompt,omitempty"`
}

// TaskAgentCapability is scoped to one task attempt and is used by the
// background runner to access the same reporting contract as interactive sessions.
type TaskAgentCapability struct {
	API       string `json:"api"`
	Token     string `json:"token"`
	TaskID    string `json:"task_id"`
	AttemptID string `json:"attempt_id"`
}

// StreamLogsRequest is used by runner to stream batches of output lines to the server.
type StreamLogsRequest struct {
	RunnerID  string           `json:"runner_id"`
	AttemptID string           `json:"attempt_id"`
	Logs      []task.LogRecord `json:"logs"`
}

// CompleteAttemptRequest reports the final outcome of an attempt.
type CompleteAttemptRequest struct {
	RepositoryVerification  *RepositoryVerification `json:"repository_verification,omitempty"`
	RunnerID                string                  `json:"runner_id"`
	RoomID                  string                  `json:"room_id,omitempty"`
	Status                  string                  `json:"status"` // succeeded, failed, canceled
	ExitCode                int                     `json:"exit_code"`
	PeakRSSBytes            int64                   `json:"peak_rss_bytes"`
	ErrorMessage            string                  `json:"error_message,omitempty"`
	BranchName              string                  `json:"branch_name,omitempty"`
	PRURL                   string                  `json:"pr_url,omitempty"`
	PRNumber                int                     `json:"pr_number,omitempty"`
	CommitSHA               string                  `json:"commit_sha,omitempty"`
	DiffSummary             string                  `json:"diff_summary,omitempty"`
	TestSummary             string                  `json:"test_summary,omitempty"`
	AgentReportConfirmed    bool                    `json:"agent_report_confirmed"`
	AgentReportRequired     bool                    `json:"agent_report_required"`
	LiveDeploymentConfirmed bool                    `json:"live_deployment_confirmed,omitempty"`
	DoDValidation           *DoDValidationResult    `json:"dod_validation,omitempty"`
	RequeueFeedback         string                  `json:"requeue_feedback,omitempty"`
	RequiresDeploy          *bool                   `json:"requires_deploy,omitempty"`
	HasCodeChanges          *bool                   `json:"has_code_changes,omitempty"`
}

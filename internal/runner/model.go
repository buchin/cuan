package runner

import (
	"github.com/buchin/cuan/internal/executionpolicy"
	"time"

	"github.com/buchin/cuan/internal/release"
	"github.com/buchin/cuan/internal/task"
)

// Runner status constants
const (
	StatusIdle    = "idle"
	StatusBusy    = "busy"
	StatusOffline = "offline"
)

// RunnerInfo contains metadata, status, and health metrics of a runner daemon.
type RunnerRuntimeEvidence struct {
	BrowserRuntime        *task.VerificationRuntime          `json:"browser_runtime,omitempty"`
	DependencyPreparation *task.DependencyPreparationRuntime `json:"dependency_preparation,omitempty"`
}

type RunnerInfo struct {
	RuntimeEvidence  RunnerRuntimeEvidence `json:"runtime_evidence,omitempty"`
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Hostname         string                `json:"hostname"`
	SupportedAgents  []string              `json:"supported_agents"`
	Status           string                `json:"status"` // idle, busy, offline
	ActiveLanes      []string              `json:"active_lanes,omitempty"`
	ActiveAttemptIDs []string              `json:"active_attempt_ids,omitempty"`
	LastHeartbeatAt  time.Time             `json:"last_heartbeat_at"`
	CurrentTaskID    string                `json:"current_task_id,omitempty"`
	CurrentAttemptID string                `json:"current_attempt_id,omitempty"`
	CPUPercent       float64               `json:"cpu_percent"`
	MemoryUsedBytes  int64                 `json:"memory_used_bytes"`
	MemoryTotalBytes int64                 `json:"memory_total_bytes"`
	DiskUsedBytes    int64                 `json:"disk_used_bytes"`
	DiskTotalBytes   int64                 `json:"disk_total_bytes"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// HeartbeatRequest is sent periodically by runner daemon to keep its lease alive and report metrics.
type HeartbeatRequest struct {
	DependencyPreparation *task.DependencyPreparationRuntime `json:"dependency_preparation,omitempty"`
	BrowserRuntime        *task.VerificationRuntime          `json:"browser_runtime,omitempty"`
	RunnerID              string                             `json:"runner_id"`
	Hostname              string                             `json:"hostname,omitempty"`
	SupportedAgents       []string                           `json:"supported_agents,omitempty"`
	CurrentAttemptID         string                             `json:"current_attempt_id,omitempty"`
	ActiveAttemptIDs         []string                           `json:"active_attempt_ids,omitempty"`
	ActiveProgressAttemptIDs []string                           `json:"active_progress_attempt_ids,omitempty"`
	CPUPercent               float64                            `json:"cpu_percent"`
	MemoryUsedBytes       int64                              `json:"memory_used_bytes"`
	MemoryTotalBytes      int64                              `json:"memory_total_bytes"`
	DiskUsedBytes         int64                              `json:"disk_used_bytes"`
	DiskTotalBytes        int64                              `json:"disk_total_bytes"`
}

// HeartbeatResponse instructs the runner on lease renewal and whether cancellation is requested.
type HeartbeatResponse struct {
	Status             string               `json:"status"`
	LeaseDurationSec   int                  `json:"lease_duration_sec"`
	ShouldCancel       bool                 `json:"should_cancel"`
	CanceledAttemptIDs []string             `json:"canceled_attempt_ids,omitempty"`
	SteeringDirectives []SteeringDirective  `json:"steering_directives,omitempty"`
	ExtendedDeadlines  map[string]time.Time `json:"extended_deadlines,omitempty"`
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
	SupportsNativeRelease            bool     `json:"supports_native_release,omitempty"`
	SupportsEvidenceRevalidation     bool     `json:"supports_evidence_revalidation,omitempty"`
	SupportsNativeVisualVerification bool     `json:"supports_native_visual_verification,omitempty"`
	SupportsCurrentOutcomeInspection bool     `json:"supports_current_outcome_inspection,omitempty"`
	SupportsLiveInspectionRecovery   bool     `json:"supports_live_inspection_recovery,omitempty"`
	SupportsLiveInspection           bool     `json:"supports_live_inspection,omitempty"`
	SupportsVerificationFinalization bool     `json:"supports_verification_finalization,omitempty"`
	RunnerID                         string   `json:"runner_id"`
	SupportedAgents                  []string `json:"supported_agents"`
	AvailableLanes                   []string `json:"available_lanes,omitempty"` // code, ops, readonly
	PreferredTaskID                  string   `json:"preferred_task_id,omitempty"` // Task Affinity: run-to-completion active task bundle
}

// ClaimTaskResponse contains the claimed attempt and task payload if a task was assigned.
type ClaimTaskResponse struct {
	NativeReleaseExecution *NativeReleaseExecution           `json:"native_release_execution,omitempty"`
	ManagedRelease         *ManagedReleaseClaim              `json:"managed_release,omitempty"`
	NativeRelease          *task.NativeReleaseIntent         `json:"native_release,omitempty"`
	PHPUnitTargets         string                            `json:"phpunit_targets,omitempty"`
	VisualRecipe           *task.VisualRecipe                `json:"visual_recipe,omitempty"`
	LiveInspection         *task.LiveInspectionIntent        `json:"live_inspection,omitempty"`
	Verification           *task.VerificationIntent          `json:"verification,omitempty"`
	PublicationLimits      executionpolicy.PublicationPolicy `json:"publication_limits"`
	OwnerID                string                            `json:"owner_id,omitempty"`
	RequiresVision         bool                              `json:"requires_vision,omitempty"`
	Claimed                bool                              `json:"claimed"`
	Attempt                *task.TaskAttempt                 `json:"attempt,omitempty"`
	Task                   *task.Task                        `json:"task,omitempty"`
	ProjectRepo            string                            `json:"project_repo,omitempty"`
	RepoOwner              string                            `json:"repo_owner,omitempty"`
	RepoName               string                            `json:"repo_name,omitempty"`
	RepoURL                string                            `json:"repo_url,omitempty"`
	DefaultBranch          string                            `json:"default_branch,omitempty"`
	DeployBranch           string                            `json:"deploy_branch,omitempty"`
	DeployScript           string                            `json:"deploy_script,omitempty"`
	WorkingDir             string                            `json:"working_dir,omitempty"`
	AutoDeploy             bool                              `json:"auto_deploy,omitempty"`
	Environment            string                            `json:"environment,omitempty"`
	LeaseExpires           time.Time                         `json:"lease_expires"`
	TaskAgent              *TaskAgentCapability              `json:"task_agent,omitempty"`
	AgentPrompt            string                            `json:"agent_prompt,omitempty"`
}

// TaskAgentCapability is scoped to one task attempt and is used by the
// background runner to access the same reporting contract as interactive sessions.
type TaskAgentCapability struct {
	API          string `json:"api"`
	Token        string `json:"token"`
	TaskID       string `json:"task_id"`
	AttemptID    string `json:"attempt_id"`
	ArtifactsDir string `json:"artifacts_dir,omitempty"`
	ReportFile   string `json:"report_file,omitempty"`
}

// StreamLogsRequest is used by runner to stream batches of output lines to the server.
type StreamLogsRequest struct {
	RunnerID  string           `json:"runner_id"`
	AttemptID string           `json:"attempt_id"`
	Logs      []task.LogRecord `json:"logs"`
}

// CompleteAttemptRequest reports the final outcome of an attempt.
type CompleteAttemptRequest struct {
	ExecutionTiming                *task.ExecutionTiming    `json:"execution_timing,omitempty"`
	NativeReleaseProcess           *release.Result          `json:"native_release_process,omitempty"`
	ManagedReleaseSource           *ManagedReleaseSource    `json:"managed_release_source,omitempty"`
	VisualVerification             *task.VisualVerification `json:"visual_verification,omitempty"`
	LiveInspectionCommit           string                   `json:"live_inspection_commit,omitempty"`
	SourceAncestryVerified         bool                     `json:"source_ancestry_verified,omitempty"`
	CurrentOutcomeVerified         bool                     `json:"current_outcome_verified,omitempty"`
	FinalizationManifestHash       string                   `json:"finalization_manifest_hash,omitempty"`
	VerificationRuntimeFingerprint string                   `json:"verification_runtime_fingerprint,omitempty"`
	VerifiedSourceCommit           string                   `json:"verified_source_commit,omitempty"`
	RepositoryVerification         *RepositoryVerification  `json:"repository_verification,omitempty"`
	RunnerID                       string                   `json:"runner_id"`
	RoomID                         string                   `json:"room_id,omitempty"`
	Status                         string                   `json:"status"` // succeeded, failed, canceled
	ExitCode                       int                      `json:"exit_code"`
	PeakRSSBytes                   int64                    `json:"peak_rss_bytes"`
	ErrorMessage                   string                   `json:"error_message,omitempty"`
	BranchName                     string                   `json:"branch_name,omitempty"`
	PRURL                          string                   `json:"pr_url,omitempty"`
	PRNumber                       int                      `json:"pr_number,omitempty"`
	CommitSHA                      string                   `json:"commit_sha,omitempty"`
	DiffSummary                    string                   `json:"diff_summary,omitempty"`
	TestSummary                    string                   `json:"test_summary,omitempty"`
	AgentReportConfirmed           bool                     `json:"agent_report_confirmed"`
	AgentReportRequired            bool                     `json:"agent_report_required"`
	LiveDeploymentConfirmed        bool                     `json:"live_deployment_confirmed,omitempty"`
	DoDValidation                  *DoDValidationResult     `json:"dod_validation,omitempty"`
	RequeueFeedback                string                   `json:"requeue_feedback,omitempty"`
	RequiresDeploy                 *bool                    `json:"requires_deploy,omitempty"`
	HasCodeChanges                 *bool                    `json:"has_code_changes,omitempty"`
}

// VisualVerificationRequest is accepted only through authenticated runner transport.
type VisualVerificationRequest struct {
	RunnerID     string                   `json:"runner_id"`
	Verification *task.VisualVerification `json:"verification"`
	Files        [][]byte                 `json:"files"`
}

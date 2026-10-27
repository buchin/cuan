package deployer

import (
	"encoding/json"
	"io"
	"time"
)

// Runtime constants
const (
	RuntimePHP    = "php"
	RuntimeNodeJS = "nodejs"
	RuntimeStatic = "static"
	RuntimeCustom = "custom"
)

// Source type constants
const (
	SourceTypeZip = "zip"
	SourceTypeGit = "git"
)

// Status constants
const (
	StatusPending    = "pending"
	StatusExtracting = "extracting"
	StatusBuilding   = "building"
	StatusRunning    = "running"
	StatusStopped    = "stopped"
	StatusFailed     = "failed"
	StatusCrashed    = "crashed"
)

// Stream constants
const (
	StreamStdout = "stdout"
	StreamStderr = "stderr"
	StreamSystem = "system"
)

// Deployment represents an application deployment record.
type Deployment struct {
	ID           string                 `json:"id"`
	ServerID     string                 `json:"server_id,omitempty"` // Foreign key to servers.id for BYOS deployments (empty for local)
	ProjectName  string                 `json:"project_name"`
	Slug         string                 `json:"slug"`
	SourceType   string                 `json:"source_type"` // 'zip', 'git'
	SourceRef    string                 `json:"source_ref"`
	Runtime      string                 `json:"runtime"` // 'php', 'nodejs', 'static', 'custom'
	Status       string                 `json:"status"`  // 'pending', 'extracting', 'building', 'running', 'stopped', 'failed'
	InternalPort int                    `json:"internal_port"`
	Domain       string                 `json:"domain"`
	EnvVars      map[string]interface{} `json:"env_vars"`
	ProcessPID   int                    `json:"process_pid"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// LogEntry represents a log message produced by a deployment process.
type LogEntry struct {
	ID           int64     `json:"id"`
	DeploymentID string    `json:"deployment_id"`
	Stream       string    `json:"stream"` // 'stdout', 'stderr', 'system'
	Message      string    `json:"message"`
	Timestamp    time.Time `json:"timestamp"`
}

// DeploymentRequest encapsulates the parameters needed to initiate or update a deployment.
type DeploymentRequest struct {
	ID           string                 `json:"id"`
	ServerID     string                 `json:"server_id,omitempty"` // Target server id for BYOS or empty for local
	ProjectName  string                 `json:"project_name"`
	Slug         string                 `json:"slug"`
	SourceType   string                 `json:"source_type"` // 'zip', 'git'
	SourceRef    string                 `json:"source_ref"`
	Branch       string                 `json:"branch,omitempty"`
	Runtime      string                 `json:"runtime"` // optional; auto-detected if empty or 'custom'
	InternalPort int                    `json:"internal_port"`
	Domain       string                 `json:"domain"`
	EnvVars      map[string]interface{} `json:"env_vars"`
	ZipReader    io.Reader              `json:"-"`
	ZipSize      int64                  `json:"-"`
}

// DeploymentResult captures the output of a Deploy execution.
type DeploymentResult struct {
	Deployment      *Deployment `json:"deployment"`
	DetectedRuntime string      `json:"detected_runtime"`
	ExtractedPath   string      `json:"extracted_path"`
	Status          string      `json:"status"`
}

// Helper to serialize EnvVars map to JSON
func (d *Deployment) EnvVarsJSON() ([]byte, error) {
	if d.EnvVars == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(d.EnvVars)
}

package deployer

import (
	"context"
	"errors"
)

var (
	// ErrDeploymentNotFound is returned when a deployment does not exist.
	ErrDeploymentNotFound = errors.New("deployment not found")

	// ErrSlugConflict is returned when a slug is already taken.
	ErrSlugConflict = errors.New("deployment slug already exists")
)

// Store defines persistence methods for deployments and deployment logs.
type Store interface {
	// CreateDeployment records a new deployment.
	CreateDeployment(ctx context.Context, dep *Deployment) error

	// GetDeployment retrieves a deployment by its unique ID.
	GetDeployment(ctx context.Context, id string) (*Deployment, error)

	// GetDeploymentBySlug retrieves a deployment by its URL slug.
	GetDeploymentBySlug(ctx context.Context, slug string) (*Deployment, error)

	// UpdateDeployment updates mutable fields of an existing deployment.
	UpdateDeployment(ctx context.Context, dep *Deployment) error

	// UpdateDeploymentStatus updates only the status and updated_at timestamp.
	UpdateDeploymentStatus(ctx context.Context, id, status string) error

	// ListDeployments returns all deployments ordered by creation timestamp descending.
	ListDeployments(ctx context.Context) ([]Deployment, error)

	// DeleteDeployment removes a deployment record and cascaded logs.
	DeleteDeployment(ctx context.Context, id string) error

	// AppendLog appends a new stdout/stderr/system log message.
	AppendLog(ctx context.Context, log *LogEntry) error

	// GetLogs fetches the latest log entries for a deployment up to limit.
	GetLogs(ctx context.Context, deploymentID string, limit int) ([]LogEntry, error)
}

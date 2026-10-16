package deployer

import (
	"context"
)

// DeployDriver defines the modular abstraction for deploying, managing,
// and inspecting application runtimes.
//
// Implementations:
//   - LocalDriver: Co-located execution on local host/sandboxes (Sprint 3).
//   - SSHDriver: Remote execution on user-provided servers via SSH / BYOS (Sprint 4).
type DeployDriver interface {
	// Deploy extracts/clones project files, prepares environment, and initializes deployment state.
	Deploy(ctx context.Context, req DeploymentRequest) (*DeploymentResult, error)

	// Start transitions the deployment process to running status.
	Start(ctx context.Context, id string) error

	// Stop cleanly terminates the active deployment process.
	Stop(ctx context.Context, id string) error

	// Restart stops and restarts the deployment process.
	Restart(ctx context.Context, id string) error

	// GetLogs retrieves recent log entries for the specified deployment.
	GetLogs(ctx context.Context, id string, limit int) ([]LogEntry, error)
}

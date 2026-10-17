package deployer

import (
	"context"
	"fmt"
)

// DriverFactory routes deployment lifecycle operations to LocalDriver or RemoteSSHDriver
// based on whether server_id is configured.
type DriverFactory struct {
	localDriver  DeployDriver
	remoteDriver DeployDriver
	store        Store
}

// NewDriverFactory creates a new DriverFactory routing between local and remote drivers.
func NewDriverFactory(local DeployDriver, remote DeployDriver, store Store) *DriverFactory {
	return &DriverFactory{
		localDriver:  local,
		remoteDriver: remote,
		store:        store,
	}
}

// LocalDriver returns the underlying local execution driver.
func (f *DriverFactory) LocalDriver() DeployDriver {
	return f.localDriver
}

// RemoteDriver returns the underlying remote SSH driver.
func (f *DriverFactory) RemoteDriver() DeployDriver {
	return f.remoteDriver
}

// GetDriver selects the driver based on serverID: returns RemoteSSHDriver if serverID is non-empty,
// or LocalDriver if serverID is empty.
func (f *DriverFactory) GetDriver(serverID string) DeployDriver {
	if serverID != "" && f.remoteDriver != nil {
		return f.remoteDriver
	}
	return f.localDriver
}

// DriverFor returns the appropriate driver for an existing deployment by looking up its record.
func (f *DriverFactory) DriverFor(ctx context.Context, deploymentID string) (DeployDriver, error) {
	dep, err := f.store.GetDeployment(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	return f.GetDriver(dep.ServerID), nil
}

// Deploy dispatches deployment preparation and file transfer to the matching driver.
func (f *DriverFactory) Deploy(ctx context.Context, req DeploymentRequest) (*DeploymentResult, error) {
	driver := f.GetDriver(req.ServerID)
	if driver == nil {
		return nil, fmt.Errorf("no deploy driver configured for server %q", req.ServerID)
	}
	return driver.Deploy(ctx, req)
}

// Start launches the deployment process on the targeted runtime environment.
func (f *DriverFactory) Start(ctx context.Context, id string) error {
	driver, err := f.DriverFor(ctx, id)
	if err != nil {
		return err
	}
	return driver.Start(ctx, id)
}

// Stop cleanly terminates the deployment process.
func (f *DriverFactory) Stop(ctx context.Context, id string) error {
	driver, err := f.DriverFor(ctx, id)
	if err != nil {
		return err
	}
	return driver.Stop(ctx, id)
}

// Restart restarts the deployment process.
func (f *DriverFactory) Restart(ctx context.Context, id string) error {
	driver, err := f.DriverFor(ctx, id)
	if err != nil {
		return err
	}
	return driver.Restart(ctx, id)
}

// Health inspects liveness and port responsiveness for the deployment.
func (f *DriverFactory) Health(ctx context.Context, id string) (*HealthStatus, error) {
	driver, err := f.DriverFor(ctx, id)
	if err != nil {
		return nil, err
	}
	return driver.Health(ctx, id)
}

// GetLogs fetches log entries for the deployment.
func (f *DriverFactory) GetLogs(ctx context.Context, id string, limit int) ([]LogEntry, error) {
	driver, err := f.DriverFor(ctx, id)
	if err != nil {
		return f.store.GetLogs(ctx, id, limit)
	}
	return driver.GetLogs(ctx, id, limit)
}

// List retrieves all deployments from the persistent store.
func (f *DriverFactory) List(ctx context.Context) ([]Deployment, error) {
	return f.store.ListDeployments(ctx)
}

// Get retrieves an individual deployment record by unique ID.
func (f *DriverFactory) Get(ctx context.Context, id string) (*Deployment, error) {
	return f.store.GetDeployment(ctx, id)
}

// Delete cleans up process, files, routes, and database records for the deployment.
func (f *DriverFactory) Delete(ctx context.Context, id string) error {
	driver, err := f.DriverFor(ctx, id)
	if err != nil {
		return err
	}
	return driver.Delete(ctx, id)
}

// Compile-time check for DeployDriver implementation
var _ DeployDriver = (*DriverFactory)(nil)

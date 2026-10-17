package deployer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
)

// HealthCheckResult represents the outcome of probing a single remote deployment.
type HealthCheckResult struct {
	DeploymentID   string `json:"deployment_id"`
	ServerID       string `json:"server_id"`
	PID            int    `json:"pid"`
	Status         string `json:"status"`          // StatusRunning, StatusStopped, StatusCrashed
	PreviousStatus string `json:"previous_status"` // previous status
	Alive          bool   `json:"alive"`
	Port           int    `json:"port"`
	PortOpen       bool   `json:"port_open"`
	Error          string `json:"error,omitempty"`
	ActionTaken    string `json:"action_taken,omitempty"` // "none", "marked_crashed", "marked_stopped", "reconnect_scheduled"
}

// StateChangeFunc is a callback invoked when a deployment transitions status during polling.
type StateChangeFunc func(deploymentID string, oldStatus, newStatus string)

// ProcessHealthPoller periodically inspects running remote deployments over SSH,
// detects process crashes or unexpected terminations, releases ports, updates status to crashed/stopped,
// and gracefully handles temporary network disconnects with auto-reconnect logic.
type ProcessHealthPoller struct {
	store         Store
	serverRepo    db.ServerRepository
	keyVault      *crypto.KeyVault
	sshManager    SSHManager
	portPool      *PortPool
	interval      time.Duration
	maxRetries    int
	consecErrors  map[string]int // deploymentID -> consecutive error count
	onStateChange StateChangeFunc

	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.RWMutex
	running bool
}

// PollerOption configures ProcessHealthPoller.
type PollerOption func(*ProcessHealthPoller)

// WithPollerInterval sets the polling frequency.
func WithPollerInterval(d time.Duration) PollerOption {
	return func(p *ProcessHealthPoller) {
		if d > 0 {
			p.interval = d
		}
	}
}

// WithPollerMaxRetries sets the number of consecutive retries for network disconnects.
func WithPollerMaxRetries(n int) PollerOption {
	return func(p *ProcessHealthPoller) {
		if n > 0 {
			p.maxRetries = n
		}
	}
}

// WithPollerStateChangeCallback sets the callback for status changes.
func WithPollerStateChangeCallback(fn StateChangeFunc) PollerOption {
	return func(p *ProcessHealthPoller) {
		p.onStateChange = fn
	}
}

// NewProcessHealthPoller instantiates a new background health poller.
func NewProcessHealthPoller(
	store Store,
	serverRepo db.ServerRepository,
	keyVault *crypto.KeyVault,
	sshManager SSHManager,
	portPool *PortPool,
	opts ...PollerOption,
) *ProcessHealthPoller {
	p := &ProcessHealthPoller{
		store:        store,
		serverRepo:   serverRepo,
		keyVault:     keyVault,
		sshManager:   sshManager,
		portPool:     portPool,
		interval:     5 * time.Second,
		maxRetries:   3,
		consecErrors: make(map[string]int),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// IsRunning reports whether the background poller loop is active.
func (p *ProcessHealthPoller) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// ConsecutiveErrors returns the count of consecutive errors for the deployment.
func (p *ProcessHealthPoller) ConsecutiveErrors(deploymentID string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.consecErrors[deploymentID]
}

// Start launches the background health polling goroutine.
func (p *ProcessHealthPoller) Start(parentCtx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}

	if parentCtx == nil {
		parentCtx = context.Background()
	}

	p.ctx, p.cancel = context.WithCancel(parentCtx)
	p.done = make(chan struct{})
	p.running = true
	p.mu.Unlock()

	go p.pollLoop()
	return nil
}

// Stop cleanly terminates the background health polling loop.
func (p *ProcessHealthPoller) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	if p.cancel != nil {
		p.cancel()
	}
	doneCh := p.done
	p.mu.Unlock()

	if doneCh != nil {
		select {
		case <-doneCh:
		case <-time.After(2 * time.Second):
		}
	}
}

// pollLoop executes periodic health probes.
func (p *ProcessHealthPoller) pollLoop() {
	defer close(p.done)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			_, _ = p.PollOnce(p.ctx)
		}
	}
}

// PollOnce inspects all running remote deployments synchronously, returning their status results.
func (p *ProcessHealthPoller) PollOnce(ctx context.Context) ([]HealthCheckResult, error) {
	deployments, err := p.store.ListDeployments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list deployments for health polling: %w", err)
	}

	var results []HealthCheckResult
	for _, dep := range deployments {
		// Only poll remote deployments that are currently recorded as running
		if dep.ServerID == "" || dep.Status != StatusRunning {
			continue
		}

		res, err := p.checkDeployment(ctx, &dep)
		if err != nil {
			results = append(results, HealthCheckResult{
				DeploymentID:   dep.ID,
				ServerID:       dep.ServerID,
				PID:            dep.ProcessPID,
				Status:         dep.Status,
				PreviousStatus: dep.Status,
				Alive:          false,
				Error:          err.Error(),
				ActionTaken:    "none",
			})
			continue
		}

		if res != nil {
			results = append(results, *res)
		}
	}

	return results, nil
}

// checkDeployment probes a single running remote deployment.
func (p *ProcessHealthPoller) checkDeployment(ctx context.Context, dep *Deployment) (*HealthCheckResult, error) {
	srv, err := p.serverRepo.GetByID(ctx, dep.ServerID)
	if err != nil {
		return nil, fmt.Errorf("lookup server %s: %w", dep.ServerID, err)
	}

	decryptedKey, err := p.keyVault.Decrypt(srv.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt server ssh key: %w", err)
	}

	// 1. Connect to remote server over SSH
	client, err := p.sshManager.GetClient(ctx, srv, decryptedKey)
	if err != nil {
		// Network disconnect / SSH connection failure
		p.mu.Lock()
		p.consecErrors[dep.ID]++
		attempt := p.consecErrors[dep.ID]
		p.mu.Unlock()

		// Evict potentially stale/broken connection from pool so next cycle dials fresh
		_ = p.sshManager.CloseServer(dep.ServerID)

		// Append transient connection log warning
		_ = p.store.AppendLog(ctx, &LogEntry{
			DeploymentID: dep.ID,
			Stream:       StreamSystem,
			Message:      fmt.Sprintf("SSH connection to %s (%s) interrupted (%v); auto-reconnect scheduled (attempt %d)", srv.Name, srv.Host, err, attempt),
			Timestamp:    time.Now().UTC(),
		})

		return &HealthCheckResult{
			DeploymentID:   dep.ID,
			ServerID:       dep.ServerID,
			PID:            dep.ProcessPID,
			Status:         dep.Status,
			PreviousStatus: dep.Status,
			Alive:          false,
			Error:          fmt.Sprintf("ssh connection failed (attempt %d): %v", attempt, err),
			ActionTaken:    "reconnect_scheduled",
		}, nil
	}

	// Connection succeeded — reset consecutive error counter
	p.mu.Lock()
	hadErrors := p.consecErrors[dep.ID] > 0
	p.consecErrors[dep.ID] = 0
	p.mu.Unlock()

	if hadErrors {
		_ = p.store.AppendLog(ctx, &LogEntry{
			DeploymentID: dep.ID,
			Stream:       StreamSystem,
			Message:      fmt.Sprintf("SSH connection to server %s (%s) restored successfully", srv.Name, srv.Host),
			Timestamp:    time.Now().UTC(),
		})
	}

	// 2. Query process liveness on remote server
	remoteDir := fmt.Sprintf("cuan-apps/%s", dep.ID)
	probeScript := fmt.Sprintf(`# cuan-health-poll
PID=$(cat ~/%s/app.pid 2>/dev/null || echo %d)
if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    echo "ALIVE:$PID"
else
    if [ -f ~/%s/app.pid ]; then
        echo "CRASHED:$PID"
    else
        echo "STOPPED:$PID"
    fi
fi`, remoteDir, dep.ProcessPID, remoteDir)

	stdout, stderr, err := client.RunCommand(ctx, probeScript)
	if err != nil {
		return nil, fmt.Errorf("execute health probe script: %w (stderr: %s)", err, stderr)
	}

	out := strings.TrimSpace(stdout)
	switch {
	case strings.HasPrefix(out, "ALIVE"):
		// Process is running and healthy
		pidStr := strings.TrimPrefix(out, "ALIVE:")
		alivePID, _ := strconv.Atoi(pidStr)
		if alivePID > 0 && alivePID != dep.ProcessPID {
			dep.ProcessPID = alivePID
			_ = p.store.UpdateDeployment(ctx, dep)
		}

		// Optional check: verify port responsiveness
		portOpen := false
		if dep.InternalPort > 0 {
			portCmd := fmt.Sprintf("nc -z 127.0.0.1 %d 2>/dev/null || (echo > /dev/tcp/127.0.0.1/%d) 2>/dev/null && echo OPEN || echo CLOSED", dep.InternalPort, dep.InternalPort)
			pOut, _, _ := client.RunCommand(ctx, portCmd)
			if strings.TrimSpace(pOut) == "OPEN" {
				portOpen = true
			}
		}

		return &HealthCheckResult{
			DeploymentID:   dep.ID,
			ServerID:       dep.ServerID,
			PID:            dep.ProcessPID,
			Status:         StatusRunning,
			PreviousStatus: StatusRunning,
			Alive:          true,
			Port:           dep.InternalPort,
			PortOpen:       portOpen,
			ActionTaken:    "none",
		}, nil

	case strings.HasPrefix(out, "CRASHED"):
		// Process died unexpectedly while app.pid remained
		oldPID := dep.ProcessPID
		dep.Status = StatusCrashed
		dep.ProcessPID = 0
		dep.UpdatedAt = time.Now().UTC()
		_ = p.store.UpdateDeployment(ctx, dep)

		// Release port lease
		if p.portPool != nil {
			p.portPool.Release(dep.ID)
		}

		_ = p.store.AppendLog(ctx, &LogEntry{
			DeploymentID: dep.ID,
			Stream:       StreamSystem,
			Message:      fmt.Sprintf("Process health poller detected crash of remote process (PID %d). Deployment status set to crashed.", oldPID),
			Timestamp:    time.Now().UTC(),
		})

		p.mu.RLock()
		cb := p.onStateChange
		p.mu.RUnlock()
		if cb != nil {
			cb(dep.ID, StatusRunning, StatusCrashed)
		}

		return &HealthCheckResult{
			DeploymentID:   dep.ID,
			ServerID:       dep.ServerID,
			PID:            oldPID,
			Status:         StatusCrashed,
			PreviousStatus: StatusRunning,
			Alive:          false,
			ActionTaken:    "marked_crashed",
		}, nil

	case strings.HasPrefix(out, "STOPPED"):
		// Process stopped cleanly or app.pid missing
		oldPID := dep.ProcessPID
		dep.Status = StatusStopped
		dep.ProcessPID = 0
		dep.UpdatedAt = time.Now().UTC()
		_ = p.store.UpdateDeployment(ctx, dep)

		// Release port lease
		if p.portPool != nil {
			p.portPool.Release(dep.ID)
		}

		_ = p.store.AppendLog(ctx, &LogEntry{
			DeploymentID: dep.ID,
			Stream:       StreamSystem,
			Message:      fmt.Sprintf("Process health poller detected remote process termination (PID %d). Deployment status set to stopped.", oldPID),
			Timestamp:    time.Now().UTC(),
		})

		p.mu.RLock()
		cb := p.onStateChange
		p.mu.RUnlock()
		if cb != nil {
			cb(dep.ID, StatusRunning, StatusStopped)
		}

		return &HealthCheckResult{
			DeploymentID:   dep.ID,
			ServerID:       dep.ServerID,
			PID:            oldPID,
			Status:         StatusStopped,
			PreviousStatus: StatusRunning,
			Alive:          false,
			ActionTaken:    "marked_stopped",
		}, nil

	default:
		return &HealthCheckResult{
			DeploymentID:   dep.ID,
			ServerID:       dep.ServerID,
			PID:            dep.ProcessPID,
			Status:         dep.Status,
			PreviousStatus: dep.Status,
			Alive:          false,
			Error:          fmt.Sprintf("unexpected probe output: %q", out),
			ActionTaken:    "none",
		}, nil
	}
}

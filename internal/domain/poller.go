package domain

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

// DomainPollResult represents the outcome of probing a single custom domain.
type DomainPollResult struct {
	DomainID     string              `json:"domain_id"`
	DomainName   string              `json:"domain_name"`
	DeploymentID string              `json:"deployment_id"`
	OldDNSStatus string              `json:"old_dns_status"`
	NewDNSStatus string              `json:"new_dns_status"`
	Result       *VerificationResult `json:"result"`
	RouteUpdated bool                `json:"route_updated"`
	Error        string              `json:"error,omitempty"`
}

// DomainStateChangeFunc is called when a custom domain's DNS status changes during background polling.
type DomainStateChangeFunc func(domain cuanDB.CustomDomain, oldDNSStatus, newDNSStatus string)

// DNSHealthPoller periodically inspects custom domains, verifies DNS ownership and propagation,
// updates domain status in the database, and dynamically registers Caddy reverse proxy routes upon verification.
type DNSHealthPoller struct {
	domainRepo    cuanDB.DomainRepository
	verifier      *DNSVerifier
	depDriver     deployer.DeployDriver
	serverRepo    cuanDB.ServerRepository
	caddyRouter   *deployer.CaddyRouter
	interval      time.Duration
	onlyPending   bool
	onStateChange DomainStateChangeFunc

	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.RWMutex
	running bool
}

// PollerOption configures a DNSHealthPoller.
type PollerOption func(*DNSHealthPoller)

// WithPollerInterval sets the background polling interval.
func WithPollerInterval(d time.Duration) PollerOption {
	return func(p *DNSHealthPoller) {
		if d > 0 {
			p.interval = d
		}
	}
}

// WithPollOnlyPending restricts polling to domains with 'pending' or 'failed' status.
func WithPollOnlyPending(onlyPending bool) PollerOption {
	return func(p *DNSHealthPoller) {
		p.onlyPending = onlyPending
	}
}

// WithDomainStateChangeCallback sets a callback fired on domain DNS status transition.
func WithDomainStateChangeCallback(fn DomainStateChangeFunc) PollerOption {
	return func(p *DNSHealthPoller) {
		p.onStateChange = fn
	}
}

// NewDNSHealthPoller creates a new background DNS health poller.
func NewDNSHealthPoller(
	domainRepo cuanDB.DomainRepository,
	verifier *DNSVerifier,
	depDriver deployer.DeployDriver,
	serverRepo cuanDB.ServerRepository,
	caddyRouter *deployer.CaddyRouter,
	opts ...PollerOption,
) *DNSHealthPoller {
	p := &DNSHealthPoller{
		domainRepo:  domainRepo,
		verifier:    verifier,
		depDriver:   depDriver,
		serverRepo:  serverRepo,
		caddyRouter: caddyRouter,
		interval:    30 * time.Second,
		onlyPending: false,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Interval returns the current polling interval.
func (p *DNSHealthPoller) Interval() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.interval
}

// SetInterval updates the polling interval dynamically.
func (p *DNSHealthPoller) SetInterval(d time.Duration) {
	if d > 0 {
		p.mu.Lock()
		p.interval = d
		p.mu.Unlock()
	}
}

// IsRunning reports whether the background worker is currently active.
func (p *DNSHealthPoller) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Start launches the background polling goroutine.
func (p *DNSHealthPoller) Start(parentCtx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("dns health poller is already running")
	}

	p.ctx, p.cancel = context.WithCancel(parentCtx)
	p.done = make(chan struct{})
	p.running = true
	p.mu.Unlock()

	go p.runLoop()
	return nil
}

// Stop terminates the background polling worker and waits for clean exit.
func (p *DNSHealthPoller) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.cancel()
	p.running = false
	p.mu.Unlock()

	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		log.Println("Warning: dns health poller shutdown timed out")
	}
}

// runLoop executes the periodic polling ticker.
func (p *DNSHealthPoller) runLoop() {
	defer close(p.done)

	p.mu.RLock()
	interval := p.interval
	p.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial immediate check
	_, _ = p.PollOnce(p.ctx)

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			_, _ = p.PollOnce(p.ctx)
		}
	}
}

// PollOnce executes a single synchronous inspection pass over registered custom domains.
func (p *DNSHealthPoller) PollOnce(ctx context.Context) ([]*DomainPollResult, error) {
	if p.domainRepo == nil || p.verifier == nil {
		return nil, nil
	}

	domains, err := p.domainRepo.ListAllCustomDomains(ctx)
	if err != nil {
		return nil, fmt.Errorf("list custom domains for polling: %w", err)
	}

	var results []*DomainPollResult

	for _, cd := range domains {
		// If onlyPending is enabled, skip already verified domains
		if p.onlyPending && cd.DNSStatus == "verified" {
			continue
		}

		res := p.checkDomain(ctx, cd)
		results = append(results, res)
	}

	return results, nil
}

// checkDomain probes a single domain, updates database state, and syncs Caddy router.
func (p *DNSHealthPoller) checkDomain(ctx context.Context, cd cuanDB.CustomDomain) *DomainPollResult {
	oldStatus := cd.DNSStatus
	vRes, err := p.verifier.VerifyDomain(ctx, cd.Domain, cd.VerificationToken)
	now := time.Now().UTC()

	pollRes := &DomainPollResult{
		DomainID:     cd.ID,
		DomainName:   cd.Domain,
		DeploymentID: cd.DeploymentID,
		OldDNSStatus: oldStatus,
		Result:       vRes,
	}

	if err != nil {
		pollRes.Error = err.Error()
		pollRes.NewDNSStatus = oldStatus
		return pollRes
	}

	newStatus := "failed"
	if vRes.Verified {
		newStatus = "verified"
	}
	pollRes.NewDNSStatus = newStatus

	// Update database if status changed or last_checked_at is older
	if newStatus != oldStatus || cd.LastCheckedAt == nil {
		_ = p.domainRepo.UpdateCustomDomainStatus(ctx, cd.ID, newStatus, cd.SSLStatus, &now)
		if p.onStateChange != nil && newStatus != oldStatus {
			cd.DNSStatus = newStatus
			cd.LastCheckedAt = &now
			p.onStateChange(cd, oldStatus, newStatus)
		}
	} else {
		// Always refresh last_checked_at
		_ = p.domainRepo.UpdateCustomDomainStatus(ctx, cd.ID, newStatus, cd.SSLStatus, &now)
	}

	// Dynamic Caddy Route Sync:
	// If the domain is now verified and caddyRouter is available, ensure Caddy route is registered
	if newStatus == "verified" && p.caddyRouter != nil && p.depDriver != nil {
		dep, depErr := p.depDriver.Get(ctx, cd.DeploymentID)
		if depErr == nil && dep != nil {
			target := deployer.LocalTarget(dep.InternalPort)
			if dep.ServerID != "" && p.serverRepo != nil {
				srv, srvErr := p.serverRepo.GetByID(ctx, dep.ServerID)
				if srvErr == nil && srv != nil && srv.Host != "" {
					target = deployer.RemoteTarget(srv.Host, dep.InternalPort)
				}
			}
			if err := p.caddyRouter.AddCustomDomainRoute(ctx, cd.Domain, target); err == nil {
				pollRes.RouteUpdated = true
			}
		}
	}

	return pollRes
}

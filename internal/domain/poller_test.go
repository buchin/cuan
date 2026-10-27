package domain

import (
	"context"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

// mockDeployDriver provides a minimal implementation of DeployDriver for testing.
type mockDeployDriver struct {
	deployments map[string]*deployer.Deployment
}

func newMockDeployDriver() *mockDeployDriver {
	return &mockDeployDriver{
		deployments: make(map[string]*deployer.Deployment),
	}
}

func (m *mockDeployDriver) Deploy(ctx context.Context, req deployer.DeploymentRequest) (*deployer.DeploymentResult, error) {
	return nil, nil
}
func (m *mockDeployDriver) Start(ctx context.Context, id string) error   { return nil }
func (m *mockDeployDriver) Stop(ctx context.Context, id string) error    { return nil }
func (m *mockDeployDriver) Restart(ctx context.Context, id string) error { return nil }
func (m *mockDeployDriver) Health(ctx context.Context, id string) (*deployer.HealthStatus, error) {
	return &deployer.HealthStatus{Alive: true}, nil
}
func (m *mockDeployDriver) GetLogs(ctx context.Context, id string, limit int) ([]deployer.LogEntry, error) {
	return nil, nil
}
func (m *mockDeployDriver) List(ctx context.Context) ([]deployer.Deployment, error) {
	var list []deployer.Deployment
	for _, d := range m.deployments {
		list = append(list, *d)
	}
	return list, nil
}
func (m *mockDeployDriver) Get(ctx context.Context, id string) (*deployer.Deployment, error) {
	if d, ok := m.deployments[id]; ok {
		return d, nil
	}
	return nil, deployer.ErrDeploymentNotFound
}
func (m *mockDeployDriver) Delete(ctx context.Context, id string) error { return nil }

func TestDNSHealthPoller_PollOnce(t *testing.T) {
	repo := cuanDB.NewMemoryDomainRepository()
	mockDNS := NewMockDNSResolver()
	verifier := NewDNSVerifier(DNSVerifierConfig{
		CanonicalHosts: []string{"cuan.app"},
	}, WithResolver(mockDNS))

	// Setup mock deployments and Caddy router
	depDriver := newMockDeployDriver()
	depDriver.deployments["dep_test_1"] = &deployer.Deployment{
		ID:           "dep_test_1",
		ProjectName:  "Test Project",
		Slug:         "test-proj",
		InternalPort: 3001,
		Status:       "running",
	}

	caddyRouter := deployer.NewCaddyRouter(deployer.CaddyConfig{
		BaseDomain: "cuan.app",
	})

	// Add domain 1: will match CNAME
	_ = repo.CreateCustomDomain(context.Background(), &cuanDB.CustomDomain{
		ID:                "dom_1",
		DeploymentID:      "dep_test_1",
		Domain:            "verified.app.com",
		DNSStatus:         "pending",
		VerificationToken: "token_1",
	})
	mockDNS.SetCNAME("verified.app.com", "cuan.app.")

	// Add domain 2: will fail (no records)
	_ = repo.CreateCustomDomain(context.Background(), &cuanDB.CustomDomain{
		ID:                "dom_2",
		DeploymentID:      "dep_test_1",
		Domain:            "unverified.app.com",
		DNSStatus:         "pending",
		VerificationToken: "token_2",
	})

	var stateChanges []string
	poller := NewDNSHealthPoller(
		repo,
		verifier,
		depDriver,
		nil,
		caddyRouter,
		WithPollerInterval(100*time.Millisecond),
		WithDomainStateChangeCallback(func(d cuanDB.CustomDomain, oldStatus, newStatus string) {
			stateChanges = append(stateChanges, d.Domain+":"+oldStatus+"->"+newStatus)
		}),
	)

	results, err := poller.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error during PollOnce: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 poll results, got %d", len(results))
	}

	// Verify dom_1 updated to verified
	cd1, err := repo.GetCustomDomainByID(context.Background(), "dom_1")
	if err != nil || cd1.DNSStatus != "verified" {
		t.Fatalf("expected dom_1 to be verified, got status: %s (err: %v)", cd1.DNSStatus, err)
	}

	// Verify dom_2 updated to failed
	cd2, err := repo.GetCustomDomainByID(context.Background(), "dom_2")
	if err != nil || cd2.DNSStatus != "failed" {
		t.Fatalf("expected dom_2 to be failed, got status: %s (err: %v)", cd2.DNSStatus, err)
	}

	// Verify Caddy route registered for verified.app.com
	hasRoute, err := caddyRouter.GetCustomDomainRouteStatus(context.Background(), "verified.app.com")
	if err != nil || !hasRoute {
		t.Fatalf("expected caddy route to be registered for verified.app.com, err: %v", err)
	}

	// Verify state change callbacks were fired
	if len(stateChanges) < 2 {
		t.Fatalf("expected at least 2 state changes, got: %v", stateChanges)
	}
}

func TestDNSHealthPoller_StartAndStop(t *testing.T) {
	repo := cuanDB.NewMemoryDomainRepository()
	verifier := NewDNSVerifier(DNSVerifierConfig{}, WithResolver(NewMockDNSResolver()))
	poller := NewDNSHealthPoller(repo, verifier, nil, nil, nil, WithPollerInterval(50*time.Millisecond))

	if poller.IsRunning() {
		t.Fatalf("expected poller not running initially")
	}

	ctx := context.Background()
	err := poller.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start poller: %v", err)
	}

	if !poller.IsRunning() {
		t.Fatalf("expected poller running after Start")
	}

	// Second start should error
	err = poller.Start(ctx)
	if err == nil {
		t.Fatalf("expected error starting already running poller")
	}

	poller.SetInterval(200 * time.Millisecond)
	if poller.Interval() != 200*time.Millisecond {
		t.Fatalf("expected interval 200ms, got %v", poller.Interval())
	}

	poller.Stop()
	if poller.IsRunning() {
		t.Fatalf("expected poller not running after Stop")
	}
}

func TestDNSHealthPoller_OnlyPending(t *testing.T) {
	repo := cuanDB.NewMemoryDomainRepository()
	mockDNS := NewMockDNSResolver()
	verifier := NewDNSVerifier(DNSVerifierConfig{}, WithResolver(mockDNS))

	_ = repo.CreateCustomDomain(context.Background(), &cuanDB.CustomDomain{
		ID:                "dom_already_verified",
		DeploymentID:      "dep_1",
		Domain:            "already.verified.org",
		DNSStatus:         "verified",
		VerificationToken: "token_verified",
	})

	poller := NewDNSHealthPoller(
		repo,
		verifier,
		nil,
		nil,
		nil,
		WithPollOnlyPending(true),
	)

	results, err := poller.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results with onlyPending=true, got %d", len(results))
	}
}

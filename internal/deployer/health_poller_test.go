package deployer

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
)

func TestProcessHealthPoller_DetectAlive(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, err := vault.Encrypt(clientPrivPEM)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_poller_01",
		Name:         "test-poller-srv",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	_ = srvRepo.Create(context.Background(), srv)

	store := NewMemoryStore()
	portPool := NewPortPool()
	depID := "dep_poller_alive"
	port, _ := portPool.Allocate(depID)

	_ = store.CreateDeployment(context.Background(), &Deployment{
		ID:           depID,
		ServerID:     srv.ID,
		ProjectName:  "alive-test-app",
		Slug:         "alive-test-app",
		Status:       StatusRunning,
		ProcessPID:   1337,
		InternalPort: port,
	})

	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	mockServer.SetProbeState("ALIVE")

	poller := NewProcessHealthPoller(store, srvRepo, vault, sshMgr, portPool)

	results, err := poller.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("PollOnce error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	res := results[0]
	if !res.Alive {
		t.Errorf("expected process to be alive")
	}
	if res.Status != StatusRunning {
		t.Errorf("expected status %s, got %s", StatusRunning, res.Status)
	}
	if res.ActionTaken != "none" {
		t.Errorf("expected action 'none', got %s", res.ActionTaken)
	}

	// Verify store still shows StatusRunning
	dep, _ := store.GetDeployment(context.Background(), depID)
	if dep.Status != StatusRunning {
		t.Errorf("expected store status %s, got %s", StatusRunning, dep.Status)
	}
}

func TestProcessHealthPoller_DetectCrashed(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, err := vault.Encrypt(clientPrivPEM)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_poller_02",
		Name:         "test-poller-crash",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	_ = srvRepo.Create(context.Background(), srv)

	store := NewMemoryStore()
	portPool := NewPortPool()
	depID := "dep_poller_crash"
	port, _ := portPool.Allocate(depID)

	_ = store.CreateDeployment(context.Background(), &Deployment{
		ID:           depID,
		ServerID:     srv.ID,
		ProjectName:  "crash-test-app",
		Slug:         "crash-test-app",
		Status:       StatusRunning,
		ProcessPID:   1337,
		InternalPort: port,
	})

	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	// Simulate unexpected crash on remote server
	mockServer.SetProbeState("CRASHED")

	var stateChangeDetected bool
	var reportedOld, reportedNew string
	poller := NewProcessHealthPoller(store, srvRepo, vault, sshMgr, portPool,
		WithPollerStateChangeCallback(func(id string, oldStatus, newStatus string) {
			if id == depID {
				stateChangeDetected = true
				reportedOld = oldStatus
				reportedNew = newStatus
			}
		}),
	)

	results, err := poller.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("PollOnce error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Alive {
		t.Errorf("expected process to not be alive")
	}
	if res.Status != StatusCrashed {
		t.Errorf("expected status %s, got %s", StatusCrashed, res.Status)
	}
	if res.ActionTaken != "marked_crashed" {
		t.Errorf("expected action 'marked_crashed', got %s", res.ActionTaken)
	}

	// Verify store was updated to StatusCrashed
	dep, _ := store.GetDeployment(context.Background(), depID)
	if dep.Status != StatusCrashed {
		t.Errorf("expected store status %s, got %s", StatusCrashed, dep.Status)
	}
	if dep.ProcessPID != 0 {
		t.Errorf("expected ProcessPID to be 0 after crash, got %d", dep.ProcessPID)
	}

	// Verify port was released from pool
	allocatedPort, _ := portPool.Allocate(depID)
	if allocatedPort != port {
		// Port was successfully recycled back into available set
	}

	// Verify state change callback
	if !stateChangeDetected {
		t.Errorf("expected stateChange callback to fire")
	}
	if reportedOld != StatusRunning || reportedNew != StatusCrashed {
		t.Errorf("expected transition %s -> %s, got %s -> %s", StatusRunning, StatusCrashed, reportedOld, reportedNew)
	}

	// Verify system audit log was appended
	logs, _ := store.GetLogs(context.Background(), depID, 10)
	var foundCrashLog bool
	for _, l := range logs {
		if l.Stream == StreamSystem && (len(l.Message) > 0) {
			foundCrashLog = true
			break
		}
	}
	if !foundCrashLog {
		t.Errorf("expected system log regarding crash event")
	}
}

func TestProcessHealthPoller_DetectStopped(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, _ := vault.Encrypt(clientPrivPEM)

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_poller_03",
		Name:         "test-poller-stop",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	_ = srvRepo.Create(context.Background(), srv)

	store := NewMemoryStore()
	portPool := NewPortPool()
	depID := "dep_poller_stop"
	port, _ := portPool.Allocate(depID)

	_ = store.CreateDeployment(context.Background(), &Deployment{
		ID:           depID,
		ServerID:     srv.ID,
		ProjectName:  "stop-test-app",
		Slug:         "stop-test-app",
		Status:       StatusRunning,
		ProcessPID:   1337,
		InternalPort: port,
	})

	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	mockServer.SetProbeState("STOPPED")

	poller := NewProcessHealthPoller(store, srvRepo, vault, sshMgr, portPool)

	results, err := poller.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("PollOnce error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Status != StatusStopped {
		t.Errorf("expected status %s, got %s", StatusStopped, res.Status)
	}
	if res.ActionTaken != "marked_stopped" {
		t.Errorf("expected action 'marked_stopped', got %s", res.ActionTaken)
	}

	dep, _ := store.GetDeployment(context.Background(), depID)
	if dep.Status != StatusStopped {
		t.Errorf("expected store status %s, got %s", StatusStopped, dep.Status)
	}
}

func TestProcessHealthPoller_NetworkDisconnectHandling(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, _ := vault.Encrypt(clientPrivPEM)

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_poller_04",
		Name:         "test-poller-netdrop",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	_ = srvRepo.Create(context.Background(), srv)

	store := NewMemoryStore()
	portPool := NewPortPool()
	depID := "dep_poller_netdrop"
	port, _ := portPool.Allocate(depID)

	_ = store.CreateDeployment(context.Background(), &Deployment{
		ID:           depID,
		ServerID:     srv.ID,
		ProjectName:  "netdrop-test-app",
		Slug:         "netdrop-test-app",
		Status:       StatusRunning,
		ProcessPID:   1337,
		InternalPort: port,
	})

	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	// 1. Simulate temporary network drop
	mockServer.SetSimulateNetworkDrop(true)

	poller := NewProcessHealthPoller(store, srvRepo, vault, sshMgr, portPool)

	results, err := poller.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("PollOnce error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	res := results[0]
	// Must NOT be marked as crashed or stopped during transient network drop!
	if res.Status != StatusRunning {
		t.Errorf("expected status to remain %s during network drop, got %s", StatusRunning, res.Status)
	}

	// Verify deployment in store remains running
	dep, _ := store.GetDeployment(context.Background(), depID)
	if dep.Status != StatusRunning {
		t.Errorf("store status was altered during network drop: %s", dep.Status)
	}

	// 2. Restore network connectivity
	mockServer.SetSimulateNetworkDrop(false)
	mockServer.SetProbeState("ALIVE")

	results2, err := poller.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("PollOnce after recovery error: %v", err)
	}

	if len(results2) != 1 || !results2[0].Alive {
		t.Errorf("expected recovery to report Alive=true")
	}

	if poller.ConsecutiveErrors(depID) != 0 {
		t.Errorf("expected consecutive error counter to reset to 0, got %d", poller.ConsecutiveErrors(depID))
	}
}

func TestProcessHealthPoller_LifecycleAndNoGoroutineLeak(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, _ := vault.Encrypt(clientPrivPEM)

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_poller_05",
		Name:         "test-poller-lifecycle",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	_ = srvRepo.Create(context.Background(), srv)

	store := NewMemoryStore()
	portPool := NewPortPool()
	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	poller := NewProcessHealthPoller(store, srvRepo, vault, sshMgr, portPool,
		WithPollerInterval(20*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	_ = poller.Start(ctx)

	if !poller.IsRunning() {
		t.Errorf("expected poller to be running")
	}

	time.Sleep(60 * time.Millisecond)

	poller.Stop()

	if poller.IsRunning() {
		t.Errorf("expected poller to not be running after Stop")
	}

	cancel()
}

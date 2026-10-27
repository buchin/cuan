package deployer

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
)

func TestRemoteLogStreamer_RealTimeStreaming(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, err := vault.Encrypt(clientPrivPEM)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_streamer_01",
		Name:         "test-streamer-srv",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	if err := srvRepo.Create(context.Background(), srv); err != nil {
		t.Fatalf("create server: %v", err)
	}

	store := NewMemoryStore()
	depID := "dep_stream_01"
	_ = store.CreateDeployment(context.Background(), &Deployment{
		ID:          depID,
		ServerID:    srv.ID,
		ProjectName: "streamer-test-app",
		Slug:        "streamer-test-app",
		Status:      StatusRunning,
	})

	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	streamer := NewRemoteLogStreamer(depID, srv.ID, srvRepo, vault, sshMgr, store,
		WithStreamerBufferSize(50),
		WithStreamerBackoff(50*time.Millisecond, 200*time.Millisecond),
	)

	// Add test log lines to mock SSH server
	mockServer.AddLogLine("[app] Booting application server v1.0.0...")
	mockServer.AddLogLine("[app] Listening on 0.0.0.0:9001")
	mockServer.AddLogLine("[http] GET /healthz 200 OK 1.2ms")

	// Subscribe to live log events
	subCh, unsub := streamer.Streamer().Subscribe()
	defer unsub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := streamer.Start(ctx); err != nil {
		t.Fatalf("start streamer: %v", err)
	}

	// Verify subscriber receives first entry promptly
	select {
	case entry, ok := <-subCh:
		if !ok {
			t.Fatalf("subscriber channel closed unexpectedly")
		}
		if entry.DeploymentID != depID {
			t.Errorf("expected deployment ID %s, got %s", depID, entry.DeploymentID)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for live stream entry on subscriber channel")
	}

	// Wait briefly for all lines to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify in-memory ring buffer
	recent := streamer.Streamer().GetRecent(10)
	if len(recent) < 2 {
		t.Errorf("expected at least 2 entries in ring buffer, got %d", len(recent))
	}

	// Verify store has persisted log entries
	storeLogs, err := store.GetLogs(context.Background(), depID, 10)
	if err != nil {
		t.Fatalf("get logs from store: %v", err)
	}
	if len(storeLogs) < 2 {
		t.Errorf("expected at least 2 logs persisted in store, got %d", len(storeLogs))
	}

	// Clean shutdown
	streamer.Stop()
	if streamer.IsRunning() {
		t.Errorf("expected streamer to be stopped")
	}
}

func TestRemoteLogStreamer_FetchSnapshot(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, err := vault.Encrypt(clientPrivPEM)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_snap_01",
		Name:         "test-snap-srv",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	if err := srvRepo.Create(context.Background(), srv); err != nil {
		t.Fatalf("create server: %v", err)
	}

	store := NewMemoryStore()
	depID := "dep_snap_01"
	_ = store.CreateDeployment(context.Background(), &Deployment{
		ID:          depID,
		ServerID:    srv.ID,
		ProjectName: "snapshot-test-app",
		Slug:        "snapshot-test-app",
		Status:      StatusRunning,
	})

	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	mockServer.AddLogLine("snapshot line 1: system ready")
	mockServer.AddLogLine("snapshot line 2: connected to db")

	streamer := NewRemoteLogStreamer(depID, srv.ID, srvRepo, vault, sshMgr, store)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	entries, err := streamer.FetchSnapshot(ctx, 10)
	if err != nil {
		t.Fatalf("FetchSnapshot error: %v", err)
	}

	if len(entries) < 2 {
		t.Errorf("expected at least 2 snapshot entries, got %d", len(entries))
	}

	// Verify entries were saved to store
	storeLogs, err := store.GetLogs(context.Background(), depID, 10)
	if err != nil {
		t.Fatalf("store.GetLogs error: %v", err)
	}
	if len(storeLogs) < 2 {
		t.Errorf("expected store to contain snapshot logs, got %d", len(storeLogs))
	}
}

func TestRemoteLogStreamer_NetworkDisconnectAndAutoReconnect(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, err := vault.Encrypt(clientPrivPEM)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_reconnect_01",
		Name:         "test-reconnect-srv",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	if err := srvRepo.Create(context.Background(), srv); err != nil {
		t.Fatalf("create server: %v", err)
	}

	store := NewMemoryStore()
	depID := "dep_reconnect_01"
	_ = store.CreateDeployment(context.Background(), &Deployment{
		ID:          depID,
		ServerID:    srv.ID,
		ProjectName: "reconnect-test-app",
		Slug:        "reconnect-test-app",
		Status:      StatusRunning,
	})

	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	// Simulate disconnect on first tail connection
	mockServer.SetDisconnectOnTail(true)
	mockServer.AddLogLine("reconnected message after network recovery")

	streamer := NewRemoteLogStreamer(depID, srv.ID, srvRepo, vault, sshMgr, store,
		WithStreamerBackoff(30*time.Millisecond, 100*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := streamer.Start(ctx); err != nil {
		t.Fatalf("start streamer: %v", err)
	}

	// Wait for reconnect cycle
	deadline := time.Now().Add(3 * time.Second)
	var logs []LogEntry
	for time.Now().Before(deadline) {
		logs, _ = store.GetLogs(context.Background(), depID, 10)
		if len(logs) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(logs) == 0 {
		t.Fatalf("streamer failed to reconnect and capture logs after disconnect")
	}

	streamer.Stop()
}

func TestRemoteLogStreamer_CleanShutdown(t *testing.T) {
	mockServer, clientPrivPEM, _ := newMockDriverSSHServer(t)
	defer mockServer.Close()

	vault := crypto.DefaultKeyVault()
	encKey, err := vault.Encrypt(clientPrivPEM)
	if err != nil {
		t.Fatalf("encrypt key: %v", err)
	}

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_shutdown_01",
		Name:         "test-shutdown-srv",
		Host:         mockServer.addr,
		Port:         mockServer.port,
		Username:     "testuser",
		AuthType:     "key",
		EncryptedKey: encKey,
	}
	_ = srvRepo.Create(context.Background(), srv)

	store := NewMemoryStore()
	depID := "dep_shutdown_01"
	_ = store.CreateDeployment(context.Background(), &Deployment{
		ID:          depID,
		ServerID:    srv.ID,
		ProjectName: "shutdown-test-app",
		Slug:        "shutdown-test-app",
		Status:      StatusRunning,
	})

	sshMgr := NewClientPool(500 * time.Millisecond)
	defer sshMgr.CloseAll()

	streamer := NewRemoteLogStreamer(depID, srv.ID, srvRepo, vault, sshMgr, store)

	ctx, cancel := context.WithCancel(context.Background())
	_ = streamer.Start(ctx)

	if !streamer.IsRunning() {
		t.Fatalf("expected streamer to be running")
	}

	// Stop cleanly via Stop()
	streamer.Stop()

	if streamer.IsRunning() {
		t.Errorf("expected streamer to not be running after Stop")
	}

	cancel()
}

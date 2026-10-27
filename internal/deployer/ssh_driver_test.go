package deployer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
	"golang.org/x/crypto/ssh"
)

// mockSSHExecution captures commands and files received by mock SSH server.
type mockSSHExecution struct {
	mu                  sync.Mutex
	commands            []string
	extractedFiles      map[string][]byte
	processAlive        bool
	pid                 int
	portOpen            bool
	forceExecError      bool
	logLines            []string
	streamChan          chan string
	disconnectOnTail    bool
	tailCount           int
	probeState          string
	simulateNetworkDrop bool
}

type mockDriverSSHServer struct {
	listener       net.Listener
	addr           string
	port           int
	hostSigner     ssh.Signer
	clientFinger   string
	execState      *mockSSHExecution
	stopChan       chan struct{}
	mu             sync.Mutex
	closed         bool
	conns          []net.Conn
}

func newMockDriverSSHServer(t *testing.T) (*mockDriverSSHServer, []byte, string) {
	t.Helper()

	// 1. Host key
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("signer from host key: %v", err)
	}

	// 2. Client key
	clientPub, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	clientPrivBytes, err := x509.MarshalPKCS8PrivateKey(clientPriv)
	if err != nil {
		t.Fatalf("marshal client key: %v", err)
	}
	clientPrivPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: clientPrivBytes,
	})

	sshClientPub, err := ssh.NewPublicKey(clientPub)
	if err != nil {
		t.Fatalf("ssh public key: %v", err)
	}
	clientFingerprint := ssh.FingerprintSHA256(sshClientPub)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock ssh: %v", err)
	}
	tcpAddr := listener.Addr().(*net.TCPAddr)

	execState := &mockSSHExecution{
		extractedFiles: make(map[string][]byte),
		processAlive:   true,
		pid:            1337,
		portOpen:       true,
	}

	ms := &mockDriverSSHServer{
		listener:     listener,
		addr:         "127.0.0.1",
		port:         tcpAddr.Port,
		hostSigner:   hostSigner,
		clientFinger: clientFingerprint,
		execState:    execState,
		stopChan:     make(chan struct{}),
	}

	go ms.acceptLoop()
	return ms, clientPrivPEM, clientFingerprint
}

func (s *mockDriverSSHServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns = append(s.conns, conn)
		s.mu.Unlock()

		go s.handleConnection(conn)
	}
}

func (s *mockDriverSSHServer) handleConnection(netConn net.Conn) {
	defer netConn.Close()

	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if ssh.FingerprintSHA256(key) != s.clientFinger {
				return nil, errors.New("unauthorized key")
			}
			return nil, nil
		},
	}
	sshConfig.AddHostKey(s.hostSigner)

	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, sshConfig)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}

		go func(ch ssh.Channel, inReqs <-chan *ssh.Request) {
			defer ch.Close()
			for req := range inReqs {
				switch req.Type {
				case "exec":
					var payload struct {
						Command string
					}
					if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
						_ = req.Reply(false, nil)
						return
					}
					_ = req.Reply(true, nil)

					cmd := payload.Command
					s.execState.mu.Lock()
					s.execState.commands = append(s.execState.commands, cmd)
					forceErr := s.execState.forceExecError
					pid := s.execState.pid
					alive := s.execState.processAlive
					portOpen := s.execState.portOpen
					s.execState.mu.Unlock()

					if forceErr {
						_, _ = ch.Write([]byte("mock forced error\n"))
						status := struct{ Status uint32 }{1}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}

					// 1. Tar extraction over stdin
					if strings.Contains(cmd, "tar -xf -") {
						tr := tar.NewReader(ch)
						for {
							hdr, err := tr.Next()
							if err == io.EOF {
								break
							}
							if err != nil {
								break
							}
							var content bytes.Buffer
							_, _ = io.Copy(&content, tr)
							s.execState.mu.Lock()
							s.execState.extractedFiles[hdr.Name] = content.Bytes()
							s.execState.mu.Unlock()
						}
						status := struct{ Status uint32 }{0}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}

					// 2. Health probe script from ProcessHealthPoller
					if strings.Contains(cmd, "cuan-health-poll") {
						s.execState.mu.Lock()
						netDrop := s.execState.simulateNetworkDrop
						pState := s.execState.probeState
						pidVal := s.execState.pid
						s.execState.mu.Unlock()

						if netDrop {
							_ = ch.Close()
							return
						}

						if pState == "" {
							pState = "ALIVE"
						}

						switch pState {
						case "CRASHED":
							_, _ = ch.Write([]byte(fmt.Sprintf("CRASHED:%d\n", pidVal)))
						case "STOPPED":
							_, _ = ch.Write([]byte(fmt.Sprintf("STOPPED:%d\n", pidVal)))
						default:
							_, _ = ch.Write([]byte(fmt.Sprintf("ALIVE:%d\n", pidVal)))
						}

						status := struct{ Status uint32 }{0}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}

					// 2b. Read PID
					if strings.Contains(cmd, "cat ") && strings.Contains(cmd, "app.pid") {
						_, _ = ch.Write([]byte(fmt.Sprintf("%d\n", pid)))
						status := struct{ Status uint32 }{0}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}

					// 3. Process liveness probe
					if strings.Contains(cmd, "kill -0") && strings.Contains(cmd, "ALIVE") {
						if alive {
							_, _ = ch.Write([]byte("ALIVE\n"))
						} else {
							_, _ = ch.Write([]byte("DEAD\n"))
						}
						status := struct{ Status uint32 }{0}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}

					// 4. Port probe
					if strings.Contains(cmd, "OPEN") && strings.Contains(cmd, "CLOSED") {
						if portOpen {
							_, _ = ch.Write([]byte("OPEN\n"))
						} else {
							_, _ = ch.Write([]byte("CLOSED\n"))
						}
						status := struct{ Status uint32 }{0}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}

					// 5. Stop process
					if strings.Contains(cmd, "kill -15") || strings.Contains(cmd, "kill -9") {
						s.execState.mu.Lock()
						s.execState.processAlive = false
						s.execState.mu.Unlock()
						status := struct{ Status uint32 }{0}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}

					// 6. Delete directory
					if strings.Contains(cmd, "rm -rf") {
						s.execState.mu.Lock()
						s.execState.extractedFiles = make(map[string][]byte)
						s.execState.mu.Unlock()
						status := struct{ Status uint32 }{0}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}


					// 8. Tail app.log (Streaming / Snapshot)
					if strings.Contains(cmd, "tail -") || strings.Contains(cmd, "tail ") {
						s.execState.mu.Lock()
						s.execState.tailCount++
						disconnect := s.execState.disconnectOnTail && s.execState.tailCount == 1
						lines := append([]string{}, s.execState.logLines...)
						sChan := s.execState.streamChan
						s.execState.mu.Unlock()

						if disconnect {
							_ = ch.Close()
							return
						}

						for _, l := range lines {
							_, _ = ch.Write([]byte(l + "\n"))
						}

						if sChan != nil {
							for l := range sChan {
								_, _ = ch.Write([]byte(l + "\n"))
							}
						} else {
							time.Sleep(50 * time.Millisecond)
						}

						status := struct{ Status uint32 }{0}
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
						return
					}

					// Default success
					status := struct{ Status uint32 }{0}
					_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
					return

				default:
					_ = req.Reply(false, nil)
				}
			}
		}(channel, requests)
	}
}

func (s *mockDriverSSHServer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.stopChan)
	_ = s.listener.Close()
	for _, c := range s.conns {
		_ = c.Close()
	}
}

func (s *mockDriverSSHServer) SetProbeState(state string) {
	s.execState.mu.Lock()
	defer s.execState.mu.Unlock()
	s.execState.probeState = state
}

func (s *mockDriverSSHServer) SetSimulateNetworkDrop(drop bool) {
	s.execState.mu.Lock()
	defer s.execState.mu.Unlock()
	s.execState.simulateNetworkDrop = drop
}

func (s *mockDriverSSHServer) AddLogLine(line string) {
	s.execState.mu.Lock()
	defer s.execState.mu.Unlock()
	s.execState.logLines = append(s.execState.logLines, line)
}

func (s *mockDriverSSHServer) SetDisconnectOnTail(disconnect bool) {
	s.execState.mu.Lock()
	defer s.execState.mu.Unlock()
	s.execState.disconnectOnTail = disconnect
}

func (s *mockDriverSSHServer) SetStreamChan(ch chan string) {
	s.execState.mu.Lock()
	defer s.execState.mu.Unlock()
	s.execState.streamChan = ch
}

func createTestZipArchive(t *testing.T, files map[string]string) ([]byte, int64) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes(), int64(buf.Len())
}

// 1. Interface Compliance Test
func TestRemoteSSHDriver_InterfaceCompliance(t *testing.T) {
	var _ DeployDriver = (*RemoteSSHDriver)(nil)
	var _ DeployDriver = (*DriverFactory)(nil)
	var _ DeployDriver = (*LocalDriver)(nil)
}

// 2. Remote Path Traversal Defense Test
func TestRemotePathTraversalDefense(t *testing.T) {
	// ValidateDeploymentID
	invalidIDs := []string{
		"",
		"   ",
		"../escape",
		"dep/sub",
		"dep\\sub",
		"../../etc/passwd",
		"dep;rm -rf /",
		"dep$FOO",
		"dep`id`",
		"dep 123",
		"dep\x00null",
		"dep&whoami",
	}

	for _, id := range invalidIDs {
		err := ValidateDeploymentID(id)
		if err == nil {
			t.Errorf("expected ValidateDeploymentID(%q) to fail, got nil", id)
		}
		if id != "" && !strings.Contains(id, " ") && !errors.Is(err, ErrRemotePathTraversal) {
			t.Errorf("expected ErrRemotePathTraversal for %q, got: %v", id, err)
		}
	}

	validIDs := []string{
		"dep_12345",
		"my-app-01",
		"PROJECT_ABC",
		"deploy-99_ok",
	}

	for _, id := range validIDs {
		if err := ValidateDeploymentID(id); err != nil {
			t.Errorf("expected ValidateDeploymentID(%q) to succeed, got: %v", id, err)
		}
	}

	// ValidateRemotePath
	safePath, err := ValidateRemotePath("dep_valid")
	if err != nil {
		t.Fatalf("ValidateRemotePath failed: %v", err)
	}
	if safePath != "cuan-apps/dep_valid" {
		t.Errorf("expected cuan-apps/dep_valid, got %s", safePath)
	}

	safeSubpath, err := ValidateRemotePath("dep_valid", "config/app.json")
	if err != nil {
		t.Fatalf("ValidateRemotePath with subpath failed: %v", err)
	}
	if safeSubpath != "cuan-apps/dep_valid/config/app.json" {
		t.Errorf("expected cuan-apps/dep_valid/config/app.json, got %s", safeSubpath)
	}

	// Traversal subpaths
	traversalSubpaths := []string{
		"../../etc/passwd",
		"../other_dep",
		"sub/../../root",
		"sub\\escape",
		"sub\x00escape",
	}
	for _, sub := range traversalSubpaths {
		_, err := ValidateRemotePath("dep_valid", sub)
		if err == nil {
			t.Errorf("expected ValidateRemotePath to reject subpath %q", sub)
		}
		if !errors.Is(err, ErrRemotePathTraversal) {
			t.Errorf("expected ErrRemotePathTraversal for %q, got: %v", sub, err)
		}
	}
}

// 3. RemoteSSHDriver Full Lifecycle (Deploy -> Start -> Health -> Restart -> Stop -> Delete)
func TestRemoteSSHDriver_FullLifecycle(t *testing.T) {
	server, privKey, _ := newMockDriverSSHServer(t)
	defer server.Close()

	vault := crypto.NewKeyVault("test-secret-vault-1234")
	encKey, err := vault.Encrypt(privKey)
	if err != nil {
		t.Fatalf("encrypt privKey: %v", err)
	}

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_remote_1",
		Name:         "Worker VPS 1",
		Host:         server.addr,
		Port:         server.port,
		Username:     "ubuntu",
		AuthType:     "key",
		EncryptedKey: encKey,
		Status:       "online",
	}
	if err := srvRepo.Create(context.Background(), srv); err != nil {
		t.Fatalf("create server: %v", err)
	}

	store := NewMemoryStore()
	portPool := NewPortPool()
	caddyCfg := DefaultCaddyConfig()
	caddyCfg.BaseDomain = "cuan.test"
	router := NewCaddyRouter(caddyCfg)
	sshMgr := NewClientPool(2 * time.Second)
	defer sshMgr.CloseAll()

	driver := NewRemoteSSHDriver(srvRepo, vault, sshMgr, store, portPool, router)

	// Step 1: Deploy with Tar-over-SSH stream
	zipBytes, zipLen := createTestZipArchive(t, map[string]string{
		"index.php": "<?php echo 'cuan remote test'; ?>",
		"config.json": `{"app": "cuan-byos"}`,
	})

	ctx := context.Background()
	req := DeploymentRequest{
		ServerID:    srv.ID,
		ProjectName: "Remote PHP App",
		Slug:        "remote-php-app",
		SourceType:  SourceTypeZip,
		ZipReader:   bytes.NewReader(zipBytes),
		ZipSize:     zipLen,
		EnvVars: map[string]interface{}{
			"APP_ENV": "production",
		},
	}

	res, err := driver.Deploy(ctx, req)
	if err != nil {
		t.Fatalf("driver.Deploy failed: %v", err)
	}

	if res.Status != StatusPending {
		t.Errorf("expected status pending, got %s", res.Status)
	}
	if res.DetectedRuntime != RuntimePHP {
		t.Errorf("expected detected runtime php, got %s", res.DetectedRuntime)
	}
	if res.Deployment.ServerID != srv.ID {
		t.Errorf("expected ServerID %s, got %s", srv.ID, res.Deployment.ServerID)
	}

	// Verify mock server received extracted files
	server.execState.mu.Lock()
	if _, ok := server.execState.extractedFiles["index.php"]; !ok {
		t.Errorf("index.php was not extracted to remote mock server")
	}
	if _, ok := server.execState.extractedFiles["config.json"]; !ok {
		t.Errorf("config.json was not extracted to remote mock server")
	}
	server.execState.mu.Unlock()

	depID := res.Deployment.ID

	// Step 2: Start process
	if err := driver.Start(ctx, depID); err != nil {
		t.Fatalf("driver.Start failed: %v", err)
	}

	dep, err := store.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if dep.Status != StatusRunning {
		t.Errorf("expected status running, got %s", dep.Status)
	}
	if dep.ProcessPID != 1337 {
		t.Errorf("expected PID 1337, got %d", dep.ProcessPID)
	}
	if dep.InternalPort <= 0 {
		t.Errorf("expected valid internal port, got %d", dep.InternalPort)
	}

	// Step 3: Health check
	hs, err := driver.Health(ctx, depID)
	if err != nil {
		t.Fatalf("driver.Health failed: %v", err)
	}
	if !hs.Alive {
		t.Errorf("expected health alive=true, got false (error: %s)", hs.Error)
	}
	if hs.PID != 1337 {
		t.Errorf("expected health PID 1337, got %d", hs.PID)
	}
	if !hs.PortResponding {
		t.Errorf("expected health PortResponding=true")
	}

	// Step 4: Restart process
	if err := driver.Restart(ctx, depID); err != nil {
		t.Fatalf("driver.Restart failed: %v", err)
	}
	dep, _ = store.GetDeployment(ctx, depID)
	if dep.Status != StatusRunning {
		t.Errorf("expected status running after restart, got %s", dep.Status)
	}

	// Step 5: Stop process
	if err := driver.Stop(ctx, depID); err != nil {
		t.Fatalf("driver.Stop failed: %v", err)
	}
	dep, _ = store.GetDeployment(ctx, depID)
	if dep.Status != StatusStopped {
		t.Errorf("expected status stopped, got %s", dep.Status)
	}
	if dep.ProcessPID != 0 {
		t.Errorf("expected ProcessPID 0 after stop, got %d", dep.ProcessPID)
	}

	// Health after stop
	hsStopped, err := driver.Health(ctx, depID)
	if err != nil {
		t.Fatalf("health after stop failed: %v", err)
	}
	if hsStopped.Alive {
		t.Errorf("expected alive=false after stop, got true")
	}

	// Step 6: Delete
	if err := driver.Delete(ctx, depID); err != nil {
		t.Fatalf("driver.Delete failed: %v", err)
	}
	_, err = store.GetDeployment(ctx, depID)
	if !errors.Is(err, ErrDeploymentNotFound) {
		t.Errorf("expected ErrDeploymentNotFound after delete, got: %v", err)
	}
}

// 4. RemoteSSHDriver Error Handling (Invalid server, unreachable, zip slip)
func TestRemoteSSHDriver_ErrorHandling(t *testing.T) {
	server, privKey, _ := newMockDriverSSHServer(t)
	defer server.Close()

	vault := crypto.NewKeyVault("test-secret-vault-1234")
	encKey, _ := vault.Encrypt(privKey)

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_valid",
		Name:         "Valid Server",
		Host:         server.addr,
		Port:         server.port,
		Username:     "ubuntu",
		EncryptedKey: encKey,
	}
	_ = srvRepo.Create(context.Background(), srv)

	store := NewMemoryStore()
	portPool := NewPortPool()
	sshMgr := NewClientPool(1 * time.Second)
	defer sshMgr.CloseAll()
	driver := NewRemoteSSHDriver(srvRepo, vault, sshMgr, store, portPool, nil)

	ctx := context.Background()

	// 1. Missing server ID
	_, err := driver.Deploy(ctx, DeploymentRequest{
		ProjectName: "No Server App",
	})
	if !errors.Is(err, ErrInvalidServerID) {
		t.Errorf("expected ErrInvalidServerID, got: %v", err)
	}

	// 2. Non-existent server
	_, err = driver.Deploy(ctx, DeploymentRequest{
		ServerID:    "srv_nonexistent",
		ProjectName: "Bad Server App",
	})
	if err == nil {
		t.Errorf("expected error for non-existent server ID, got nil")
	}

	// 3. Zip Slip path traversal attempt in uploaded ZIP
	maliciousZip, mLen := createTestZipArchive(t, map[string]string{
		"../../etc/passwd": "root:x:0:0:root:/root:/bin/bash",
	})
	_, err = driver.Deploy(ctx, DeploymentRequest{
		ServerID:    "srv_valid",
		ProjectName: "Malicious App",
		ZipReader:   bytes.NewReader(maliciousZip),
		ZipSize:     mLen,
	})
	if !errors.Is(err, ErrZipSlip) {
		t.Errorf("expected ErrZipSlip for traversal zip entry, got: %v", err)
	}

	// 4. Path traversal in deployment ID
	_, err = driver.Deploy(ctx, DeploymentRequest{
		ID:          "../../evil_dep",
		ServerID:    "srv_valid",
		ProjectName: "Malicious Dep ID",
	})
	if !errors.Is(err, ErrRemotePathTraversal) {
		t.Errorf("expected ErrRemotePathTraversal for invalid deployment ID, got: %v", err)
	}
}

// 5. DriverFactory Routing Test
func TestDriverFactory_Routing(t *testing.T) {
	server, privKey, _ := newMockDriverSSHServer(t)
	defer server.Close()

	vault := crypto.NewKeyVault("test-secret-vault-1234")
	encKey, _ := vault.Encrypt(privKey)

	srvRepo := db.NewMemoryServerRepository()
	srv := &db.Server{
		ID:           "srv_remote",
		Name:         "Remote VPS",
		Host:         server.addr,
		Port:         server.port,
		Username:     "ubuntu",
		EncryptedKey: encKey,
	}
	_ = srvRepo.Create(context.Background(), srv)

	store := NewMemoryStore()
	portPool := NewPortPool()
	router := NewCaddyRouter(DefaultCaddyConfig())
	sshMgr := NewClientPool(2 * time.Second)
	defer sshMgr.CloseAll()

	storage, err := NewStorageEngine(t.TempDir())
	if err != nil {
		t.Fatalf("storage engine: %v", err)
	}

	localDriver := NewLocalDriverWithRouter(storage, store, portPool, router)
	remoteDriver := NewRemoteSSHDriver(srvRepo, vault, sshMgr, store, portPool, router)
	factory := NewDriverFactory(localDriver, remoteDriver, store)

	// Test GetDriver routing
	if factory.GetDriver("") != localDriver {
		t.Errorf("expected empty server_id to route to localDriver")
	}
	if factory.GetDriver("srv_remote") != remoteDriver {
		t.Errorf("expected srv_remote to route to remoteDriver")
	}

	ctx := context.Background()

	// 1. Deploy local (ServerID = "")
	localZip, lLen := createTestZipArchive(t, map[string]string{
		"index.html": "<h1>Hello Local</h1>",
	})
	localRes, err := factory.Deploy(ctx, DeploymentRequest{
		ProjectName: "Local App",
		Slug:        "local-app",
		ZipReader:   bytes.NewReader(localZip),
		ZipSize:     lLen,
	})
	if err != nil {
		t.Fatalf("factory.Deploy local failed: %v", err)
	}
	if localRes.Deployment.ServerID != "" {
		t.Errorf("expected local deployment ServerID to be empty, got %s", localRes.Deployment.ServerID)
	}

	// 2. Deploy remote (ServerID = "srv_remote")
	remoteZip, rLen := createTestZipArchive(t, map[string]string{
		"index.php": "<?php echo 'Remote App'; ?>",
	})
	remoteRes, err := factory.Deploy(ctx, DeploymentRequest{
		ServerID:    "srv_remote",
		ProjectName: "Remote App",
		Slug:        "remote-app",
		ZipReader:   bytes.NewReader(remoteZip),
		ZipSize:     rLen,
	})
	if err != nil {
		t.Fatalf("factory.Deploy remote failed: %v", err)
	}
	if remoteRes.Deployment.ServerID != "srv_remote" {
		t.Errorf("expected remote deployment ServerID 'srv_remote', got %s", remoteRes.Deployment.ServerID)
	}

	// Verify DriverFor correctly resolves based on stored record
	drvLocal, err := factory.DriverFor(ctx, localRes.Deployment.ID)
	if err != nil || drvLocal != localDriver {
		t.Errorf("expected DriverFor(%s) to return localDriver", localRes.Deployment.ID)
	}

	drvRemote, err := factory.DriverFor(ctx, remoteRes.Deployment.ID)
	if err != nil || drvRemote != remoteDriver {
		t.Errorf("expected DriverFor(%s) to return remoteDriver", remoteRes.Deployment.ID)
	}

	// Test List and Get across drivers
	list, err := factory.List(ctx)
	if err != nil {
		t.Fatalf("factory.List failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 deployments in list, got %d", len(list))
	}

	// Test Remote Start via Factory
	if err := factory.Start(ctx, remoteRes.Deployment.ID); err != nil {
		t.Fatalf("factory.Start remote failed: %v", err)
	}
	hs, err := factory.Health(ctx, remoteRes.Deployment.ID)
	if err != nil {
		t.Fatalf("factory.Health remote failed: %v", err)
	}
	if !hs.Alive {
		t.Errorf("expected remote deployment to be alive")
	}

	// Clean up remote via Factory
	if err := factory.Delete(ctx, remoteRes.Deployment.ID); err != nil {
		t.Fatalf("factory.Delete remote failed: %v", err)
	}
	_, err = factory.Get(ctx, remoteRes.Deployment.ID)
	if !errors.Is(err, ErrDeploymentNotFound) {
		t.Errorf("expected ErrDeploymentNotFound for deleted remote deployment")
	}
}
